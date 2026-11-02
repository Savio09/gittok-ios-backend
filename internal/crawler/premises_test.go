package crawler

import (
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

// Task 1 (R1): an absence claim needs its scope to have been inspected. Each
// case pairs the reviewer's boundary control — partial evidence that must not
// resolve to the unsupported kind, and must name the missing path or region —
// with the complete counterpart that must still resolve. The pairing is what
// shows the repair withholds unsupported premises rather than disabling a
// detector.
func TestAbsencePremisesNeedInspectedScope(t *testing.T) {
	truncate := func(p string) func(*Inspection) {
		return func(i *Inspection) {
			for n := range i.Files {
				if i.Files[n].Path == p {
					i.Files[n].Truncated, i.Files[n].Size = true, 100000
				}
			}
		}
	}
	cases := []struct {
		name      string
		language  string
		paths     []string
		sources   map[string]string
		partial   func(*Inspection) // the reviewer's boundary mutation (nil: sources already partial)
		forbidden apimodel.RepoKind // must not resolve under partial evidence
		mention   string            // a limitation must name this missing path or region
		complete  map[string]string // sources that make the evidence sufficient
		want      apimodel.RepoKind // ... and the supported verdict they yield
	}{
		{
			name:      "unavailable root manifest cannot promote an example",
			language:  "TypeScript",
			paths:     []string{"package.json", "examples/mobile/package.json"},
			sources:   map[string]string{"examples/mobile/package.json": `{"dependencies":{"expo":"51"}}`},
			partial:   func(i *Inspection) { i.Unavailable = []Unavailable{{Path: "package.json", Reason: "fetch error"}} },
			forbidden: apimodel.KindMobile,
			mention:   "package.json could not be read (fetch error)",
			complete: map[string]string{
				"package.json":                 `{"name":"core","main":"dist/index.js"}`,
				"examples/mobile/package.json": `{"dependencies":{"expo":"51"}}`,
			},
			want: apimodel.KindLibrary,
		},
		{
			name:      "clipped Go entry is not proof of no server",
			language:  "Go",
			paths:     []string{"go.mod", "cmd/api/main.go"},
			sources:   map[string]string{"go.mod": "module example.com/api\n", "cmd/api/main.go": "package main\n"},
			partial:   truncate("cmd/api/main.go"),
			forbidden: apimodel.KindCLI,
			mention:   "cmd/api/main.go was read only partially",
			complete:  map[string]string{"go.mod": "module example.com/api\n", "cmd/api/main.go": "package main\n\nfunc main() {}\n"},
			want:      apimodel.KindCLI,
		},
		{
			name:      "unparseable Go entry is not proof of no server",
			language:  "Go",
			paths:     []string{"go.mod", "cmd/api/main.go"},
			sources:   map[string]string{"go.mod": "module example.com/api\n", "cmd/api/main.go": "package main\nfunc main() {\n"},
			forbidden: apimodel.KindCLI,
			mention:   "cmd/api/main.go could not be parsed",
			complete:  map[string]string{"go.mod": "module example.com/api\n", "cmd/api/main.go": "package main\nfunc main() {}\n"},
			want:      apimodel.KindCLI,
		},
		{
			name:      "truncated tree cannot prove a Rust crate has no binary",
			language:  "Rust",
			paths:     []string{"Cargo.toml", "src/lib.rs"},
			sources:   map[string]string{"Cargo.toml": "[package]\nname='core'\nversion='0.1.0'\n"},
			partial:   func(i *Inspection) { i.Scope.Truncated = true },
			forbidden: apimodel.KindLibrary,
			mention:   "binary target cannot be ruled out",
			complete:  map[string]string{"Cargo.toml": "[package]\nname='core'\nversion='0.1.0'\n"},
			want:      apimodel.KindLibrary,
		},
		{
			name:      "clipped pyproject cannot prove no scripts",
			language:  "Python",
			paths:     []string{"pyproject.toml"},
			sources:   map[string]string{"pyproject.toml": "[project]\nname='tool'\nversion='0.1'\n"},
			partial:   truncate("pyproject.toml"),
			forbidden: apimodel.KindLibrary,
			mention:   "pyproject.toml was read only partially",
			complete:  map[string]string{"pyproject.toml": "[project]\nname='tool'\nversion='0.1'\n"},
			want:      apimodel.KindLibrary,
		},
		{
			name:      "listing an unread Go file is not proof of no package main",
			language:  "Go",
			paths:     []string{"go.mod", "app.go"},
			sources:   map[string]string{"go.mod": "module example.com/app\n"},
			forbidden: apimodel.KindLibrary,
			mention:   "app.go",
			complete:  map[string]string{"go.mod": "module example.com/app\n", "app.go": "package app\n"},
			want:      apimodel.KindLibrary,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			insp := InspectionFromSources(c.language, c.paths, c.sources)
			if c.partial != nil {
				c.partial(&insp)
			}
			got := Classify(insp)
			if got.Kind == c.forbidden && got.Status == VerdictResolved {
				t.Errorf("partial evidence resolved the unsupported kind %q: %v", got.Kind, got.Evidence())
			}
			if !limitationsContain(got.Limitations, c.mention) {
				t.Errorf("no limitation names %q: %v", c.mention, got.Limitations)
			}

			full := Classify(InspectionFromSources(c.language, c.paths, c.complete))
			if full.Kind != c.want || full.Status != VerdictResolved {
				t.Errorf("complete counterpart = %q/%q, want %q/resolved: %v", full.Kind, full.Status, c.want, full.Evidence())
			}
		})
	}
}

