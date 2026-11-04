package crawler

import (
	"reflect"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

// The verdict's basis is the typed premise set a presentation layer may cite:
// one complete validated evidence group, never parsed from observation prose.

func basisPremisesValidate(t *testing.T, insp Inspection, b *Basis) {
	t.Helper()
	st := indexInspection(insp)
	for _, p := range b.Premises {
		if err := validateWitness(st, p.Witness); err != nil {
			t.Errorf("basis premise does not validate: %+v: %v", p.Witness, err)
		}
		if p.Context {
			t.Errorf("supporting context is not a premise: %+v", p)
		}
	}
}

func TestBasisForACommandWiredToACLIFramework(t *testing.T) {
	gomod := "module example.com/tool\n\nrequire github.com/spf13/cobra v1.8.0\n"
	insp := InspectionFromSources("Go", []string{"go.mod", "cmd/tool/main.go"}, map[string]string{
		"go.mod":           gomod,
		"cmd/tool/main.go": "package main\n\nfunc main() {}\n",
	})
	c := Classify(insp)
	if c.Kind != apimodel.KindCLI || c.Basis == nil {
		t.Fatalf("got %s with basis %+v", c.Kind, c.Basis)
	}
	b := c.Basis
	if b.Component != "" || b.ByComparison || len(b.Compared) != 0 {
		t.Errorf("root verdict basis: %+v", b)
	}
	var clause, framework bool
	for _, p := range b.Premises {
		switch {
		case p.Witness.Kind == WitnessSpan && p.Witness.Construct == constructGoPackageMain && p.Witness.Path == "cmd/tool/main.go":
			clause = true
		case p.Witness.Kind == WitnessField && reflect.DeepEqual(p.Witness.Field, []string{"require", "github.com/spf13/cobra"}):
			framework = true
			if p.Witness.Quote != "require github.com/spf13/cobra v1.8.0" {
				t.Errorf("framework premise lacks its verified location: %+v", p.Witness)
			}
		}
	}
	if !clause || !framework {
		t.Errorf("basis must carry both premises (package main and the CLI framework): %+v", b.Premises)
	}
	basisPremisesValidate(t, insp, b)
}

func TestBasisCarriesTypedAbsencePremises(t *testing.T) {
	insp := InspectionFromSources("Go", []string{"go.mod", "lib.go"}, map[string]string{
		"go.mod": "module example.com/lib\n",
		"lib.go": "package lib\n\nfunc F() {}\n",
	})
	c := Classify(insp)
	if c.Kind != apimodel.KindLibrary || c.Basis == nil {
		t.Fatalf("got %s with basis %+v", c.Kind, c.Basis)
	}
	found := false
	for _, p := range c.Basis.Premises {
		if p.Witness.Kind == WitnessScope {
			found = true
			if p.Absence != AbsenceGoCommand {
				t.Errorf("absence premise is untyped: %+v", p)
			}
		}
	}
	if !found {
		t.Errorf("library basis lost its bounded absence premise: %+v", c.Basis.Premises)
	}
	basisPremisesValidate(t, insp, c.Basis)
}

func TestBasisIsAbsentForAnAbstention(t *testing.T) {
	c := Classify(InspectionFromSources("", []string{"README.md"}, map[string]string{"README.md": "hello"}))
	if c.Status != VerdictInconclusive || c.Basis != nil {
		t.Fatalf("an abstention has no basis: %s %+v", c.Status, c.Basis)
	}
}

func TestBasisRecordsANestedComparison(t *testing.T) {
	insp := InspectionFromSources("TypeScript", []string{"web/package.json", "web/index.html", "lib/package.json"}, map[string]string{
		"web/package.json": `{"dependencies":{"react":"18"}}`,
		"lib/package.json": `{"name":"lib","exports":"./index.js"}`,
	})
	c := Classify(insp)
	if c.Kind != apimodel.KindFrontend || c.Basis == nil {
		t.Fatalf("got %s/%s with basis %+v: %v", c.Kind, c.Status, c.Basis, c.Evidence())
	}
	b := c.Basis
	if b.Component != "web" || !b.ByComparison || len(b.Compared) != 1 || b.Compared[0].Component != "lib" || b.Compared[0].Kind != apimodel.KindLibrary {
		t.Fatalf("comparison basis: %+v", b)
	}
	basisPremisesValidate(t, insp, b)
	for _, other := range b.Compared {
		basisPremisesValidate(t, insp, &other)
	}
}

func TestInspectionKeepsTheTreeRequestThatProducedItsListing(t *testing.T) {
	tree := githubclient.FileTree{Entries: []githubclient.TreeEntry{{Path: "go.mod", Type: "blob"}}, Request: githubclient.TreeRequestPath("o", "r", "0123456789abcdef0123456789abcdef01234567")}
	insp := NewInspection("Go", Revision{Ref: "0123456789abcdef0123456789abcdef01234567", Resolved: true}, tree, githubclient.FetchedContents{})
	if insp.TreeRequest != "/repos/o/r/git/trees/0123456789abcdef0123456789abcdef01234567?recursive=1" {
		t.Fatalf("tree provenance lost: %q", insp.TreeRequest)
	}
	if InspectionFromSources("", []string{"go.mod"}, nil).TreeRequest != "" {
		t.Fatal("a synthetic listing must not claim provider provenance")
	}
}

// SW2 (swarm review): "no command at the conventional locations" rests on the
// package clauses of the inspected root and cmd/ Go files, not only on go.mod
// and the tree. The premise names every such file, and its validator rejects a
// premise that omits one or relies on one that was not read.
func TestGoNoCommandPremiseNamesEveryInspectedClause(t *testing.T) {
	files := map[string]string{"go.mod": "module example.com/lib\n", "lib.go": "package lib\nfunc F() {}\n", "cmd/tool/helper.go": "package helper\n"}
	paths := []string{"go.mod", "lib.go", "cmd/tool/helper.go"}
	insp := InspectionFromSources("Go", paths, files)
	c := Classify(insp)
	if c.Kind != apimodel.KindLibrary || c.Basis == nil {
		t.Fatalf("fixture: %s %+v", c.Kind, c.Basis)
	}
	var premise *Observation
	for i, p := range c.Basis.Premises {
		if p.Absence == AbsenceGoCommand {
			premise = &c.Basis.Premises[i]
		}
	}
	if premise == nil {
		t.Fatalf("no typed no-command premise: %+v", c.Basis.Premises)
	}
	if !reflect.DeepEqual(premise.Witness.Paths, []string{"go.mod", "cmd/tool/helper.go", "lib.go"}) {
		t.Errorf("premise scope %v must name go.mod and every inspected clause it relies on", premise.Witness.Paths)
	}
	st := indexInspection(insp)
	if err := validateWitness(st, premise.Witness); err != nil {
		t.Fatalf("published premise does not validate: %v", err)
	}
	omitted := premise.Witness
	omitted.Paths = []string{"go.mod", "lib.go"}
	if validateWitness(st, omitted) == nil {
		t.Error("a premise omitting cmd/tool/helper.go validated")
	}
	delete(files, "cmd/tool/helper.go")
	unread := InspectionFromSources("Go", paths, files)
	if validateWitness(indexInspection(unread), premise.Witness) == nil {
		t.Error("a premise relying on an unread file validated")
	}
	if got := Classify(unread); got.Kind == apimodel.KindLibrary {
		t.Error("an unread cmd/ file still allowed the library verdict")
	}
}

// SW4 (swarm review): "no app shell" is a listing absence. An app shell
// (index.html) may sit at any depth, so a truncated or depth-limited listing
// cannot establish it and the library verdict abstains.
func TestNPMLibraryAbsenceNeedsACompleteListing(t *testing.T) {
	for name, tc := range map[string]struct {
		manifest string
		scope    TreeScope
		want     apimodel.RepoKind
	}{
		"complete listing":             {`{"main":"./index.js"}`, TreeScope{}, apimodel.KindLibrary},
		"truncated listing":            {`{"main":"./index.js"}`, TreeScope{Truncated: true}, apimodel.KindOther},
		"depth-limited listing":        {`{"main":"./index.js"}`, TreeScope{DepthLimited: true, MaxDepth: 4}, apimodel.KindOther},
		"framework, truncated listing": {`{"main":"./index.js","dependencies":{"react":"19"}}`, TreeScope{Truncated: true}, apimodel.KindOther},
		"framework, complete listing":  {`{"main":"./index.js","dependencies":{"react":"19"}}`, TreeScope{}, apimodel.KindLibrary},
	} {
		t.Run(name, func(t *testing.T) {
			insp := InspectionFromSources("JavaScript", []string{"package.json", "index.js"}, map[string]string{"package.json": tc.manifest})
			insp.Scope = tc.scope
			c := Classify(insp)
			if c.Kind != tc.want {
				t.Fatalf("kind %s, want %s: %v", c.Kind, tc.want, c.Evidence())
			}
			if c.Basis != nil {
				basisPremisesValidate(t, insp, c.Basis)
			}
		})
	}
}
