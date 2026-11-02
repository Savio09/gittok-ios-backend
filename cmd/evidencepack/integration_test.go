package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/crawler"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

// Slice 9: a deterministic, network-free pass through the whole analysis
// handoff — metadata → tree → selection → bounded contents → evidence →
// classification — driven by a mocked GitHub. It proves positive classification
// with provenance, honest degradation when evidence is removed, and that a
// branch ref is never recorded as a pinned commit.

// mockGitHub serves the three endpoints the pipeline uses. missing is the set of
// paths whose contents fetch should 404 (to simulate a failed read). A path in
// block is held open until the request's context ends; arrived, when set,
// receives each contents path as its request arrives, so a test can cancel at
// an exact point without relying on timing.
type mockGitHub struct {
	defaultBranch string
	language      string
	entries       []githubclient.TreeEntry
	truncated     bool
	files         map[string]string
	missing       map[string]bool
	block         map[string]bool
	arrived       chan string
}

func (m *mockGitHub) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)

	mux.HandleFunc("/repos/o/r", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name": "r", "full_name": "o/r",
			"owner":          map[string]any{"login": "o"},
			"language":       m.language,
			"default_branch": m.defaultBranch,
		})
	})
	mux.HandleFunc("/repos/o/r/commits/", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"sha": "0123456789abcdef0123456789abcdef01234567"})
	})
	mux.HandleFunc("/repos/o/r/git/trees/", func(w http.ResponseWriter, _ *http.Request) {
		tree := make([]map[string]any, 0, len(m.entries))
		for _, e := range m.entries {
			tree = append(tree, map[string]any{"path": e.Path, "type": e.Type, "size": e.Size})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"truncated": m.truncated, "tree": tree})
	})
	mux.HandleFunc("/repos/o/r/contents/", func(w http.ResponseWriter, req *http.Request) {
		path := strings.TrimPrefix(req.URL.Path, "/repos/o/r/contents/")
		if m.arrived != nil {
			m.arrived <- path
		}
		if m.block[path] {
			<-req.Context().Done()
			return
		}
		if m.missing[path] {
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
			return
		}
		content, ok := m.files[path]
		if !ok {
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type": "file", "encoding": "base64", "size": len(content),
			"content": base64.StdEncoding.EncodeToString([]byte(content)),
		})
	})
	t.Cleanup(srv.Close)
	return srv
}

func blobEntries(paths ...string) []githubclient.TreeEntry {
	e := make([]githubclient.TreeEntry, len(paths))
	for i, p := range paths {
		e[i] = githubclient.TreeEntry{Path: p, Type: "blob"}
	}
	return e
}

func TestAnalyzeBackendWithProvenanceAndNoReadme(t *testing.T) {
	m := &mockGitHub{
		defaultBranch: "main", language: "Go",
		entries: blobEntries("go.mod", "cmd/api/main.go"),
		files: map[string]string{
			"go.mod":          "module example.com/api\n",
			"cmd/api/main.go": "package main\nimport \"net/http\"\nfunc main() { http.ListenAndServe(\":80\", nil) }\n",
		},
	}
	c := githubclient.New("t", githubclient.WithBaseURL(m.server(t).URL))

	a, err := analyze(context.Background(), c, "o", "r")
	if err != nil {
		t.Fatal(err)
	}
	if a.Classification.Kind != apimodel.KindBackend {
		t.Errorf("Kind = %q, want backend (no README, classified from code)", a.Classification.Kind)
	}
	// The verdict is traceable to the actual source file.
	cited := false
	for _, o := range a.Classification.Observations {
		if o.Path == "cmd/api/main.go" {
			cited = true
		}
	}
	if !cited {
		t.Errorf("no observation cited cmd/api/main.go; %+v", a.Classification.Observations)
	}
	// All reads use the resolved commit, not a moving branch.
	if !a.Inspection.Revision.Resolved || a.Inspection.Revision.Ref != "0123456789abcdef0123456789abcdef01234567" {
		t.Errorf("unexpected revision: %+v", a.Inspection.Revision)
	}
}

