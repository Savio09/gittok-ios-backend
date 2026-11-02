package crawler

import (
	"reflect"
	"strings"
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
	// The UI remains attributable to its own component.
	attributed := false
	for _, o := range got.Secondary {
		if o.Component == "ui" && o.Role == apimodel.KindFrontend && o.Path == "ui/package.json" {
			attributed = true
		}
	}
	if !attributed {
		t.Errorf("the admin UI was not kept as secondary evidence of component ui: %+v", got.Secondary)
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

// Task 3 (R2): the two reviewer fixtures. An independent nested app or module is
// a secondary component of the root product, never its replacement, and its
// evidence stays attributable to it.
func TestNestedComponentsDoNotReplaceTheRootProduct(t *testing.T) {
	cases := []struct {
		name, language, nested string
		paths                  []string
		sources                map[string]string
		forbidden              apimodel.RepoKind
		nestedRole             apimodel.RepoKind
	}{
		{
			name: "nested ui app beside a root React library", language: "TypeScript", nested: "ui",
			paths: []string{"package.json", "src/index.ts", "ui/package.json", "ui/index.html"},
			sources: map[string]string{
				"package.json":    `{"exports":"./dist/index.js","dependencies":{"react":"18"}}`,
				"ui/package.json": `{"private":true,"dependencies":{"react":"18"}}`,
			},
			forbidden: apimodel.KindFrontend, nestedRole: apimodel.KindFrontend,
		},
		{
			name: "separate demo module beside a root Go library", language: "Go", nested: "packages/demo",
			paths: []string{"go.mod", "lib.go", "packages/demo/go.mod", "packages/demo/main.go"},
			sources: map[string]string{
				"go.mod": "module example.com/lib\n", "lib.go": "package lib\n",
				"packages/demo/go.mod":  "module example.com/demo\n",
				"packages/demo/main.go": "package main\nimport \"net/http\"\nfunc main(){http.ListenAndServe(\":8080\",nil)}\n",
			},
			forbidden: apimodel.KindBackend, nestedRole: apimodel.KindBackend,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Classify(InspectionFromSources(c.language, c.paths, c.sources))
			if got.Kind != apimodel.KindLibrary || got.Status != VerdictResolved {
				t.Fatalf("got %q/%q, want the root library: %v", got.Kind, got.Status, got.Evidence())
			}
			for _, o := range got.Observations {
				if o.Component != "" {
					t.Errorf("primary evidence borrowed from component %q: %+v", o.Component, o)
				}
			}
			kept := false
			for _, o := range got.Secondary {
				if o.Component == c.nested && o.Role == c.nestedRole {
					kept = true
				}
			}
			if !kept {
				t.Errorf("the nested %s evidence was not kept for component %q: %+v", c.nestedRole, c.nested, got.Secondary)
			}
		})
	}
}

// With no product at the root, nested components are compared: disagreeing
// roles, or an unresolved candidate, abstain with an explanation rather than
// letting detector order pick a winner; agreeing roles resolve.
func TestNestedCandidatesWithoutARootProduct(t *testing.T) {
	disagree := Classify(InspectionFromSources("", []string{"api/go.mod", "api/main.go", "web/package.json", "web/index.html"},
		map[string]string{
			"api/go.mod":       "module example.com/api\n",
			"api/main.go":      "package main\nimport \"net/http\"\nfunc main(){http.ListenAndServe(\":80\",nil)}\n",
			"web/package.json": `{"private":true,"dependencies":{"react":"18"}}`,
		}))
	if disagree.Status != VerdictInconclusive || !limitationsContain(disagree.Limitations, "different roles") {
		t.Errorf("independent backend + frontend: got %q/%q, want an explained abstention: %v", disagree.Kind, disagree.Status, disagree.Limitations)
	}
	if len(disagree.Secondary) == 0 {
		t.Errorf("an abstention must still list its candidates")
	}

	unresolved := Classify(Inspection{
		Paths: []string{"svc/go.mod", "svc/main.go"},
		Files: []SourceFile{{Path: "svc/go.mod", Content: "module example.com/svc\n"}},
	})
	if unresolved.Status != VerdictInconclusive || !limitationsContain(unresolved.Limitations, "component svc could not be resolved") {
		t.Errorf("unresolved nested candidate: got %q/%q: %v", unresolved.Kind, unresolved.Status, unresolved.Limitations)
	}

	agree := Classify(InspectionFromSources("", []string{"a/Cargo.toml", "a/src/main.rs", "b/Cargo.toml", "b/src/main.rs"},
		map[string]string{
			"a/Cargo.toml": "[package]\nname='a'\n[dependencies]\nclap='4'\n",
			"b/Cargo.toml": "[package]\nname='b'\n[dependencies]\nclap='4'\n",
		}))
	if agree.Kind != apimodel.KindCLI || agree.Status != VerdictResolved {
		t.Errorf("two CLI crates: got %q/%q, want cli/resolved: %v", agree.Kind, agree.Status, agree.Evidence())
	}
}

// A workspace root aggregates rather than ships; members that another member
// depends on are its supporting libraries; the remaining member is the product.
func TestWorkspaceAppIsTheProductOverItsLibraries(t *testing.T) {
	got := Classify(InspectionFromSources("TypeScript",
		[]string{"package.json", "app/package.json", "app/index.html", "packages/ui/package.json", "examples/demo/package.json", "examples/demo/index.html"},
		map[string]string{
			"package.json":               `{"private":true,"workspaces":["app","packages/*","examples/*"]}`,
			"app/package.json":           `{"name":"app","private":true,"dependencies":{"react":"18","@acme/ui":"*"}}`,
			"packages/ui/package.json":   `{"name":"@acme/ui","exports":"./dist/index.js","peerDependencies":{"react":"18"}}`,
			"examples/demo/package.json": `{"private":true,"dependencies":{"react":"18"}}`,
		}))
	if got.Kind != apimodel.KindFrontend || got.Status != VerdictResolved {
		t.Fatalf("got %q/%q, want the app's frontend: %v", got.Kind, got.Status, got.Evidence())
	}
	if len(got.Observations) == 0 || got.Observations[0].Component != "app" {
		t.Errorf("primary evidence should come from component app: %+v", got.Observations)
	}
}

// Removing the decisive primary evidence changes the decision, while the valid
// secondary evidence of another component is retained.
func TestRemovingPrimaryEvidenceKeepsSecondaryEvidence(t *testing.T) {
	paths := []string{"go.mod", "cmd/app/main.go", "ui/package.json", "ui/index.html"}
	complete := Classify(InspectionFromSources("Go", paths, map[string]string{
		"go.mod":          "module example.com/app\n",
		"cmd/app/main.go": "package main\nimport \"net/http\"\nfunc main(){http.ListenAndServe(\":80\",nil)}\n",
		"ui/package.json": `{"private":true,"dependencies":{"react":"18"}}`,
	}))
	if complete.Kind != apimodel.KindBackend || complete.Status != VerdictResolved {
		t.Fatalf("complete: got %q/%q", complete.Kind, complete.Status)
	}
	removed := Classify(InspectionFromSources("Go", paths, map[string]string{
		"go.mod":          "module example.com/app\n",
		"ui/package.json": `{"private":true,"dependencies":{"react":"18"}}`,
	}))
	if removed.Status != VerdictInconclusive || removed.Kind == apimodel.KindFrontend {
		t.Errorf("without the server source: got %q/%q, want an abstention, not the UI", removed.Kind, removed.Status)
	}
	ui := false
	for _, o := range removed.Secondary {
		if o.Component == "ui" && o.Path == "ui/package.json" {
			ui = true
		}
	}
	if !ui {
		t.Errorf("the UI's valid evidence was dropped: %+v", removed.Secondary)
	}
}

// The whole result — kind, status, observations, secondary evidence and
// limitations — is identical for any ordering of the inspection.
func TestClassificationIsOrderStableAcrossComponents(t *testing.T) {
	base := Inspection{
		Language: "Go",
		Paths:    []string{"go.mod", "lib.go", "packages/demo/go.mod", "packages/demo/main.go", "ui/package.json", "ui/index.html", "cmd/x/main.go"},
		Files: []SourceFile{
			{Path: "go.mod", Content: "module example.com/lib\n"},
			{Path: "lib.go", Content: "package lib\n"},
			{Path: "packages/demo/go.mod", Content: "module example.com/demo\n"},
			{Path: "packages/demo/main.go", Content: "package main\nimport \"net/http\"\nfunc main(){http.ListenAndServe(\":8080\",nil)}\n"},
			{Path: "ui/package.json", Content: `{"dependencies":{"react":"18"}}`},
		},
		Unavailable: []Unavailable{{Path: "cmd/x/main.go", Reason: "not found"}},
	}
	want := Classify(base)
	for i := 0; i < len(base.Paths); i++ {
		shuffled := base
		shuffled.Paths = append(append([]string(nil), base.Paths[i:]...), base.Paths[:i]...)
		shuffled.Files = append([]SourceFile(nil), base.Files...)
		for a, b := 0, len(shuffled.Files)-1; a < b; a, b = a+1, b-1 {
			shuffled.Files[a], shuffled.Files[b] = shuffled.Files[b], shuffled.Files[a]
		}
		if got := Classify(shuffled); !reflect.DeepEqual(got, want) {
			t.Fatalf("rotation %d changed the result:\n got  %+v\n want %+v", i, got, want)
		}
	}
}

// Task 7 (R5): a Dockerfile's EXPOSE is a deployment port, not a product role.
// Alone it stays inconclusive and is surfaced; beside independent backend
// evidence it is kept as supporting context. Neither wording claims the
// container was observed listening or run.
func TestDockerPortIsDeploymentEvidenceOnly(t *testing.T) {
	dockerfile := "FROM alpine\nEXPOSE 8080\n"
	alone := Classify(InspectionFromSources("", []string{"Dockerfile"}, map[string]string{"Dockerfile": dockerfile}))
	if alone.Kind == apimodel.KindBackend || alone.Status != VerdictInconclusive {
		t.Errorf("a port alone resolved %q/%q, want an abstention", alone.Kind, alone.Status)
	}
	if !limitationsContain(alone.Limitations, "Dockerfile declares EXPOSE 8080") {
		t.Errorf("the declared port should stay visible: %v", alone.Limitations)
	}

	withServer := Classify(InspectionFromSources("Go", []string{"Dockerfile", "go.mod", "main.go"}, map[string]string{
		"Dockerfile": dockerfile,
		"go.mod":     "module example.com/api\n",
		"main.go":    "package main\nimport \"net/http\"\nfunc main(){ http.ListenAndServe(\":8080\", nil) }\n",
	}))
	if withServer.Kind != apimodel.KindBackend || withServer.Status != VerdictResolved {
		t.Fatalf("independent backend + Dockerfile: got %q/%q", withServer.Kind, withServer.Status)
	}
	kept := false
	for _, o := range withServer.Observations {
		if o.Path == "Dockerfile" && strings.Contains(o.Detail, "EXPOSE 8080") {
			kept = true
		}
	}
	if !kept {
		t.Errorf("the deployment port was not kept beside the backend evidence: %+v", withServer.Observations)
	}

	for _, line := range append(alone.Evidence(), withServer.Evidence()...) {
		low := strings.ToLower(line)
		for _, claim := range []string{"serves on", "is listening", "listens on", "was run", "is running", "starts an http server"} {
			if strings.Contains(low, claim) {
				t.Errorf("evidence claims runtime behaviour (%q): %s", claim, line)
			}
		}
	}
}

// With no product at the root, one executable member beside importable
// libraries is the product (an artifact outranks a library, as within a
// component); members that are all libraries make a library; two different
// executable roles still abstain (see TestNestedCandidatesWithoutARootProduct).
func TestWorkspaceExecutableOutranksLibraries(t *testing.T) {
	appAndLibs := Classify(InspectionFromSources("TypeScript",
		[]string{"package.json", "app/package.json", "app/index.html", "packages/utils/package.json"},
		map[string]string{
			"package.json":                `{"private":true,"workspaces":["app","packages/*"]}`,
			"app/package.json":            `{"name":"app","private":true,"dependencies":{"react":"18"}}`,
			"packages/utils/package.json": `{"name":"@acme/utils","main":"dist/index.js"}`,
		}))
	if appAndLibs.Kind != apimodel.KindFrontend || appAndLibs.Status != VerdictResolved {
		t.Errorf("app + standalone library: got %q/%q, want frontend", appAndLibs.Kind, appAndLibs.Status)
	}
	kept := false
	for _, o := range appAndLibs.Secondary {
		if o.Component == "packages/utils" && o.Role == apimodel.KindLibrary {
			kept = true
		}
	}
	if !kept {
		t.Errorf("the published library should stay attributable: %+v", appAndLibs.Secondary)
	}

	libsOnly := Classify(InspectionFromSources("TypeScript",
		[]string{"package.json", "packages/a/package.json", "packages/b/package.json"},
		map[string]string{
			"package.json":            `{"private":true,"workspaces":["packages/*"]}`,
			"packages/a/package.json": `{"name":"a","main":"index.js"}`,
			"packages/b/package.json": `{"name":"b","main":"index.js"}`,
		}))
	if libsOnly.Kind != apimodel.KindLibrary || libsOnly.Status != VerdictResolved {
		t.Errorf("a library-only workspace: got %q/%q, want library", libsOnly.Kind, libsOnly.Status)
	}
}

// Comparing nested components needs a complete listing: an omitted component
// could itself be a product. A resolved root does not need it.
func TestNestedComparisonNeedsACompleteListing(t *testing.T) {
	nested := InspectionFromSources("", []string{"apps/mobile/package.json"},
		map[string]string{"apps/mobile/package.json": `{"dependencies":{"expo":"51"}}`})
	nested.Scope.Truncated = true
	if got := Classify(nested); got.Status != VerdictInconclusive || !limitationsContain(got.Limitations, "the listing is incomplete") {
		t.Errorf("nested-only over a truncated listing: got %q/%q: %v", got.Kind, got.Status, got.Limitations)
	}

	root := InspectionFromSources("", []string{"package.json", "apps/web/package.json"},
		map[string]string{"package.json": `{"dependencies":{"expo":"51"}}`, "apps/web/package.json": `{"dependencies":{"react":"18"}}`})
	root.Scope.Truncated = true
	if got := Classify(root); got.Kind != apimodel.KindMobile || got.Status != VerdictResolved {
		t.Errorf("a resolved root speaks for the repository whatever the listing omits: got %q/%q", got.Kind, got.Status)
	}
}
