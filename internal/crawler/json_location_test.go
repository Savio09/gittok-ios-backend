package crawler

import (
	"strings"
	"testing"
)

// packageJSONLocationSource exercises what a position-aware JSON pass must get
// right: CRLF, an escaped key that decodes to "scripts", a nested lookalike
// "dependencies" table, a duplicated key whose later value is effective, a
// value on the line after its key, non-ASCII text and string/map "bin" forms.
const packageJSONLocationSource = "{\r\n" +
	"  \"name\": \"café-app\",\r\n" +
	"  \"config\": {\"dependencies\": {\"react\": \"0.0.1\"}},\r\n" +
	"  \"dependencies\": {\r\n" +
	"    \"react\": \"^18.0.0\",\r\n" +
	"    \"left-pad\": \"1.3.0\",\r\n" +
	"    \"react\": \"^19.1.0\"\r\n" +
	"  },\r\n" +
	"  \"devDependencies\": {\"vite\": \"^8.0.0\", \"éclair\": \"2.0.0\"},\r\n" +
	"  \"scr\\u0069pts\": {\r\n" +
	"    \"dev\": \"vite\",\r\n" +
	"    \"build\":\r\n" +
	"      \"dprint fmt && vite build\"\r\n" +
	"  },\r\n" +
	"  \"bin\": {\"tool\": \"./cli.js\"}\r\n" +
	"}\r\n"

func npmFacts(t *testing.T, content string) map[string]Fact {
	t.Helper()
	out := map[string]Fact{}
	for _, f := range CollectFacts(InspectionFromSources("", []string{"package.json"}, map[string]string{"package.json": content})) {
		out[string(f.Kind)+":"+f.Name] = f
	}
	return out
}

func TestPackageJSONFactsCarryExactSourceLocations(t *testing.T) {
	facts := npmFacts(t, packageJSONLocationSource)
	for key, quote := range map[string]string{
		"dependency:react":    "\"react\": \"^19.1.0\"",
		"dependency:left-pad": "\"left-pad\": \"1.3.0\"",
		"dependency:vite":     "\"vite\": \"^8.0.0\"",
		"dependency:éclair":   "\"éclair\": \"2.0.0\"",
		"script:dev":          "\"dev\": \"vite\"",
		"script:build":        "\"build\":\r\n      \"dprint fmt && vite build\"",
		"command:bin.tool":    "\"tool\": \"./cli.js\"",
	} {
		f, ok := facts[key]
		if !ok {
			t.Errorf("%s: not published (got %v)", key, facts)
			continue
		}
		if strings.Count(packageJSONLocationSource, quote) != 1 {
			t.Fatalf("fixture quote %q is not unique", quote)
		}
		start := strings.Index(packageJSONLocationSource, quote)
		w := f.Witness
		if w.Start != start || w.End != start+len(quote) || w.Quote != quote {
			t.Errorf("%s: span %d-%d %q, want %d-%d %q", key, w.Start, w.End, w.Quote, start, start+len(quote), quote)
		}
	}
	if f := facts["dependency:react"]; f.Spec != "^19.1.0" {
		t.Errorf("duplicate key must cite the effective (last) value: %+v", f)
	}
}

func TestPackageJSONStringBinCitesItsMember(t *testing.T) {
	content := "{\"name\":\"x\",\n \"bin\": \"./cli.js\"}\n"
	f, ok := npmFacts(t, content)["command:bin"]
	if !ok || f.Witness.Quote != "\"bin\": \"./cli.js\"" {
		t.Fatalf("string bin location: %+v", f)
	}
}

func TestPackageJSONLocationTamperingIsRejected(t *testing.T) {
	insp := InspectionFromSources("", []string{"package.json"}, map[string]string{"package.json": packageJSONLocationSource})
	st := indexInspection(insp)
	facts := npmFacts(t, packageJSONLocationSource)
	good, other := facts["dependency:left-pad"].Witness, facts["script:dev"].Witness
	if err := validateWitness(st, good); err != nil {
		t.Fatalf("published witness does not validate: %v", err)
	}
	nested := strings.Index(packageJSONLocationSource, "\"react\": \"0.0.1\"")
	for name, mutate := range map[string]func(*Witness){
		"shifted start":  func(w *Witness) { w.Start++; w.Quote = packageJSONLocationSource[w.Start:w.End] },
		"another member": func(w *Witness) { w.Start, w.End, w.Quote = other.Start, other.End, other.Quote },
		"altered quote":  func(w *Witness) { w.Quote = strings.Replace(w.Quote, "1.3.0", "9.9.9", 1) },
		"nested lookalike": func(w *Witness) {
			w.Field, w.Value = []string{"dependencies", "react"}, "^19.1.0"
			w.Start, w.End = nested, nested+len("\"react\": \"0.0.1\"")
			w.Quote = packageJSONLocationSource[w.Start:w.End]
		},
		"overwritten duplicate": func(w *Witness) {
			first := strings.Index(packageJSONLocationSource, "\"react\": \"^18.0.0\"")
			w.Field, w.Value = []string{"dependencies", "react"}, "^19.1.0"
			w.Start, w.End = first, first+len("\"react\": \"^18.0.0\"")
			w.Quote = packageJSONLocationSource[w.Start:w.End]
		},
	} {
		w := good
		w.Field = append([]string(nil), good.Field...)
		mutate(&w)
		if validateWitness(st, w) == nil {
			t.Errorf("%s: accepted %+v", name, w)
		}
	}
}

// SW1 (swarm review): a legal key containing NUL must not alias a nested
// field path. Location keys are structural, not joined strings.
func TestJSONLocationKeysCannotCollide(t *testing.T) {
	raw := "{\n  \"dependencies\": {\"react\": \"19\"},\n  \"dependencies\\u0000react\": \"UNRELATED\"\n}\n"
	f, ok := npmFacts(t, raw)["dependency:react"]
	if !ok || f.Witness.Quote != `"react": "19"` {
		t.Fatalf("react cites %q, want its own declaration", f.Witness.Quote)
	}
	insp := InspectionFromSources("", []string{"package.json"}, map[string]string{"package.json": raw})
	st := indexInspection(insp)
	forged := f.Witness
	other := strings.Index(raw, `"dependencies\u0000react"`)
	forged.Start, forged.End = other, other+len(`"dependencies\u0000react": "UNRELATED"`)
	forged.Quote = raw[forged.Start:forged.End]
	if validateWitness(st, forged) == nil {
		t.Fatal("the colliding member validated as the react declaration")
	}
}
