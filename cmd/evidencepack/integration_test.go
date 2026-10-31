package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
// paths whose contents fetch should 404 (to simulate a failed read). hang, when
// set, blocks the contents endpoint until the test cancels the context.
type mockGitHub struct {
	defaultBranch string
	language      string
	entries       []githubclient.TreeEntry
	truncated     bool
	files         map[string]string
	missing       map[string]bool
	hang          chan struct{}
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
	mux.HandleFunc("/repos/o/r/git/trees/", func(w http.ResponseWriter, _ *http.Request) {
		tree := make([]map[string]any, 0, len(m.entries))
		for _, e := range m.entries {
			tree = append(tree, map[string]any{"path": e.Path, "type": e.Type, "size": e.Size})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"truncated": m.truncated, "tree": tree})
	})
	mux.HandleFunc("/repos/o/r/contents/", func(w http.ResponseWriter, req *http.Request) {
		if m.hang != nil {
			<-m.hang // block until released/cancelled
		}
		path := strings.TrimPrefix(req.URL.Path, "/repos/o/r/contents/")
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
	if a.classification.Kind != apimodel.KindBackend {
		t.Errorf("Kind = %q, want backend (no README, classified from code)", a.classification.Kind)
	}
	// The verdict is traceable to the actual source file.
	cited := false
	for _, o := range a.classification.Observations {
		if o.Path == "cmd/api/main.go" {
			cited = true
		}
	}
	if !cited {
		t.Errorf("no observation cited cmd/api/main.go; %+v", a.classification.Observations)
	}
	// A branch ref is not a pinned commit.
	if a.insp.Revision.Resolved {
		t.Errorf("Revision.Resolved = true for a branch ref; want false")
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
	if a.classification.Status != crawler.VerdictInconclusive {
		t.Errorf("Status = %q, want inconclusive when the entry source could not be fetched", a.classification.Status)
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
	if a.classification.Kind == apimodel.KindLibrary {
		t.Errorf("asserted library from a truncated tree; want abstention")
	}
}

func TestAnalyzeCancellationStopsCollection(t *testing.T) {
	m := &mockGitHub{
		defaultBranch: "main", language: "Go",
		entries: blobEntries("go.mod", "main.go"),
		files:   map[string]string{"go.mod": "module x\n", "main.go": "package main\nfunc main(){}\n"},
		hang:    make(chan struct{}),
	}
	defer close(m.hang)
	c := githubclient.New("t", githubclient.WithBaseURL(m.server(t).URL))

	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()

	start := time.Now()
	_, err := analyze(ctx, c, "o", "r")
	if err == nil {
		t.Fatal("expected an error when the context was cancelled mid-fetch")
	}
	if time.Since(start) > 2*time.Second {
		t.Errorf("analyze did not stop promptly on cancellation")
	}
}
