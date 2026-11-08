package crawler

import (
	"path"
	"strings"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

// blobs builds a blob-only tree from the given paths, which is what SelectFiles
// ranks. A "tree" (directory) entry is added too so the blob/tree filter is
// exercised and directories never leak into the result.
func blobs(paths ...string) []githubclient.TreeEntry {
	entries := []githubclient.TreeEntry{{Path: "somedir", Type: "tree"}}
	for _, p := range paths {
		entries = append(entries, githubclient.TreeEntry{Path: p, Type: "blob"})
	}
	return entries
}

func paths(sel []SelectedFile) []string {
	out := make([]string, len(sel))
	for i, s := range sel {
		out[i] = s.Path
	}
	return out
}

func contains(sel []SelectedFile, p string) bool {
	for _, s := range sel {
		if s.Path == p {
			return true
		}
	}
	return false
}

// rank returns the 0-based position of p in the result, or -1 if absent.
func rank(sel []SelectedFile, p string) int {
	for i, s := range sel {
		if s.Path == p {
			return i
		}
	}
	return -1
}

func countBase(sel []SelectedFile, base string) int {
	n := 0
	for _, s := range sel {
		if strings.EqualFold(path.Base(s.Path), base) {
			n++
		}
	}
	return n
}

// TestSelectFiles covers the inclusion/ranking behavior per repo shape, the
// per-basename dedupe, and — the point of G16 — the entry-source guarantee that
// keeps the classifier able to tell a server from a CLI.
func TestSelectFiles(t *testing.T) {
	cases := []struct {
		name   string
		tree   []githubclient.TreeEntry
		budget SelectionBudget
		// assertions
		mustInclude []string // paths that must be selected
		mustExclude []string // paths that must NOT be selected
		// orderedBefore[i] must out-rank orderedBefore[i+1] when both present
		orderedBefore []string
	}{
		{
			name: "contribution and readme and CI are all selected and ranked",
			tree: blobs(
				"CONTRIBUTING.md", "README.md", "go.mod",
				".github/workflows/ci.yml", "main.go",
			),
			budget:        DefaultSelectionBudget(),
			mustInclude:   []string{"go.mod", "CONTRIBUTING.md", "README.md", "main.go", ".github/workflows/ci.yml"},
			orderedBefore: []string{"go.mod", "CONTRIBUTING.md", "README.md", "main.go", ".github/workflows/ci.yml"},
		},
		{
			name: "directories and unscored files are never selected",
			tree: append(
				blobs("go.mod", "main.go", "LICENSE", "assets/logo.png", ".gitignore"),
				githubclient.TreeEntry{Path: "internal", Type: "tree"},
			),
			budget:      DefaultSelectionBudget(),
			mustInclude: []string{"go.mod", "main.go"},
			mustExclude: []string{"LICENSE", "assets/logo.png", ".gitignore", "internal", "somedir"},
		},
		{
			name: "independent monorepo manifests remain eligible",
			tree: blobs(
				"package.json",
				"packages/a/package.json", "packages/b/package.json",
				"packages/c/package.json", "packages/d/package.json",
				"packages/e/package.json",
			),
			budget: DefaultSelectionBudget(),
			// Root wins (shallowest); independent manifests share the total cap,
			// rather than displacing one another through a global basename cap.
			mustInclude: []string{"package.json"},
		},
		{
			name: "go entry source is guaranteed over higher-scored files (caddy gap)",
			tree: blobs("go.mod", "README.md", "cmd/caddy/main.go"),
			// A tight budget forces the squeeze: without the guarantee the two
			// higher-scored files (go.mod=100, README=85) would fill MaxFiles and
			// drop the entry source (main.go=48), hiding ListenAndServe from the
			// classifier and misclassifying the server as a CLI.
			budget:      SelectionBudget{MaxFiles: 2, MaxSource: 6, MaxPerBase: 3, MaxEntry: 1},
			mustInclude: []string{"cmd/caddy/main.go", "go.mod"},
			mustExclude: []string{"README.md"},
		},
		{
			name: "entry reservation is bounded by MaxEntry so manifests survive",
			tree: blobs(
				"go.mod", "package.json",
				"cmd/a/main.go", "cmd/b/main.go", "cmd/c/main.go",
			),
			budget: SelectionBudget{MaxFiles: 2, MaxSource: 6, MaxPerBase: 3, MaxEntry: 1},
			// Only one entry is reserved, leaving room for the top manifest.
			mustInclude: []string{"go.mod"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := SelectFiles(c.tree, c.budget)
			for _, p := range c.mustInclude {
				if !contains(got, p) {
					t.Errorf("want %q selected; got %v", p, paths(got))
				}
			}
			for _, p := range c.mustExclude {
				if contains(got, p) {
					t.Errorf("want %q NOT selected; got %v", p, paths(got))
				}
			}
			for i := 0; i+1 < len(c.orderedBefore); i++ {
				a, b := c.orderedBefore[i], c.orderedBefore[i+1]
				ra, rb := rank(got, a), rank(got, b)
				if ra >= 0 && rb >= 0 && ra > rb {
					t.Errorf("want %q ranked before %q; got order %v", a, b, paths(got))
				}
			}
			// Every selection must carry a non-empty reason (evidence philosophy).
			for _, s := range got {
				if strings.TrimSpace(s.Reason) == "" {
					t.Errorf("selected %q has an empty Reason", s.Path)
				}
			}
		})
	}
}

// Repeated docs still obey the basename cap; independent component manifests
// are separately covered by TestG16IndependentManifestCoverage.
func TestSelectFilesPerBaseCap(t *testing.T) {
	tree := blobs("README.md", "a/README.md", "b/README.md", "c/README.md", "d/README.md")
	budget := DefaultSelectionBudget()
	got := SelectFiles(tree, budget)
	if n := countBase(got, "README.md"); n != budget.MaxPerBase {
		t.Fatalf("README count=%d, want=%d: %v", n, budget.MaxPerBase, paths(got))
	}
	if !contains(got, "README.md") {
		t.Fatal("root README was displaced")
	}
}

// TestSelectFilesBudgetEnforced throws a large, varied tree at the selector and
// asserts the result honors every cap — the budget can never be exceeded, by
// any selection path (including the guaranteed entry reservation).
func TestSelectFilesBudgetEnforced(t *testing.T) {
	var ps []string
	// Many manifests across a monorepo.
	for _, d := range []string{"a", "b", "c", "d", "e", "f"} {
		ps = append(ps, "packages/"+d+"/package.json")
	}
	// Many docs and CI files.
	ps = append(ps, "README.md", "CONTRIBUTING.md", "docs/README.md",
		".github/workflows/ci.yml", ".github/workflows/release.yml")
	// Many entry points.
	for _, d := range []string{"a", "b", "c", "d", "e"} {
		ps = append(ps, "cmd/"+d+"/main.go")
	}
	// Lots of generic source with distinct basenames to pressure the source
	// sub-cap (shared basenames would be clipped by MaxPerBase first).
	for i := 0; i < 20; i++ {
		n := string(rune('a' + i))
		ps = append(ps, "internal/mod"+n+"/svc"+n+".go")
	}
	tree := blobs(ps...)

	budget := DefaultSelectionBudget()
	got := SelectFiles(tree, budget)

	if len(got) > budget.MaxFiles {
		t.Errorf("selected %d files, exceeds MaxFiles %d", len(got), budget.MaxFiles)
	}
	// Per-basename cap.
	perBase := map[string]int{}
	sourceCount := 0
	for _, s := range got {
		perBase[strings.ToLower(path.Base(s.Path))]++
		if isSourceFile(s.Path) && scoreTreeFile(s.Path).isSource {
			sourceCount++
		}
	}
	for base, n := range perBase {
		if base != "package.json" && n > budget.MaxPerBase {
			t.Errorf("base %q selected %d times, exceeds MaxPerBase %d", base, n, budget.MaxPerBase)
		}
	}
	if sourceCount > budget.MaxSource {
		t.Errorf("generic source files selected %d, exceeds MaxSource %d", sourceCount, budget.MaxSource)
	}
	// The guarantee still held under pressure: at least one entry source made it.
	if countBase(got, "main.go") == 0 {
		t.Errorf("no entry source selected; the caddy-gap guarantee failed: %v", paths(got))
	}
}

// TestSelectFilesEmpty confirms an empty or signal-free tree yields no panic
// and an empty result.
func TestSelectFilesEmpty(t *testing.T) {
	if got := SelectFiles(nil, DefaultSelectionBudget()); len(got) != 0 {
		t.Errorf("empty tree: got %v, want none", paths(got))
	}
	noSignal := blobs("LICENSE", "assets/logo.png", ".gitignore")
	if got := SelectFiles(noSignal, DefaultSelectionBudget()); len(got) != 0 {
		t.Errorf("signal-free tree: got %v, want none", paths(got))
	}
}