// A positive fact read from a partial file is still a fact: a clipped manifest
// that already shows [project.scripts], or a clipped entry that already shows a
// server call, keeps its supported verdict.
func TestPartialFilesKeepTheirPositiveFacts(t *testing.T) {
	scripts := InspectionFromSources("Python", []string{"pyproject.toml"},
		map[string]string{"pyproject.toml": "[project]\nname='tool'\n[project.scripts]\ntool='tool:main'\n"})
	scripts.Files[0].Truncated, scripts.Files[0].Size = true, 100000
	if got := Classify(scripts); got.Kind != apimodel.KindCLI || got.Status != VerdictResolved {
		t.Errorf("clipped pyproject with [project.scripts]: %q/%q, want cli/resolved", got.Kind, got.Status)
	}

	server := InspectionFromSources("Go", []string{"go.mod", "cmd/api/main.go"}, map[string]string{
		"go.mod":          "module example.com/api\n",
		"cmd/api/main.go": "package main\nimport \"net/http\"\nfunc main() { http.ListenAndServe(\":80\", nil) }\n",
	})
	for n := range server.Files {
		if server.Files[n].Path == "cmd/api/main.go" {
			server.Files[n].Truncated, server.Files[n].Size = true, 100000
		}
	}
	if got := Classify(server); got.Kind != apimodel.KindBackend || got.Status != VerdictResolved {
		t.Errorf("clipped entry with a server call: %q/%q, want backend/resolved", got.Kind, got.Status)
	}
}

// A command that imports a package from its own module cannot be called a CLI by
// absence: the import is not followed, so a server there cannot be ruled out.
func TestNoServerPremiseDoesNotFollowOwnModuleImports(t *testing.T) {
	got := Classify(InspectionFromSources("Go", []string{"go.mod", "main.go", "internal/srv/srv.go"}, map[string]string{
		"go.mod":  "module example.com/app\n",
		"main.go": "package main\nimport \"example.com/app/internal/srv\"\nfunc main() { srv.Run() }\n",
	}))
	if got.Kind == apimodel.KindCLI && got.Status == VerdictResolved {
		t.Errorf("concluded CLI though the command's own-module import was not followed: %v", got.Evidence())
	}
	if !limitationsContain(got.Limitations, "example.com/app/internal/srv") {
		t.Errorf("limitation should name the unfollowed import: %v", got.Limitations)
	}
}
