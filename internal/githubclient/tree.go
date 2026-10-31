package githubclient

import (
	"context"
	"net/url"
	"strings"
)

// TreeEntry is one node in a repository's file tree.
type TreeEntry struct {
	Path string // full path from the repo root, e.g. "internal/app.go"
	Type string // "blob" (file) or "tree" (directory)
	Size int    // bytes; 0 for directories
}

// FileTree is a repository's file listing, bounded by TreeLimits. Truncated is
// true when the listing is incomplete — either GitHub truncated a very large
// tree, or the entry cap clipped it. DepthLimited is true when the MaxDepth
// scope actually dropped one or more deeper entries.
//
// The two flags are kept distinct because they bound a reader differently:
// Truncated means "there may be more at any depth"; DepthLimited means "the
// shallow levels are complete but anything below MaxDepth was not listed". Both
// mean an absence ("no main package anywhere") cannot be asserted from this
// listing alone. (Depth filtering does not set Truncated.)
type FileTree struct {
	Entries      []TreeEntry
	Truncated    bool
	DepthLimited bool
}

// TreeLimits bounds how much of a tree GetTree returns, so a huge repository
// cannot blow the analysis budget. A non-positive value means that dimension is
// unbounded.
type TreeLimits struct {
	MaxEntries int // cap on the number of entries returned
	MaxDepth   int // cap on path depth; "a/b.go" is depth 2
}

// DefaultTreeLimits is a sensible bound for ingesting a repo for analysis.
func DefaultTreeLimits() TreeLimits {
	return TreeLimits{MaxEntries: 5000, MaxDepth: 12}
}

type treePayload struct {
	Truncated bool `json:"truncated"`
	Tree      []struct {
		Path string `json:"path"`
		Type string `json:"type"`
		Size int    `json:"size"`
	} `json:"tree"`
}

// GetTree fetches a repository's file tree at ref (a branch name or commit SHA),
// recursively, then applies limits. It returns ErrNotFound when the repo or ref
// does not exist.
func (c *Client) GetTree(ctx context.Context, owner, name, ref string, limits TreeLimits) (FileTree, error) {
	path := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name) +
		"/git/trees/" + url.PathEscape(ref) + "?recursive=1"

	var p treePayload
	if err := c.get(ctx, path, &p); err != nil {
		return FileTree{}, err
	}

	tree := FileTree{Truncated: p.Truncated}
	entries := make([]TreeEntry, 0, len(p.Tree))
	for _, e := range p.Tree {
		if limits.MaxDepth > 0 && depth(e.Path) > limits.MaxDepth {
			tree.DepthLimited = true
			continue
		}
		entries = append(entries, TreeEntry{Path: e.Path, Type: e.Type, Size: e.Size})
	}
	if limits.MaxEntries > 0 && len(entries) > limits.MaxEntries {
		entries = entries[:limits.MaxEntries]
		tree.Truncated = true
	}
	tree.Entries = entries
	return tree, nil
}

// depth is the number of path segments: "README.md" is 1, "a/b.go" is 2.
func depth(path string) int {
	return strings.Count(path, "/") + 1
}
