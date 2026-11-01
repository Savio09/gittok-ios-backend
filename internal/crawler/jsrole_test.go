package crawler

import (
	"strings"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

// Slice 5: a build-tool dev dependency or a dev script does not make a package a
// frontend; a demo does not make a library one. Runtime framework + a primary
// app entry does.
func TestJSToolingVsProduct(t *testing.T) {
	cases := []struct {
		name  string
		paths []string
		files map[string]string
		want  apimodel.RepoKind
	}{
		{
			name:  "npm CLI with a bin and a vite build script is a CLI",
			paths: []string{"package.json", "src/cli.ts"},
			files: map[string]string{
				"package.json": `{"name":"tool","bin":{"tool":"dist/cli.js"},"devDependencies":{"vite":"^5"},"scripts":{"dev":"vite build --watch"}}`,
			},
			want: apimodel.KindCLI,
		},
		{
			name:  "published UI library with a vite demo is a library, not a frontend",
			paths: []string{"package.json", "src/index.ts", "demo/index.html", "demo/main.tsx"},
			files: map[string]string{
				"package.json": `{"name":"ui-kit","main":"dist/index.js","module":"dist/index.mjs","peerDependencies":{"react":"^18"},"devDependencies":{"vite":"^5"}}`,
			},
			want: apimodel.KindLibrary,
		},
		{
			name:  "genuine react/vite app is still a frontend",
			paths: []string{"index.html", "package.json", "vite.config.ts", "src/App.tsx", "src/main.tsx"},
			files: map[string]string{
				"package.json": `{"name":"webapp","private":true,"dependencies":{"react":"^18.2.0","react-dom":"^18.2.0"},"devDependencies":{"vite":"^5"},"scripts":{"dev":"vite"}}`,
			},
			want: apimodel.KindFrontend,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Classify(InspectionFromSources("TypeScript", c.paths, c.files))
			if got.Kind != c.want {
				t.Errorf("Kind = %q, want %q (evidence: %v)", got.Kind, c.want, got.Evidence())
			}
		})
	}
}

// Task 5 (R4): the dependency table a framework is declared in survives into the
// role decision for mobile and backend too. A development or peer declaration
// describes tooling or an integration; only a runtime one establishes the role.
func TestJSCategoryGovernsMobileAndBackend(t *testing.T) {
	cases := []struct {
		name  string
		pkg   string
		paths []string
		want  apimodel.RepoKind
		cites string // a primary observation must cite this field
		notes string // a limitation must name the non-runtime table, when set
	}{
		{
			name: "dev-only express in an npm CLI is not a backend (R4)", pkg: `{"bin":"bin/tool.js","devDependencies":{"express":"5"}}`,
			paths: []string{"package.json", "bin/tool.js"}, want: apimodel.KindCLI,
			cites: `declares a "bin" field`, notes: "only in devDependencies",
		},
		{
			name: "peer-only expo in an exported library is not mobile (R4)", pkg: `{"exports":"./dist/index.js","peerDependencies":{"expo":"51"}}`,
			paths: []string{"package.json", "src/index.ts"}, want: apimodel.KindLibrary,
			cites: "lib entry", notes: "only in peerDependencies",
		},
		{
			name: "runtime express is a backend", pkg: `{"name":"api","dependencies":{"express":"^4.19.0"}}`,
			paths: []string{"package.json", "server.js"}, want: apimodel.KindBackend,
			cites: `declares "express" (^4.19.0) in dependencies`,
		},
		{
			name: "runtime expo is a mobile app", pkg: `{"name":"app","dependencies":{"expo":"~51.0.0","react":"18.2.0","react-native":"0.74.0"}}`,
			paths: []string{"package.json", "App.tsx"}, want: apimodel.KindMobile,
			cites: `declares "expo" (~51.0.0) in dependencies`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Classify(InspectionFromSources("TypeScript", c.paths, map[string]string{"package.json": c.pkg}))
			if got.Kind != c.want || got.Status != VerdictResolved {
				t.Fatalf("got %q/%q, want %q/resolved: %v", got.Kind, got.Status, c.want, got.Evidence())
			}
			cited := false
			for _, o := range got.Observations {
				if o.Path == "package.json" && o.Component == "" && o.Role == c.want && strings.Contains(o.Detail, c.cites) {
					cited = true
				}
			}
			if !cited {
				t.Errorf("no primary observation cites %q with its owner and role: %+v", c.cites, got.Observations)
			}
			if c.notes != "" && !limitationsContain(got.Limitations, c.notes) {
				t.Errorf("the non-runtime declaration should be surfaced with its table (%q): %v", c.notes, got.Limitations)
			}
		})
	}
}
