package crawler

import (
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

// Slice 3a: Go signals come from the parsed program, not from substrings. A
// server call or a dependency mentioned only in a comment must not count, while
// the real construct still does.

func TestGoStartsHTTPServerIgnoresComments(t *testing.T) {
	commented := "package main\n// ListenAndServe would be used for a future web mode.\nfunc main() {}\n"
	if starts, parsed := goStartsHTTPServer(commented); starts {
		t.Errorf("a commented ListenAndServe was read as a server call (parsed=%v)", parsed)
	}

	real := "package main\nimport \"net/http\"\nfunc main() { http.ListenAndServe(\":8000\", nil) }\n"
	if starts, _ := goStartsHTTPServer(real); !starts {
		t.Errorf("a real http.ListenAndServe call was not detected")
	}

	stringLit := "package main\nfunc main() { _ = \"ListenAndServe\" }\n"
	if starts, _ := goStartsHTTPServer(stringLit); starts {
		t.Errorf("a string literal \"ListenAndServe\" was read as a server call")
	}
}

func TestGoModRequiresIgnoresComments(t *testing.T) {
	commented := "module example.com/tool\n// migration note: no longer uses github.com/gin-gonic/gin\n"
	if dep := goRequiresAny(commented, goServerDeps); dep != "" {
		t.Errorf("a dependency named only in a comment was treated as required: %q", dep)
	}

	real := "module example.com/svc\n\nrequire github.com/gin-gonic/gin v1.9.1\n"
	if dep := goRequiresAny(real, goServerDeps); dep != "gin-gonic/gin" {
		t.Errorf("goRequiresAny = %q, want the real gin requirement", dep)
	}
}

// At the classifier level, the two commented cases resolve to CLI (a plain
// command), and their real counterparts to backend — so rejecting the false
// positive did not simply disable the detector.
func TestGoCommentsDoNotCreateBackend(t *testing.T) {
	cases := []struct {
		name  string
		paths []string
		files map[string]string
		want  apimodel.RepoKind
	}{
		{
			name:  "commented ListenAndServe is a plain command",
			paths: []string{"go.mod", "cmd/tool/main.go"},
			files: map[string]string{
				"go.mod":           "module example.com/tool\n",
				"cmd/tool/main.go": "package main\n// ListenAndServe would be used for a future web mode.\nfunc main() {}\n",
			},
			want: apimodel.KindCLI,
		},
		{
			name:  "dependency named only in a comment is not a dependency",
			paths: []string{"go.mod", "cmd/tool/main.go"},
			files: map[string]string{
				"go.mod":           "module example.com/tool\n// migration note: no longer uses github.com/gin-gonic/gin\n",
				"cmd/tool/main.go": "package main\nfunc main() {}\n",
			},
			want: apimodel.KindCLI,
		},
		{
			name:  "real ListenAndServe under cmd/ is a backend",
			paths: []string{"go.mod", "cmd/caddy/main.go"},
			files: map[string]string{
				"go.mod":            "module example.com/caddy\n",
				"cmd/caddy/main.go": "package main\nimport \"net/http\"\nfunc main() { http.ListenAndServe(\":2015\", nil) }\n",
			},
			want: apimodel.KindBackend,
		},
		{
			name:  "real gin requirement with a command is a backend",
			paths: []string{"go.mod", "cmd/api/main.go"},
			files: map[string]string{
				"go.mod":          "module example.com/api\n\nrequire github.com/gin-gonic/gin v1.9.1\n",
				"cmd/api/main.go": "package main\nfunc main() {}\n",
			},
			want: apimodel.KindBackend,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Classify(InspectionFromSources("Go", c.paths, c.files))
			if got.Kind != c.want {
				t.Errorf("Kind = %q, want %q (evidence: %v)", got.Kind, c.want, got.Evidence())
			}
		})
	}
}
