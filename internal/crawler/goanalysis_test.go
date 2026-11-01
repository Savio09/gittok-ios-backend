package crawler

import (
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

// Slice 3a: Go signals come from the parsed program, not from substrings. A
// server call or a dependency mentioned only in a comment must not count, while
// the real construct still does.

// Task 4 (R3): a server call counts only when the type checker resolves it to
// net/http — through the real import, an alias or a dot import, or a receiver
// whose type is *http.Server. A same-named method on another type is not one; an
// http.Server literal is a value, not a call; and a receiver or package whose
// identity is not established is recorded as uncertain rather than guessed.
func TestServerCallIdentity(t *testing.T) {
	cases := []struct {
		name      string
		src       string
		calls     int // resolved net/http server calls
		uncertain int // server-like calls whose identity was not established
	}{
		{"direct package call", "package main\nimport \"net/http\"\nfunc main() { http.ListenAndServe(\":80\", nil) }\n", 1, 0},
		{"aliased import", "package main\nimport h \"net/http\"\nfunc main() { h.ListenAndServeTLS(\":443\", \"c\", \"k\", nil) }\n", 1, 0},
		{"dot import", "package main\nimport . \"net/http\"\nfunc main() { ListenAndServe(\":80\", nil) }\n", 1, 0},
		{"pointer literal receiver", "package main\nimport \"net/http\"\nfunc main() { srv := &http.Server{Addr: \":80\"}; srv.ListenAndServe() }\n", 1, 0},
		{"value receiver", "package main\nimport \"net/http\"\nfunc main() { var srv http.Server; srv.ListenAndServe() }\n", 1, 0},
		{"parameter typed *http.Server", "package main\nimport \"net/http\"\nfunc run(s *http.Server) { s.Serve(nil) }\nfunc main() {}\n", 1, 0},
		{"inline literal receiver", "package main\nimport \"net/http\"\nfunc main() { (&http.Server{}).ListenAndServe() }\n", 1, 0},

		{"method on a local type", "package main\ntype logger struct{}\nfunc (logger) ListenAndServe() {}\nfunc main() { logger{}.ListenAndServe() }\n", 0, 0},
		{"unused http.Server literal", "package main\nimport \"net/http\"\nfunc main() { _ = http.Server{} }\n", 0, 0},
		{"import without a call", "package main\nimport \"net/http\"\nvar _ = http.StatusOK\nfunc main() {}\n", 0, 0},
		{"comment mention", "package main\n// ListenAndServe would be used for a future web mode.\nfunc main() {}\n", 0, 0},
		{"string mention", "package main\nfunc main() { _ = \"ListenAndServe\" }\n", 0, 0},

		{"shadowed package name", "package main\nimport \"net/http\"\nfunc main() { http := struct{ ListenAndServe func() }{}; http.ListenAndServe(); _ = http }\nvar _ = http.StatusOK\n", 0, 0},
		{"receiver from another file", "package main\nfunc main() { srv := newServer(); srv.ListenAndServe() }\n", 0, 1},
		{"another package's ListenAndServe", "package main\nimport \"github.com/valyala/fasthttp\"\nfunc main() { fasthttp.ListenAndServe(\":80\", nil) }\n", 0, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := analyzeGoFile(SourceFile{Path: "main.go", Content: c.src})
			if len(g.serverCalls) != c.calls || len(g.uncertain) != c.uncertain {
				t.Errorf("calls=%d uncertain=%d, want %d/%d (calls %+v, uncertain %v)",
					len(g.serverCalls), len(g.uncertain), c.calls, c.uncertain, g.serverCalls, g.uncertain)
			}
		})
	}
}

// A resolved call keeps its construct's location in the original file.
func TestServerCallKeepsItsSpan(t *testing.T) {
	src := "package main\n\nimport \"net/http\"\n\nfunc main() {\n\thttp.ListenAndServe(\":80\", nil)\n}\n"
	g := analyzeGoFile(SourceFile{Path: "cmd/api/main.go", Content: src})
	if len(g.serverCalls) != 1 {
		t.Fatalf("calls = %+v", g.serverCalls)
	}
	c := g.serverCalls[0]
	if got := src[c.start:c.end]; got != `http.ListenAndServe(":80", nil)` || c.line != 6 {
		t.Errorf("span = %q at line %d, want the call expression at line 6", got, c.line)
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
			name:  "a same-named method on a local type is not an HTTP server (R3)",
			paths: []string{"go.mod", "cmd/tool/main.go"},
			files: map[string]string{
				"go.mod":           "module example.com/tool\n",
				"cmd/tool/main.go": "package main\ntype logger struct{}\nfunc (logger) ListenAndServe(){}\nfunc main(){ logger{}.ListenAndServe() }\n",
			},
			want: apimodel.KindCLI,
		},
		{
			name:  "an unused http.Server literal is not a start call (R3)",
			paths: []string{"go.mod", "cmd/tool/main.go"},
			files: map[string]string{
				"go.mod":           "module example.com/tool\n",
				"cmd/tool/main.go": "package main\nimport \"net/http\"\nfunc main(){ _ = http.Server{} }\n",
			},
			want: apimodel.KindCLI,
		},
		{
			name:  "an aliased net/http server call is a backend",
			paths: []string{"go.mod", "cmd/api/main.go"},
			files: map[string]string{
				"go.mod":          "module example.com/api\n",
				"cmd/api/main.go": "package main\nimport web \"net/http\"\nfunc main(){ srv := &web.Server{}; srv.ListenAndServe() }\n",
			},
			want: apimodel.KindBackend,
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

// A module with several commands cites every read command, and the no-server
// premise must hold for each of them (cli/cli has cmd/gh and cmd/gen-docs).
func TestEveryReadCommandIsCited(t *testing.T) {
	paths := []string{"go.mod", "cmd/gen-docs/main.go", "cmd/gh/main.go"}
	cobra := Classify(InspectionFromSources("Go", paths, map[string]string{
		"go.mod":               "module example.com/cli\n\nrequire github.com/spf13/cobra v1.8.0\n",
		"cmd/gen-docs/main.go": "package main\nfunc main(){}\n",
		"cmd/gh/main.go":       "package main\nfunc main(){}\n",
	}))
	cited := map[string]bool{}
	for _, o := range cobra.Observations {
		cited[o.Path] = true
	}
	if cobra.Kind != apimodel.KindCLI || !cited["cmd/gen-docs/main.go"] || !cited["cmd/gh/main.go"] {
		t.Errorf("got %q citing %v, want cli citing both commands", cobra.Kind, cited)
	}

	oneServes := Classify(InspectionFromSources("Go", paths, map[string]string{
		"go.mod":               "module example.com/cli\n",
		"cmd/gen-docs/main.go": "package main\nfunc main(){}\n",
		"cmd/gh/main.go":       "package main\nimport \"net/http\"\nfunc main(){ http.ListenAndServe(\":80\", nil) }\n",
	}))
	if oneServes.Kind != apimodel.KindBackend {
		t.Errorf("a server call in any command makes it a backend; got %q", oneServes.Kind)
	}

	oneUnread := Classify(Inspection{
		Language: "Go", Paths: paths,
		Files: []SourceFile{
			{Path: "go.mod", Content: "module example.com/cli\n"},
			{Path: "cmd/gen-docs/main.go", Content: "package main\nfunc main(){}\n"},
		},
		Unavailable: []Unavailable{{Path: "cmd/gh/main.go", Reason: "fetch error"}},
	})
	if oneUnread.Kind == apimodel.KindCLI && oneUnread.Status == VerdictResolved {
		t.Errorf("an unread command could be the server; got a resolved CLI")
	}
}
