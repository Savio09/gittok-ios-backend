package crawler

import (
	"strings"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

// These tests cover Slice 1 of the classification review repair: the classifier
// must distinguish "the repository does not do X" from "X was not in the part we
// inspected". A bounded fetch — an unread entry point, a truncated or
// depth-limited tree, a manifest that would not parse — must surface as a
// limitation and an honest abstention, never as a silent negative fact.

func limitationsContain(limits []string, substr string) bool {
	for _, l := range limits {
		if strings.Contains(l, substr) {
			return true
		}
	}
	return false
}

// A Go repo whose only entry point was NOT read must not be called a CLI by the
// absence of a server signal; it must abstain and say why. Reading the same
// entry point flips the verdict — proving failure is not treated as empty content.
func TestUnreadGoEntryAbstainsInsteadOfGuessingCLI(t *testing.T) {
	paths := []string{"go.mod", "cmd/service/main.go", "internal/server/server.go"}

	// Arrange: the entry point's source could not be fetched.
	unread := Inspection{
		Language: "Go",
		Paths:    paths,
		Files:    []SourceFile{{Path: "go.mod", Content: "module example.com/service\n", Size: 27}},
		Unavailable: []Unavailable{
			{Path: "cmd/service/main.go", Reason: "fetch error"},
			{Path: "internal/server/server.go", Reason: "fetch error"},
		},
	}
	got := Classify(unread)

	if got.Kind == apimodel.KindCLI {
		t.Errorf("unread entry point classified as CLI; want an abstention (got %q)", got.Kind)
	}
	if got.Status != VerdictInconclusive {
		t.Errorf("Status = %q, want %q for an unread entry point", got.Status, VerdictInconclusive)
	}
	if !limitationsContain(got.Limitations, "cmd/service/main.go") {
		t.Errorf("limitations %v, want one naming the unread entry point", got.Limitations)
	}

	// Act: the same listing, but now the entry source was read and shows a plain
	// command (package main, under cmd/, no server call).
	read := unread
	read.Files = []SourceFile{
		{Path: "go.mod", Content: "module example.com/service\n", Size: 27},
		{Path: "cmd/service/main.go", Content: "package main\n\nfunc main() {}\n", Size: 29},
	}
	read.Unavailable = nil
	readGot := Classify(read)

	if readGot.Kind != apimodel.KindCLI {
		t.Errorf("read entry point: Kind = %q, want %q", readGot.Kind, apimodel.KindCLI)
	}
	if readGot.Status != VerdictResolved {
		t.Errorf("read entry point: Status = %q, want %q", readGot.Status, VerdictResolved)
	}
}

// A complete tree with no package main is an importable library; a truncated or
// depth-limited tree is not enough to assert that absence.
func TestLibraryByAbsenceRequiresCompleteTree(t *testing.T) {
	base := Inspection{
		Language: "Go",
		Paths:    []string{"go.mod", "pkg/calc/calc.go"},
		Files:    []SourceFile{{Path: "go.mod", Content: "module example.com/calc\n", Size: 24}},
	}

	complete := base // Scope zero value is complete
	if got := Classify(complete); got.Kind != apimodel.KindLibrary {
		t.Errorf("complete tree, no main: Kind = %q, want %q", got.Kind, apimodel.KindLibrary)
	}

	truncated := base
	truncated.Scope = TreeScope{Truncated: true}
	if got := Classify(truncated); got.Kind == apimodel.KindLibrary {
		t.Errorf("truncated tree: classified library by absence of main; want abstention (got %q, status %q)", got.Kind, got.Status)
	} else if !limitationsContain(got.Limitations, "truncated") {
		t.Errorf("truncated tree: limitations %v, want one mentioning truncation", got.Limitations)
	}

	depthLimited := base
	depthLimited.Scope = TreeScope{DepthLimited: true}
	if got := Classify(depthLimited); got.Kind == apimodel.KindLibrary {
		t.Errorf("depth-limited tree: classified library by absence of main; want abstention (got %q)", got.Kind)
	} else if !limitationsContain(got.Limitations, "depth-limited") {
		t.Errorf("depth-limited tree: limitations %v, want one mentioning the depth limit", got.Limitations)
	}
}

// A malformed manifest must never panic and must leave a visible limitation
// rather than being read as "no signal".
func TestMalformedPackageJSONYieldsLimitationNotPanic(t *testing.T) {
	insp := InspectionFromSources("TypeScript",
		[]string{"package.json", "src/index.ts"},
		map[string]string{"package.json": "{ this is not valid json "},
	)
	got := Classify(insp) // must not panic
	if !limitationsContain(got.Limitations, "could not be parsed") {
		t.Errorf("limitations %v, want one reporting the unparseable package.json", got.Limitations)
	}
	if got.Status != VerdictInconclusive {
		t.Errorf("Status = %q, want %q when the only manifest is unparseable", got.Status, VerdictInconclusive)
	}
}

// An observation must cite the actual source path it was read from — a nested
// manifest is never relabelled as the repository root.
func TestObservationCitesActualNestedPath(t *testing.T) {
	insp := InspectionFromSources("TypeScript",
		[]string{"apps/mobile/package.json"},
		map[string]string{"apps/mobile/package.json": `{"dependencies":{"expo":"51"}}`},
	)
	got := Classify(insp)
	if got.Kind != apimodel.KindMobile {
		t.Fatalf("Kind = %q, want %q", got.Kind, apimodel.KindMobile)
	}
	cited := false
	for _, o := range got.Observations {
		if o.Path == "apps/mobile/package.json" {
			cited = true
		}
		if o.Path == "package.json" {
			t.Errorf("observation relabelled the nested manifest as root %q", o.Path)
		}
	}
	if !cited {
		t.Errorf("no observation cited apps/mobile/package.json; observations = %+v", got.Observations)
	}
}

// A completely empty inspection abstains with KindOther and an explanation,
// rather than asserting a confident classification.
func TestEmptyInspectionAbstains(t *testing.T) {
	got := Classify(Inspection{})
	if got.Kind != apimodel.KindOther {
		t.Errorf("Kind = %q, want %q", got.Kind, apimodel.KindOther)
	}
	if got.Status != VerdictInconclusive {
		t.Errorf("Status = %q, want %q", got.Status, VerdictInconclusive)
	}
	if len(got.Evidence()) == 0 {
		t.Errorf("an abstention must still explain itself; Evidence() was empty")
	}
}
