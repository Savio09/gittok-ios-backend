package crawler

import (
	"reflect"
	"testing"
)

func TestFactsKeepDeclarationsWhenKindIsUnknown(t *testing.T) {
	insp := InspectionFromSources("", []string{"web/package.json", "tools/go.mod"}, map[string]string{
		"web/package.json": `{"devDependencies":{"react":"^19"},"optionalDependencies":{"sharp":"^1"},"scripts":{"build":"vite build"}}`,
		"tools/go.mod":     "module example.org/tools\nrequire example.org/library v1.2.3\n",
	})
	facts := CollectFacts(insp)
	want := map[string]string{"react": "devDependencies", "sharp": "optionalDependencies", "example.org/library": "require", "build": "scripts"}
	for _, f := range facts {
		if cat, ok := want[f.Name]; ok {
			if f.Category != cat {
				t.Fatalf("category lost: %+v", f)
			}
			delete(want, f.Name)
		}
		if f.Component != "web" && f.Component != "tools" {
			t.Fatalf("wrong scope: %+v", f)
		}
		if err := ValidateFact(insp, f); err != nil {
			t.Fatal(err)
		}
	}
	if len(want) > 0 {
		t.Fatalf("missing independent facts: %v, got %+v", want, facts)
	}
	reversed := insp
	reversed.Files = append([]SourceFile(nil), insp.Files...)
	reversed.Files[0], reversed.Files[1] = reversed.Files[1], reversed.Files[0]
	if !reflect.DeepEqual(facts, CollectFacts(reversed)) {
		t.Fatal("input-order dependent facts")
	}
}

func TestFactsRejectWrongSupportAndComponent(t *testing.T) {
	insp := InspectionFromSources("", []string{"web/package.json"}, map[string]string{"web/package.json": `{"dependencies":{"react":"19"}}`})
	facts := CollectFacts(insp)
	if len(facts) != 1 {
		t.Fatalf("facts: %+v", facts)
	}
	for _, mutate := range []func(*Fact){func(f *Fact) { f.Name = "express" }, func(f *Fact) { f.Component = "" }, func(f *Fact) { f.Spec = "18" }, func(f *Fact) { f.Kind = FactScript }} {
		f := facts[0]
		mutate(&f)
		if ValidateFact(insp, f) == nil {
			t.Fatalf("accepted forged fact %+v", f)
		}
	}
}

func TestFactsSourceCallsAreSyntaxNotTestRuntime(t *testing.T) {
	content := "package main\nimport \"net/http\"\nfunc main(){http.ListenAndServe(\":80\",nil)}\n"
	insp := InspectionFromSources("Go", []string{"main.go", "main_test.go"}, map[string]string{"main.go": content, "main_test.go": content})
	facts := CollectFacts(insp)
	if len(facts) != 1 || facts[0].Kind != FactServerCall || facts[0].Witness.Path != "main.go" {
		t.Fatalf("runtime syntax facts: %+v", facts)
	}
	w := facts[0].Witness
	if content[w.Start:w.End] != w.Quote {
		t.Fatal("not an exact span")
	}
}

func TestFactsMalformedManifestDoesNotYieldDeclarations(t *testing.T) {
	insp := InspectionFromSources("", []string{"package.json"}, map[string]string{"package.json": `{"dependencies":{"react":"19"},`})
	if got := CollectFacts(insp); len(got) != 0 {
		t.Fatalf("malformed source yielded facts: %+v", got)
	}
}

func TestFactsMalformedGoCallCannotPanic(t *testing.T) {
	content := "package main\nimport \"net/http\"\nfunc main(){http.ListenAndServe("
	insp := InspectionFromSources("Go", []string{"main.go"}, map[string]string{"main.go": content})
	insp.Files[0].Truncated = true
	if got := CollectFacts(insp); len(got) != 0 {
		t.Fatalf("invented complete call: %+v", got)
	}
}

func TestFactsPreservePubspecAndCargoDeclarationContext(t *testing.T) {
	insp := InspectionFromSources("", []string{"pubspec.yaml", "rust/Cargo.toml"}, map[string]string{
		"pubspec.yaml":    "name: example\ndependencies:\n  http: ^1.2.0\n",
		"rust/Cargo.toml": "[package]\nname='x'\nversion='0.1.0'\n[dependencies]\nserde={version='1',optional=true}\n",
	})
	got := CollectFacts(insp)
	found := map[string]bool{}
	for _, f := range got {
		if f.Name == "http" {
			found["http"] = f.Spec == "^1.2.0"
		}
		if f.Name == "serde" {
			found["serde"] = f.Optional && f.Spec == "1"
		}
	}
	if !found["http"] || !found["serde"] {
		t.Fatalf("lost declaration context: %+v", got)
	}
}
