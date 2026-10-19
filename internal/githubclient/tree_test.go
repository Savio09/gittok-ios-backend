package githubclient

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// treeBody is a git-trees response with entries at several depths.
const treeBody = `{
  "sha": "root-sha",
  "truncated": false,
  "tree": [
    {"path": "README.md",        "type": "blob", "size": 1024, "mode": "100644"},
    {"path": "go.mod",           "type": "blob", "size": 120,  "mode": "100644"},
    {"path": "internal",         "type": "tree",               "mode": "040000"},
    {"path": "internal/app.go",  "type": "blob", "size": 2048, "mode": "100644"},
    {"path": "a/b/c/deep.go",    "type": "blob", "size": 64,   "mode": "100644"}
  ]
}`

func TestGetTreeMapsEntriesAndPath(t *testing.T) {
	var gotPath, gotQuery string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(treeBody))
	})

	tree, err := c.GetTree(context.Background(), "owner", "repo", "main", TreeLimits{})
	if err != nil {
		t.Fatalf("GetTree error = %v", err)
	}

	if gotPath != "/repos/owner/repo/git/trees/main" {
		t.Errorf("request path = %q", gotPath)
	}
	if !strings.Contains(gotQuery, "recursive=1") {
		t.Errorf("query = %q, want recursive=1", gotQuery)
	}
	if tree.Truncated {
		t.Errorf("Truncated = true, want false")
	}
	if len(tree.Entries) != 5 {
		t.Fatalf("entries = %d, want 5", len(tree.Entries))
	}
	// Spot-check a blob and a tree entry.
	var readme, dir *TreeEntry
	for i := range tree.Entries {
		switch tree.Entries[i].Path {
		case "README.md":
			readme = &tree.Entries[i]
		case "internal":
			dir = &tree.Entries[i]
		}
	}
	if readme == nil || readme.Type != "blob" || readme.Size != 1024 {
		t.Errorf("README entry wrong: %+v", readme)
	}
	if dir == nil || dir.Type != "tree" {
		t.Errorf("internal entry wrong: %+v", dir)
	}
}

func TestGetTreeEntryCapTruncates(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(treeBody))
	})

	tree, err := c.GetTree(context.Background(), "o", "r", "main", TreeLimits{MaxEntries: 2})
	if err != nil {
		t.Fatalf("GetTree error = %v", err)
	}
	if len(tree.Entries) != 2 {
		t.Errorf("entries = %d, want 2 (capped)", len(tree.Entries))
	}
	if !tree.Truncated {
		t.Errorf("Truncated = false, want true when the entry cap clips the tree")
	}
}

func TestGetTreeDepthCapFilters(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(treeBody))
	})

	// Depth 2 keeps README.md, go.mod, internal, internal/app.go; drops a/b/c/deep.go (depth 4).
	tree, err := c.GetTree(context.Background(), "o", "r", "main", TreeLimits{MaxDepth: 2})
	if err != nil {
		t.Fatalf("GetTree error = %v", err)
	}
	for _, e := range tree.Entries {
		if strings.Count(e.Path, "/")+1 > 2 {
			t.Errorf("entry %q exceeds MaxDepth 2", e.Path)
		}
	}
	if len(tree.Entries) != 4 {
		t.Errorf("entries = %d, want 4 after depth filter", len(tree.Entries))
	}
}

func TestGetTreeHonorsGitHubTruncated(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"sha":"s","truncated":true,"tree":[{"path":"x","type":"blob","size":1}]}`))
	})
	tree, err := c.GetTree(context.Background(), "o", "r", "main", TreeLimits{})
	if err != nil {
		t.Fatalf("GetTree error = %v", err)
	}
	if !tree.Truncated {
		t.Errorf("Truncated = false, want true when GitHub reports truncated")
	}
}

func TestGetTreeNotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	_, err := c.GetTree(context.Background(), "o", "r", "nope", TreeLimits{})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
