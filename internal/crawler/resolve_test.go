package crawler

import (
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

// Slice 4: the primary kind is resolved by component scope, not by first-match.
// A nested admin UI does not override a root backend; an example server does not
// promote a root library; and a decisive-but-uninspected primary entry abstains.

func TestBackendWithAdminUIResolvesBackend(t *testing.T) {
	insp := InspectionFromSources("Go",
		[]string{"go.mod", "main.go", "ui/package.json", "ui/index.html"},
		map[string]string{
			"go.mod":          "module example.com/app\n",
			"main.go":         "package main\nimport \"net/http\"\nfunc main() { http.ListenAndServe(\":8090\", nil) }\n",
			"ui/package.json": `{"name":"admin","dependencies":{"react":"^18.2.0"}}`,
		},
	)
	got := Classify(insp)
	if got.Kind != apimodel.KindBackend {
		t.Fatalf("Kind = %q, want %q (evidence: %v)", got.Kind, apimodel.KindBackend, got.Evidence())
	}
	// The UI remains attributable to its component.
	if !limitationsContain(got.Limitations, "ui/package.json") {
		t.Errorf("the admin UI was not recorded as a secondary component: %v", got.Limitations)
	}
}

func TestRootLibraryWithExampleServerStaysLibrary(t *testing.T) {
	insp := InspectionFromSources("Go",
		[]string{"go.mod", "client.go", "examples/server/main.go"},
		map[string]string{
			"go.mod":                  "module example.com/client\n",
			"client.go":               "package client\n",
			"examples/server/main.go": "package main\nimport \"net/http\"\nfunc main() { http.ListenAndServe(\":8000\", nil) }\n",
		},
	)
	got := Classify(insp)
	if got.Kind != apimodel.KindLibrary {
		t.Errorf("Kind = %q, want %q (an example server must not promote to backend; evidence: %v)",
			got.Kind, apimodel.KindLibrary, got.Evidence())
	}
}

func TestRemovingDecisiveBackendSourceBecomesAmbiguous(t *testing.T) {
	// Same shape as the backend+UI fixture, but the Go entry source was never
	// read — so the only inspected signal is the UI. The verdict must abstain
	// rather than mislabel a backend's admin UI as the product.
	insp := Inspection{
		Language: "Go",
		Paths:    []string{"go.mod", "cmd/app/main.go", "ui/package.json", "ui/index.html"},
		Files: []SourceFile{
			{Path: "go.mod", Content: "module example.com/app\n"},
			{Path: "ui/package.json", Content: `{"name":"admin","dependencies":{"react":"^18.2.0"}}`},
		},
		Unavailable: []Unavailable{{Path: "cmd/app/main.go", Reason: "fetch error"}},
	}
	got := Classify(insp)
	if got.Status != VerdictInconclusive {
		t.Errorf("Status = %q, want %q when the decisive backend source was not inspected (kind %q)",
			got.Status, VerdictInconclusive, got.Kind)
	}
	if got.Kind == apimodel.KindFrontend {
		t.Errorf("mislabelled a backend's admin UI as the product (frontend)")
	}
}