func TestAnalyzeDegradesWhenDecisiveSourceUnfetched(t *testing.T) {
	m := &mockGitHub{
		defaultBranch: "main", language: "Go",
		entries: blobEntries("go.mod", "cmd/api/main.go"),
		files:   map[string]string{"go.mod": "module example.com/api\n"},
		missing: map[string]bool{"cmd/api/main.go": true}, // fetch fails
	}
	c := githubclient.New("t", githubclient.WithBaseURL(m.server(t).URL))

	a, err := analyze(context.Background(), c, "o", "r")
	if err != nil {
		t.Fatal(err)
	}
	if a.Classification.Status != crawler.VerdictInconclusive {
		t.Errorf("Status = %q, want inconclusive when the entry source could not be fetched", a.Classification.Status)
	}
}

func TestAnalyzeTruncatedTreeDoesNotAssertAbsence(t *testing.T) {
	m := &mockGitHub{
		defaultBranch: "main", language: "Go",
		entries:   blobEntries("go.mod", "pkg/calc/calc.go"),
		truncated: true,
		files:     map[string]string{"go.mod": "module example.com/calc\n"},
	}
	c := githubclient.New("t", githubclient.WithBaseURL(m.server(t).URL))

	a, err := analyze(context.Background(), c, "o", "r")
	if err != nil {
		t.Fatal(err)
	}
	if a.Classification.Kind == apimodel.KindLibrary {
		t.Errorf("asserted library from a truncated tree; want abstention")
	}
}

// Cancellation during the final — or only — contents request stops the whole
// analysis with the context error, rather than reporting a skipped file and a
// verdict built on it. The test cancels exactly when that request arrives.
func TestAnalyzeCancellationOnFinalContentsRequest(t *testing.T) {
	cases := []struct {
		name    string
		entries []githubclient.TreeEntry
		files   map[string]string
		last    string
	}{
		{"only file", blobEntries("go.mod"), map[string]string{"go.mod": "module x\n"}, "go.mod"},
		{"final file", blobEntries("go.mod", "main.go"), map[string]string{"go.mod": "module x\n", "main.go": "package main\nfunc main(){}\n"}, "main.go"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := &mockGitHub{
				defaultBranch: "main", language: "Go", entries: c.entries, files: c.files,
				block: map[string]bool{c.last: true}, arrived: make(chan string, 8),
			}
			cl := githubclient.New("t", githubclient.WithBaseURL(m.server(t).URL), githubclient.WithMaxRetries(0))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go func() {
				for p := range m.arrived {
					if p == c.last {
						cancel()
						return
					}
				}
			}()

			a, err := analyze(ctx, cl, "o", "r")
			if !errors.Is(err, context.Canceled) {
				t.Errorf("err = %v (verdict %q/%q), want context.Canceled", err, a.Classification.Kind, a.Classification.Status)
			}
		})
	}
}

// The rendered pack shows exactly the validated facts the verdict used, each
// with its witness.
func TestAnalyzeRenderedPackCarriesWitnesses(t *testing.T) {
	m := &mockGitHub{
		defaultBranch: "main", language: "Go",
		entries: blobEntries("go.mod", "cmd/api/main.go"),
		files: map[string]string{
			"go.mod":          "module example.com/api\n",
			"cmd/api/main.go": "package main\nimport \"net/http\"\nfunc main() { http.ListenAndServe(\":80\", nil) }\n",
		},
	}
	c := githubclient.New("t", githubclient.WithBaseURL(m.server(t).URL))
	a, err := analyze(context.Background(), c, "o", "r")
	if err != nil {
		t.Fatal(err)
	}
	md := renderPack(a.Repo, a.Tree, a.Requested, a.Fetched, a.Classification)
	for _, want := range []string{"**Kind:** backend", "witness: cmd/api/main.go bytes", "(go.server-call)", "(go.package-main)"} {
		if !strings.Contains(md, want) {
			t.Errorf("rendered pack missing %q", want)
		}
	}
}
