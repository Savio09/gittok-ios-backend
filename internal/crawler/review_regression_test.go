package crawler

import (
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

// Checkpoint B: the eleven semantic counterexamples from the 2026-10-30
// classification review (docs/reviews/2026-10-30-classification), preserved as
// maintained regressions. Each originally mis-classified; each is now resolved
// (or, where the archived label was itself only defensible as an abstention,
// resolves to KindOther with an explanation — noted per case). True-positive
// counterparts live in the per-feature test files (goanalysis_test, pyanalysis_test,
// rustrole_test, jsrole_test, …) so rejecting a false positive never just
// disables a detector.
func TestReviewCounterexamples(t *testing.T) {
	cases := []struct {
		name     string
		language string
		paths    []string
		files    map[string]string
		want     apimodel.RepoKind
		note     string
	}{
		{
			name: "rust_implicit_cli", language: "Rust",
			paths: []string{"Cargo.toml", "src/main.rs"},
			files: map[string]string{"Cargo.toml": "[package]\nname = \"tool\"\nversion = \"0.1.0\"\n[dependencies]\nclap = \"4\"\n"},
			want:  apimodel.KindCLI, note: "implicit binary target + clap",
		},
		{
			name: "rust_implicit_library", language: "Rust",
			paths: []string{"Cargo.toml", "src/lib.rs"},
			files: map[string]string{"Cargo.toml": "[package]\nname = \"math\"\nversion = \"0.1.0\"\n"},
			want:  apimodel.KindLibrary, note: "implicit library target",
		},
		{
			name: "python_modern_cli", language: "Python",
			paths: []string{"pyproject.toml", "src/tool.py"},
			files: map[string]string{"pyproject.toml": "[project]\nname = \"tool\"\nversion = \"0.1.0\"\n[project.scripts]\ntool = \"tool:main\"\n"},
			want:  apimodel.KindCLI, note: "[project.scripts] command entry",
		},
		{
			name: "python_requirements_backend", language: "Python",
			paths: []string{"requirements.txt", "app.py"},
			files: map[string]string{"requirements.txt": "Flask==3.0.0\n", "app.py": "from flask import Flask\napp = Flask(__name__)\n"},
			want:  apimodel.KindBackend, note: "flask declared in requirements.txt",
		},
		{
			name: "python_tooling_is_not_library", language: "Python",
			paths: []string{"pyproject.toml", "script.py"},
			files: map[string]string{"pyproject.toml": "[tool.ruff]\nline-length = 88\n"},
			want:  apimodel.KindOther, note: "tool config only: abstain, not a library",
		},
		{
			name: "python_project_name_is_not_dependency", language: "Python",
			paths: []string{"pyproject.toml"},
			files: map[string]string{"pyproject.toml": "[project]\nname = \"flask-utils\"\nversion = \"0.1.0\"\ndependencies = []\n"},
			want:  apimodel.KindLibrary, note: "name is not a flask dependency",
		},
		{
			name: "npm_cli_with_vite_build_tool", language: "TypeScript",
			paths: []string{"package.json", "src/cli.ts"},
			files: map[string]string{"package.json": `{"name":"tool","bin":{"tool":"dist/cli.js"},"devDependencies":{"vite":"^5"},"scripts":{"dev":"vite build --watch"}}`},
			want:  apimodel.KindCLI, note: "vite devDependency is a build tool, not a frontend",
		},
		{
			name: "go_comment_is_not_http_server", language: "Go",
			paths: []string{"go.mod", "cmd/tool/main.go"},
			files: map[string]string{"go.mod": "module example.com/tool\n", "cmd/tool/main.go": "package main\n// ListenAndServe would be used for a future web mode.\nfunc main() {}\n"},
			want:  apimodel.KindCLI, note: "commented ListenAndServe is not a server call",
		},
		{
			name: "go_dependency_comment_is_not_dependency", language: "Go",
			paths: []string{"go.mod", "cmd/tool/main.go"},
			files: map[string]string{"go.mod": "module example.com/tool\n// migration note: no longer uses github.com/gin-gonic/gin\n", "cmd/tool/main.go": "package main\nfunc main() {}\n"},
			want:  apimodel.KindCLI, note: "commented dependency is not required",
		},
		{
			name: "flutter_comment_is_not_sdk", language: "Dart",
			paths: []string{"pubspec.yaml", "lib/math.dart"},
			files: map[string]string{"pubspec.yaml": "name: math\n# No flutter dependency; ordinary Dart library.\nenvironment:\n  sdk: ^3.0.0\n"},
			want:  apimodel.KindOther, note: "commented flutter is not an SDK dependency; plain Dart lib is outside the kind taxonomy",
		},
		{
			name: "go_library_example_is_not_main_product", language: "Go",
			paths: []string{"go.mod", "client.go", "examples/server/main.go"},
			files: map[string]string{"go.mod": "module example.com/client\n", "client.go": "package client\n", "examples/server/main.go": "package main\nimport \"net/http\"\nfunc main(){http.ListenAndServe(\":8000\",nil)}\n"},
			want:  apimodel.KindLibrary, note: "example server does not promote the root library",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Classify(InspectionFromSources(c.language, c.paths, c.files))
			if got.Kind != c.want {
				t.Errorf("Kind = %q, want %q (%s)\nevidence: %v", got.Kind, c.want, c.note, got.Evidence())
			}
		})
	}
}
