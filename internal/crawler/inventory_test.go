package crawler

import (
	"reflect"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

// Slice 2: manifest selection must be deterministic and component-scope aware.
// A nested example's manifest can never supply the primary signal or overwrite
// the root's provenance, and the choice must not depend on input ordering.

// The root manifest speaks for the repository even when nested components
// carry their own manifests; when it could not be read, the nested example is
// not promoted in its place (see TestAbsencePremisesNeedInspectedScope).
func TestRootManifestSpeaksForTheRepository(t *testing.T) {
	got := Classify(InspectionFromSources("TypeScript",
		[]string{"package.json", "docs/package.json", "examples/mobile/package.json", "packages/pkg-a/package.json"},
		map[string]string{
			"package.json":                 `{"name":"root","main":"dist/index.js"}`,
			"docs/package.json":            `{"name":"docs","dependencies":{"react":"18"}}`,
			"examples/mobile/package.json": `{"dependencies":{"expo":"51"}}`,
			"packages/pkg-a/package.json":  `{"name":"a","bin":"cli.js"}`,
		}))
	if got.Kind != apimodel.KindLibrary || got.Status != VerdictResolved {
		t.Fatalf("root library: got %q/%q: %v", got.Kind, got.Status, got.Evidence())
	}
	for _, o := range got.Observations {
		if o.Component != "" {
			t.Errorf("a primary observation came from component %q", o.Component)
		}
	}
	owners := map[string]bool{}
	for _, o := range got.Secondary {
		owners[o.Component] = true
	}
	if !owners["examples/mobile"] || !owners["packages/pkg-a"] {
		t.Errorf("nested components must stay attributable as secondary evidence: %v", got.Secondary)
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
