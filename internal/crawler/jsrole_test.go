package crawler

import (
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
