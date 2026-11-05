package crawler

import (
	"strings"
	"testing"
)

// goModLocationSource mixes the layouts x/mod accepts: CRLF line endings, a
// single-line require, a grouped block, a quoted module path, indirect suffix
// comments and a duplicated requirement whose later declaration is effective.
const goModLocationSource = "module example.com/m\r\n\r\n" +
	"require github.com/a/b v1.0.0\r\n" +
	"require (\r\n" +
	"\tgithub.com/c/d v1.2 // indirect\r\n" +
	"\t\"github.com/e/f\" v2.0.0+incompatible\r\n" +
	"\tgithub.com/i/j v0.1.0 // indirect\r\n" +
	"\tgithub.com/c/d v1.3.0\r\n" +
	")\r\n" +
	"require github.com/g/h v1.0.0 // indirect\n"

func goModFacts(t *testing.T, content string, truncated bool) map[string]Fact {
	t.Helper()
	insp := InspectionFromSources("Go", []string{"go.mod"}, map[string]string{"go.mod": content})
	if truncated {
		insp.Files[0].Truncated, insp.Files[0].Size = true, len(content)+100
	}
	out := map[string]Fact{}
	for _, f := range CollectFacts(insp) {
		if f.Kind == FactDependency {
			out[f.Name] = f
		}
	}
	return out
}

// Each published requirement cites the exact bytes of its own declaration. The
// expected ranges come from the literal text above, not from the parser.
func TestGoRequirementFactsCarryExactSourceLocations(t *testing.T) {
	facts := goModFacts(t, goModLocationSource, false)
	for name, quote := range map[string]string{
		"github.com/a/b": "require github.com/a/b v1.0.0",
		"github.com/c/d": "github.com/c/d v1.3.0",
		"github.com/e/f": "\"github.com/e/f\" v2.0.0+incompatible",
		"github.com/i/j": "github.com/i/j v0.1.0 // indirect",
		"github.com/g/h": "require github.com/g/h v1.0.0 // indirect",
	} {
		f, ok := facts[name]
		if !ok {
			t.Errorf("%s: no published declaration (got %v)", name, facts)
			continue
		}
		if strings.Count(goModLocationSource, quote) != 1 {
			t.Fatalf("fixture quote %q is not unique", quote)
		}
		start := strings.Index(goModLocationSource, quote)
		w := f.Witness
		if w.Start != start || w.End != start+len(quote) || w.Quote != quote {
			t.Errorf("%s: witness span %d-%d %q, want %d-%d %q", name, w.Start, w.End, w.Quote, start, start+len(quote), quote)
		}
	}
	if f := facts["github.com/c/d"]; f.Spec != "v1.3.0" {
		t.Errorf("duplicate requirement must cite the effective declaration: %+v", f)
	}
}

// A location is part of the claim: any change to the span, its quote or the
// field it is attached to must fail validation, even with a real path.
func TestGoRequirementLocationTamperingIsRejected(t *testing.T) {
	insp := InspectionFromSources("Go", []string{"go.mod"}, map[string]string{"go.mod": goModLocationSource})
	st := indexInspection(insp)
	facts := goModFacts(t, goModLocationSource, false)
	good, other := facts["github.com/a/b"].Witness, facts["github.com/e/f"].Witness
	if err := validateWitness(st, good); err != nil {
		t.Fatalf("published witness does not validate: %v", err)
	}
	for name, mutate := range map[string]func(*Witness){
		"shifted start":       func(w *Witness) { w.Start++; w.Quote = goModLocationSource[w.Start:w.End] },
		"extended end":        func(w *Witness) { w.End++; w.Quote = goModLocationSource[w.Start:w.End] },
		"another declaration": func(w *Witness) { w.Start, w.End, w.Quote = other.Start, other.End, other.Quote },
		"altered quote":       func(w *Witness) { w.Quote = strings.Replace(w.Quote, "v1.0.0", "v9.0.0", 1) },
		"different field":     func(w *Witness) { w.Field, w.Value = []string{"require", "github.com/e/f"}, "v2.0.0+incompatible" },
		"span past the file":  func(w *Witness) { w.End = len(goModLocationSource) + 5 },
	} {
		w := good
		w.Field = append([]string(nil), good.Field...)
		mutate(&w)
		if validateWitness(st, w) == nil {
			t.Errorf("%s: accepted %+v", name, w)
		}
	}
}

// A clipped go.mod can end inside a declaration ("v1.2" of "v1.2.3"). Only
// declarations whose line ends inside the consumed bytes may be published.
func TestClippedGoModCannotPublishACutDeclaration(t *testing.T) {
	content := "module example.com/m\nrequire github.com/a/b v1.0.0\nrequire github.com/c/d v1.2"
	facts := goModFacts(t, content, true)
	if _, ok := facts["github.com/c/d"]; ok {
		t.Errorf("published a declaration cut by the read limit: %+v", facts["github.com/c/d"])
	}
	f, ok := facts["github.com/a/b"]
	if !ok || f.Witness.Quote != "require github.com/a/b v1.0.0" {
		t.Errorf("lost a complete declaration before the clip: %+v", facts)
	}
	complete := goModFacts(t, content, false)
	if f := complete["github.com/c/d"]; f.Witness.Quote != "require github.com/c/d v1.2" {
		t.Errorf("a fully read final line is a complete declaration: %+v", complete)
	}
}

// Classification observations that cite a requirement without a span remain
// valid; a span on a field kind with no verified locator is rejected.
func TestFieldSpanNeedsAVerifiedLocator(t *testing.T) {
	insp := InspectionFromSources("", []string{"go.mod", "package.json", "Cargo.toml"}, map[string]string{
		"go.mod":       "module example.com/m\nrequire github.com/a/b v1.0.0\n",
		"package.json": `{"name":"x"}`,
		"Cargo.toml":   "[package]\nname = \"x\"\n",
	})
	st := indexInspection(insp)
	if err := validateWitness(st, Witness{Kind: WitnessField, Path: "go.mod", Field: []string{"require", "github.com/a/b"}, Value: "v1.0.0"}); err != nil {
		t.Errorf("span-free field witness rejected: %v", err)
	}
	if err := validateWitness(st, Witness{Kind: WitnessField, Path: "package.json", Field: []string{"name"}, Value: "x", Start: 1, End: 11, Quote: `"name":"x"`}); err != nil {
		t.Errorf("exact JSON member location rejected: %v", err)
	}
	// Cargo.toml has no verified locator: a span claimed there cannot be checked.
	if validateWitness(st, Witness{Kind: WitnessField, Path: "Cargo.toml", Field: []string{"package", "name"}, Value: "x", Start: 10, End: 20, Quote: "name = \"x\""}) == nil {
		t.Error("accepted an unverifiable field span")
	}
}
