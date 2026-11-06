package githubclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// TreeEntry is one node in a repository's file tree.
type TreeEntry struct {
	Path string // full path from the repo root, e.g. "internal/app.go"
	Type string // "blob" (file) or "tree" (directory)
	Size int    // bytes; 0 for directories
	Mode string // Git mode; 120000 denotes a symlink, never regular-file evidence
	SHA  string // Git blob/tree object ID reported for this entry
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
	// MaxDepth is the depth bound that was applied (0 when unbounded), so a
	// reader can tell exactly which paths a depth-limited listing still covers.
	MaxDepth int
	// Request is the API path that returned this listing, so evidence that
	// cites the tree names the request that actually ran.
	Request string
}

// TreeRequestPath is the API path GetTree requests for a listing at ref.
func TreeRequestPath(owner, name, ref string) string {
	return "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name) +
		"/git/trees/" + url.PathEscape(ref) + "?recursive=1"
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

type treeResponse struct {
	limits TreeLimits
	tree   FileTree
}

// Stream the listing so the entry cap bounds allocation and reading, rather
// than clipping a fully allocated provider tree. Once capped, the unread tail
// is unknown regardless of the provider's truncated flag.
func (p *treeResponse) decodeJSON(dec *json.Decoder) error {
	if token, err := dec.Token(); err != nil {
		return err
	} else if token != json.Delim('{') {
		return fmt.Errorf("invalid tree response")
	}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return err
		}
		switch key {
		case "tree":
			if token, err := dec.Token(); err != nil {
				return err
			} else if token != json.Delim('[') {
				return fmt.Errorf("invalid tree entries")
			}
			for dec.More() {
				if p.limits.MaxEntries > 0 && len(p.tree.Entries) >= p.limits.MaxEntries {
					p.tree.Truncated = true
					return nil
				}
				var e TreeEntry
				if err := dec.Decode(&e); err != nil {
					return err
				}
				if p.limits.MaxDepth > 0 && depth(e.Path) > p.limits.MaxDepth {
					p.tree.DepthLimited = true
					continue
				}
				p.tree.Entries = append(p.tree.Entries, e)
			}
			if _, err := dec.Token(); err != nil {
				return err
			}
		case "truncated":
			if err := dec.Decode(&p.tree.Truncated); err != nil {
				return err
			}
		default:
			var discard json.RawMessage
			if err := dec.Decode(&discard); err != nil {
				return err
			}
		}
	}
	_, err := dec.Token()
	return err
}

// GetTree fetches a repository's file tree at ref (a branch name or commit SHA),
// recursively, then applies limits. It returns ErrNotFound when the repo or ref
// does not exist.
func (c *Client) GetTree(ctx context.Context, owner, name, ref string, limits TreeLimits) (FileTree, error) {
	path := TreeRequestPath(owner, name, ref)

	p := treeResponse{limits: limits, tree: FileTree{MaxDepth: max(limits.MaxDepth, 0), Request: path}}
	if err := c.get(ctx, path, &p); err != nil {
		return FileTree{}, err
	}

	return p.tree, nil
}

// depth is the number of path segments: "README.md" is 1, "a/b.go" is 2.
func depth(path string) int {
	return strings.Count(path, "/") + 1
}
