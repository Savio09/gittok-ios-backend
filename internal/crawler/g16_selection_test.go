package crawler

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

func TestG16IndependentManifestCoverage(t *testing.T) {
	for _, prefix := range []string{"packages", "components"} {
		var names []string
		files := map[string]string{}
		for _, name := range []string{"a", "b", "c", "d"} {
			p := prefix + "/" + name + "/package.json"
			names = append(names, p)
			files[p] = fmt.Sprintf(`{"name":%q,"exports":"./index.js"}`, name)
		}
		selection := SelectFiles(blobs(names...), DefaultSelectionBudget())
		if len(selection) != 4 {
			t.Fatalf("independent manifests hidden by basename: %v", paths(selection))
		}
		got := Classify(selectedInspection("JavaScript", names, files, selection))
		if got.Kind != apimodel.KindLibrary || got.Status != VerdictResolved {
			t.Fatalf("independent libraries: %s/%s: %v", got.Kind, got.Status, got.Evidence())
		}
		for i, j := 0, len(names)-1; i < j; i, j = i+1, j-1 {
			names[i], names[j] = names[j], names[i]
		}
		if reverse := SelectFiles(blobs(names...), DefaultSelectionBudget()); !reflect.DeepEqual(selection, reverse) {
			t.Fatalf("tree order changed selection: %v versus %v", paths(selection), paths(reverse))
		}
		limited := DefaultSelectionBudget()
		limited.MaxFiles = 3
		selected := SelectFiles(blobs(names...), limited)
		if len(selected) != 3 {
			t.Fatalf("manifest coverage exceeded total budget: %v", paths(selected))
		}
		got = Classify(selectedInspection("JavaScript", names, files, selected))
		if got.Status != VerdictInconclusive || got.Kind != apimodel.KindOther {
			t.Fatalf("unread component disappeared at cap: %+v", got)
		}
	}
}

func TestG16GoPackageEvidencePrecedesTests(t *testing.T) {
	names := []string{"go.mod", "lib.go", "cmd/service.go"}
	files := map[string]string{
		"go.mod": "module example.com/pkg\n", "lib.go": "package pkg\n",
		"cmd/service.go": "package cmd\n",
	}
	for i := 0; i < 8; i++ {
		p := fmt.Sprintf("a%d_test.go", i)
		names = append(names, p)
		files[p] = "package pkg\n"
	}
	selected := SelectFiles(blobs(names...), DefaultSelectionBudget())
	if !contains(selected, "lib.go") || !contains(selected, "cmd/service.go") {
		t.Fatalf("tests displaced Go package evidence: %v", paths(selected))
	}
	got := Classify(selectedInspection("Go", names, files, selected))
	if got.Kind != apimodel.KindLibrary || got.Status != VerdictResolved {
		t.Fatalf("package evidence did not survive selection: %s/%s: %v", got.Kind, got.Status, got.Evidence())
	}
}

func TestG16PrimaryEntryPrecedesSupportingEntries(t *testing.T) {
	names := []string{"go.mod", "examples/main.go", "docs/main.go", "tools/main.go", "cmd/service/main.go"}
	budget := DefaultSelectionBudget()
	budget.MaxFiles = 2
	selected := SelectFiles(blobs(names...), budget)
	if !contains(selected, "cmd/service/main.go") || !contains(selected, "go.mod") || len(selected) != 2 {
		t.Fatalf("supporting entries displaced product: %v", paths(selected))
	}
}

func TestG16CurrentSelectionReplaysCapturedEvidence(t *testing.T) {
	for _, tc := range corpusCases {
		s := loadSnapshot(t, tc.file)
		t.Run(s.Repo, func(t *testing.T) {
			selection := SelectFiles(s.FileTree().Entries, DefaultSelectionBudget())
			requested := paths(selection)
			got := Classify(snapshotInspection(s, requested))
			// Reuse the recorded complete-enough role judgments, but supply only
			// bytes the CURRENT bounded selector requested. Uncaptured bytes remain
			// explicitly unavailable through Snapshot.Fetched; no merged extras.
			if got.Kind != tc.complete.kind || got.Status != tc.complete.status {
				t.Fatalf("current bounded selection: %s/%s, want %s; selected %v\n%s", got.Kind, got.Status, tc.complete, requested, strings.Join(got.Evidence(), "\n"))
			}
			fetched := s.Fetched(requested)
			t.Logf("%s/%s: selected=%d captured=%d unavailable=%d", got.Kind, got.Status, len(requested), len(fetched.Files), len(fetched.Skipped))
		})
	}
}

func selectedInspection(language string, names []string, files map[string]string, selected []SelectedFile) Inspection {
	read := map[string]string{}
	for _, f := range selected {
		if content, ok := files[f.Path]; ok {
			read[f.Path] = content
		}
	}
	return InspectionFromSources(language, names, read)
}

func TestG16CommandScopeCaseMatchesClassifier(t *testing.T) {
	for _, commandDir := range []string{"cmd", "CMD", "Cmd"} {
		names := []string{"go.mod", "lib.go", commandDir + "/app/service.go"}
		files := map[string]string{"go.mod": "module example.com/pkg\n", "lib.go": "package pkg\n", commandDir + "/app/service.go": "package cmd\n"}
		for _, name := range []string{"a", "b", "c", "d", "e", "f"} {
			p := "internal/" + name + ".go"
			names = append(names, p)
			files[p] = "package internal\n"
		}
		selection := SelectFiles(blobs(names...), DefaultSelectionBudget())
		if !contains(selection, commandDir+"/app/service.go") {
			t.Fatalf("command scope %q omitted: %v", commandDir, paths(selection))
		}
		got := Classify(selectedInspection("Go", names, files, selection))
		if got.Kind != apimodel.KindLibrary || got.Status != VerdictResolved {
			t.Fatalf("scope %s: %s/%s", commandDir, got.Kind, got.Status)
		}
	}
}

func TestG16CommandAndComponentNamesRemainOpaque(t *testing.T) {
	for _, name := range []string{"app", "tool", "build", "test", "docs", "examples"} {
		for _, module := range []string{"", "modules/" + name + "/"} {
			command := module + "cmd/" + name + "/main.go"
			mod := module + "go.mod"
			names := []string{mod, "README.md", command}
			budget := DefaultSelectionBudget()
			budget.MaxFiles = 2
			selected := SelectFiles(blobs(names...), budget)
			if !contains(selected, command) || !contains(selected, mod) {
				t.Errorf("product name %q module %q displaced: %v", name, module, paths(selected))
			}
		}
	}
	// Supporting ancestors and command-internal fixtures keep their scope,
	// even when fixture directories define their own components.
	for _, helper := range []string{"examples/cmd/tool/main.go", "cmd/tool/testdata/main.go", "cmd/tool/internal/cmd/testdata/main.go"} {
		for _, manifest := range []string{"", "package.json", "go.mod"} {
			budget := DefaultSelectionBudget()
			budget.MaxFiles = 2
			names := []string{"go.mod", "README.md", helper}
			if manifest != "" {
				names = append(names, strings.TrimSuffix(helper, "main.go")+manifest)
			}
			selected := SelectFiles(blobs(names...), budget)
			if contains(selected, helper) {
				t.Errorf("supporting path reserved with manifest %q: %s", manifest, helper)
			}
		}
	}
}
