package crawler

import (
	"strings"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

// Slice 9 negative controls: provenance is not support. An inspected file that
// does not contain the construct, and an omitted source, must never become an
// accepted supporting observation.

func TestNegativeControlIrrelevantSourceIsNotSupport(t *testing.T) {
	insp := InspectionFromSources("Go",
		[]string{"go.mod", "cmd/tool/main.go", "internal/util/util.go"},
		map[string]string{
			"go.mod":                "module example.com/tool\n",
			"cmd/tool/main.go":      "package main\nfunc main() {}\n",
			"internal/util/util.go": "package util\n// one day we might call http.ListenAndServe here\nfunc Noop() {}\n",
		},
	)
	got := Classify(insp)
	if got.Kind == apimodel.KindBackend {
		t.Errorf("an irrelevant file's comment promoted the repo to backend: %v", got.Evidence())
	}
	for _, o := range got.Observations {
		if strings.Contains(o.Detail, "ListenAndServe") {
			t.Errorf("fabricated a server observation from an irrelevant file: %+v", o)
		}
	}
}

func TestNegativeControlOmittedSourceIsNotSupport(t *testing.T) {
	insp := Inspection{
		Language:    "Go",
		Paths:       []string{"go.mod", "cmd/app/main.go"},
		Files:       []SourceFile{{Path: "go.mod", Content: "module example.com/app\n"}},
		Unavailable: []Unavailable{{Path: "cmd/app/main.go", Reason: "fetch error"}},
	}
	got := Classify(insp)
	for _, o := range got.Observations {
		if o.Path == "cmd/app/main.go" && strings.Contains(o.Detail, "produces a binary") {
			t.Errorf("claimed inspected-source evidence for an omitted file: %+v", o)
		}
	}
	if !limitationsContain(got.Limitations, "cmd/app/main.go") {
		t.Errorf("omitted source was not recorded as a limitation: %v", got.Limitations)
	}
}

// A small acceptance corpus: resolved verdicts must be correct, and the
// inconclusive rate is reported separately — correctness among classified cases
// and abstention rate are different measures, never substituted for each other.
func TestAcceptanceCorpusSeparatesCorrectnessFromAbstention(t *testing.T) {
	type fixture struct {
		name string
		insp Inspection
		want apimodel.RepoKind // meaningful only when the verdict resolves
	}
	corpus := []fixture{
		{"go-backend", InspectionFromSources("Go",
			[]string{"go.mod", "cmd/api/main.go"},
			map[string]string{"go.mod": "module x\n", "cmd/api/main.go": "package main\nimport \"net/http\"\nfunc main(){http.ListenAndServe(\":80\",nil)}\n"}),
			apimodel.KindBackend},
		{"go-library", InspectionFromSources("Go",
			[]string{"go.mod", "lib.go"},
			map[string]string{"go.mod": "module x\n", "lib.go": "package x\n"}),
			apimodel.KindLibrary},
		{"ts-frontend", InspectionFromSources("TypeScript",
			[]string{"package.json", "index.html", "src/App.tsx"},
			map[string]string{"package.json": `{"dependencies":{"react":"^18"}}`}),
			apimodel.KindFrontend},
		{"rust-cli", InspectionFromSources("Rust",
			[]string{"Cargo.toml", "src/main.rs"},
			map[string]string{"Cargo.toml": "[package]\nname=\"t\"\n[dependencies]\nclap=\"4\"\n"}),
			apimodel.KindCLI},
		// Incomplete evidence: must abstain, not guess. want is ignored.
		{"unread-entry", Inspection{Language: "Go",
			Paths:       []string{"go.mod", "cmd/app/main.go"},
			Files:       []SourceFile{{Path: "go.mod", Content: "module x\n"}},
			Unavailable: []Unavailable{{Path: "cmd/app/main.go", Reason: "fetch error"}}},
			apimodel.KindOther},
		{"truncated-tree", Inspection{Language: "Go",
			Paths: []string{"go.mod", "pkg/x.go"},
			Files: []SourceFile{{Path: "go.mod", Content: "module x\n"}},
			Scope: TreeScope{Truncated: true}},
			apimodel.KindOther},
	}

	var resolved, correct, inconclusive int
	for _, f := range corpus {
		got := Classify(f.insp)
		if got.Status == VerdictInconclusive {
			inconclusive++
			continue
		}
		resolved++
		if got.Kind == f.want {
			correct++
		} else {
			t.Errorf("%s: resolved to %q, want %q", f.name, got.Kind, f.want)
		}
	}
	t.Logf("corpus: resolved=%d correct=%d inconclusive=%d (of %d)", resolved, correct, inconclusive, len(corpus))

	if resolved == 0 {
		t.Fatal("no fixture resolved; the classifier abstains on everything")
	}
	if correct != resolved {
		t.Errorf("a resolved verdict was incorrect (%d/%d correct)", correct, resolved)
	}
	if inconclusive != 2 {
		t.Errorf("inconclusive count = %d, want 2 (the two incomplete fixtures)", inconclusive)
	}
}
