package crawler

import (
	"reflect"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

// Slice 2: manifest selection must be deterministic and component-scope aware.
// A nested example's manifest can never supply the primary signal or overwrite
// the root's provenance, and the choice must not depend on input ordering.

// pickManifest prefers the root-scoped manifest over nested, example and docs
// scopes, regardless of the order it sees the files in; when the root manifest
// could not be read it stays the chosen one, with its state, rather than being
// silently replaced by a nested manifest.
func TestPickManifestPrefersRootScope(t *testing.T) {
	x := indexInspection(Inspection{
		Paths: []string{"docs/package.json", "examples/mobile/package.json", "package.json", "packages/pkg-a/package.json"},
		Files: []SourceFile{
			{Path: "docs/package.json", Content: `{"name":"docs"}`},
			{Path: "examples/mobile/package.json", Content: `{"name":"ex"}`},
			{Path: "package.json", Content: `{"name":"root"}`},
			{Path: "packages/pkg-a/package.json", Content: `{"name":"a"}`},
		},
	})
	if m := pickManifest(x, "package.json"); m.path != "package.json" || m.state != StateRead {
		t.Errorf("pickManifest chose %+v, want the root package.json, read", m)
	}

	onlyNested := indexInspection(Inspection{
		Paths: []string{"apps/mobile/package.json"},
		Files: []SourceFile{{Path: "apps/mobile/package.json", Content: `{"name":"m"}`}},
	})
	if m := pickManifest(onlyNested, "package.json"); m.path != "apps/mobile/package.json" {
		t.Errorf("pickManifest chose %q, want the only (nested) package.json", m.path)
	}

	failedRoot := indexInspection(Inspection{
		Paths:       []string{"package.json", "examples/mobile/package.json"},
		Files:       []SourceFile{{Path: "examples/mobile/package.json", Content: `{"dependencies":{"expo":"51"}}`}},
		Unavailable: []Unavailable{{Path: "package.json", Reason: "fetch error"}},
	})
	if m := pickManifest(failedRoot, "package.json"); m.path != "package.json" || m.state != StateUnavailable || m.content != "" {
		t.Errorf("a failed root manifest must stay chosen with its state; got %+v", m)
	}
}

// A root library with a nested Expo example classifies as a library every time
// and across shuffled input, and the example never makes it mobile.
func TestRootLibraryWithNestedExampleIsStableAndNotMobile(t *testing.T) {
	paths := []string{"package.json", "src/index.ts", "examples/mobile/package.json"}
	rootPkg := SourceFile{Path: "package.json", Content: `{"name":"core","main":"dist/index.js"}`}
	examplePkg := SourceFile{Path: "examples/mobile/package.json", Content: `{"dependencies":{"expo":"51"}}`}

	ordered := Inspection{Language: "TypeScript", Paths: paths, Files: []SourceFile{rootPkg, examplePkg}}
	shuffled := Inspection{Language: "TypeScript", Paths: paths, Files: []SourceFile{examplePkg, rootPkg}}

	got := Classify(ordered)
	if got.Kind != apimodel.KindLibrary {
		t.Errorf("Kind = %q, want %q (the nested Expo example must not set the primary kind)", got.Kind, apimodel.KindLibrary)
	}
	for _, o := range got.Observations {
		if o.Path == "examples/mobile/package.json" {
			t.Errorf("a primary observation was attributed to the example manifest %q", o.Path)
		}
	}

	// Order independence: the whole verdict is byte-identical whatever order the
	// files arrive in.
	if !reflect.DeepEqual(got, Classify(shuffled)) {
		t.Errorf("classification differs by input order:\n ordered = %+v\n shuffled = %+v", got, Classify(shuffled))
	}

	// Repeated calls are stable.
	seen := map[apimodel.RepoKind]int{}
	for i := 0; i < 1000; i++ {
		seen[Classify(ordered).Kind]++
	}
	if len(seen) != 1 {
		t.Errorf("identical input yielded different kinds across runs: %v", seen)
	}
}

// A nested manifest of the same kind does not overwrite the root's provenance:
// the library verdict cites the root go.mod, not a sibling component's.
func TestNestedManifestDoesNotOverwriteRootProvenance(t *testing.T) {
	insp := Inspection{
		Language: "Go",
		Paths:    []string{"go.mod", "calc.go", "packages/helper/go.mod", "packages/helper/helper.go"},
		Files: []SourceFile{
			{Path: "go.mod", Content: "module example.com/calc\n"},
			{Path: "calc.go", Content: "package calc\n"},
			{Path: "packages/helper/go.mod", Content: "module example.com/calc/helper\n"},
			{Path: "packages/helper/helper.go", Content: "package helper\n"},
		},
	}
	got := Classify(insp)
	if got.Kind != apimodel.KindLibrary {
		t.Fatalf("Kind = %q, want %q", got.Kind, apimodel.KindLibrary)
	}
	citedRoot := false
	for _, o := range got.Observations {
		if o.Path == "go.mod" {
			citedRoot = true
		}
		if o.Path == "packages/helper/go.mod" {
			t.Errorf("library verdict cited the nested module %q instead of the root", o.Path)
		}
	}
	if !citedRoot {
		t.Errorf("library verdict did not cite the root go.mod; observations = %+v", got.Observations)
	}
}
