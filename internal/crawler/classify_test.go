package crawler

import (
	"strings"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

// TestClassify covers one representative repository per kind, plus the
// precedence cases the ordering exists to get right (an Expo app that also
// depends on react is mobile, not frontend; a Go server under cmd/ is backend,
// not cli; an importable Go module with no main is a library, not a backend).
// Each case asserts both the Kind and that the decisive evidence is recorded.
func TestClassify(t *testing.T) {
	cases := []struct {
		name         string
		language     string
		paths        []string
		manifests    map[string]string
		wantKind     apimodel.RepoKind
		wantEvidence string // substring that must appear in some Evidence entry
	}{
		{
			name:     "react vite app is frontend",
			language: "TypeScript",
			paths:    []string{"index.html", "package.json", "vite.config.ts", "src/App.tsx", "src/main.tsx"},
			manifests: map[string]string{
				"package.json": `{
					"name": "webapp",
					"private": true,
					"dependencies": {"react": "^18.2.0", "react-dom": "^18.2.0"},
					"devDependencies": {"vite": "^5.0.0"},
					"scripts": {"dev": "vite", "build": "vite build"}
				}`,
			},
			wantKind:     apimodel.KindFrontend,
			wantEvidence: `"react"`,
		},
		{
			name:     "importable go module with no main is a library (gin)",
			language: "Go",
			paths:    []string{"go.mod", "gin.go", "context.go", "routergroup.go", "errors.go"},
			manifests: map[string]string{
				"go.mod": "module github.com/gin-gonic/gin\n\ngo 1.21\n\nrequire github.com/goccy/go-json v0.10.2\n",
				// The library verdict rests on an inspected package clause, not on
				// the mere listing of gin.go.
				"gin.go": "package gin\n",
			},
			wantKind:     apimodel.KindLibrary,
			wantEvidence: "no package main",
		},
		{
			name:     "go command with cobra is a cli (gum)",
			language: "Go",
			paths:    []string{"go.mod", "main.go", "commands/root.go", "style/style.go"},
			manifests: map[string]string{
				"go.mod":  "module github.com/charmbracelet/gum\n\ngo 1.21\n\nrequire (\n\tgithub.com/spf13/cobra v1.8.0\n)\n",
				"main.go": "package main\n\nimport \"github.com/charmbracelet/gum/commands\"\n\nfunc main() { commands.Execute() }\n",
			},
			wantKind:     apimodel.KindCLI,
			wantEvidence: "spf13/cobra",
		},
		{
			name:     "go http server is a backend even under cmd/ (caddy)",
			language: "Go",
			paths:    []string{"go.mod", "cmd/caddy/main.go", "caddy.go", "modules/caddyhttp/app.go"},
			manifests: map[string]string{
				"go.mod":            "module github.com/caddyserver/caddy/v2\n\ngo 1.21\n",
				"cmd/caddy/main.go": "package main\n\nimport \"net/http\"\n\nfunc main() {\n\thttp.ListenAndServe(\":2015\", nil)\n}\n",
			},
			wantKind:     apimodel.KindBackend,
			wantEvidence: "ListenAndServe",
		},
		{
			name:     "expo react-native app is mobile not frontend (plantly)",
			language: "TypeScript",
			paths:    []string{"package.json", "app.json", "App.tsx", "assets/icon.png"},
			manifests: map[string]string{
				"package.json": `{
					"name": "plantly",
					"dependencies": {"expo": "~51.0.0", "react": "18.2.0", "react-native": "0.74.0"}
				}`,
			},
			wantKind:     apimodel.KindMobile,
			wantEvidence: "react-native",
		},
		{
			name:         "empty repo is other",
			language:     "",
			paths:        nil,
			manifests:    nil,
			wantKind:     apimodel.KindOther,
			wantEvidence: "no decisive",
		},
		// --- extra ecosystem coverage ---------------------------------------
		{
			name:     "npm package with lib entry and no app is a library",
			language: "TypeScript",
			paths:    []string{"package.json", "src/index.ts", "dist/index.js"},
			manifests: map[string]string{
				"package.json": `{
					"name": "tiny-lib",
					"main": "dist/index.js",
					"module": "dist/index.mjs",
					"peerDependencies": {"react": "^18.0.0"}
				}`,
			},
			wantKind:     apimodel.KindLibrary,
			wantEvidence: "lib entry",
		},
		{
			name:     "python flask app is a backend",
			language: "Python",
			paths:    []string{"pyproject.toml", "app/main.py"},
			manifests: map[string]string{
				"pyproject.toml": "[project]\nname = \"svc\"\ndependencies = [\"flask>=3.0\"]\n",
			},
			wantKind:     apimodel.KindBackend,
			wantEvidence: "flask",
		},
		{
			name:     "flutter pubspec is mobile",
			language: "Dart",
			paths:    []string{"pubspec.yaml", "lib/main.dart", "android/build.gradle", "ios/Runner.xcodeproj"},
			manifests: map[string]string{
				"pubspec.yaml": "name: myapp\nenvironment:\n  sdk: '>=3.0.0'\ndependencies:\n  flutter:\n    sdk: flutter\n",
			},
			wantKind:     apimodel.KindMobile,
			wantEvidence: "Flutter",
		},
		{
			name:     "a backend keeps its Dockerfile port as deployment evidence",
			language: "Go",
			paths:    []string{"Dockerfile", "go.mod", "cmd/app/main.go"},
			manifests: map[string]string{
				"go.mod": "module example.com/app\n\ngo 1.21\n",
				// EXPOSE alone never decides the role (R5): the server call does, and
				// the declared port is kept beside it as deployment evidence.
				"cmd/app/main.go": "package main\n\nimport \"net/http\"\n\nfunc main() { http.ListenAndServe(\":8080\", nil) }\n",
				"Dockerfile":      "FROM golang:1.21\nEXPOSE 8080\nCMD [\"/app\"]\n",
			},
			wantKind:     apimodel.KindBackend,
			wantEvidence: "EXPOSE 8080",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Classify(InspectionFromSources(c.language, c.paths, c.manifests))
			evidence := got.Evidence()
			if got.Kind != c.wantKind {
				t.Errorf("Kind = %q, want %q (evidence: %v)", got.Kind, c.wantKind, evidence)
			}
			if !evidenceContains(evidence, c.wantEvidence) {
				t.Errorf("evidence %v, want an entry containing %q", evidence, c.wantEvidence)
			}
			if len(evidence) == 0 {
				t.Errorf("Evidence is empty; every classification must name its signals")
			}
		})
	}
}

// TestClassifyKindIsValid guards that Classify only ever returns a known kind.
func TestClassifyKindIsValid(t *testing.T) {
	got := Classify(InspectionFromSources("Go", []string{"go.mod", "lib.go"}, map[string]string{"go.mod": "module x\n"}))
	if !got.Kind.Valid() {
		t.Errorf("Kind %q is not a valid apimodel.RepoKind", got.Kind)
	}
}

func evidenceContains(evidence []string, substr string) bool {
	for _, e := range evidence {
		if strings.Contains(e, substr) {
			return true
		}
	}
	return false
}
