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
