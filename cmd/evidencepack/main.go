// Command evidencepack builds a per-repo "evidence pack" for the gold-standard
// previews (G13). For each repo it pulls metadata, a bounded file tree, and the
// contents of the key files that typically carry a project's signal (manifests,
// entry points, config, contribution docs, README) via the same ingestion
// client the backend uses. The output is raw material for a human to write the
// hand-made gold-standard preview against — it makes no claims of its own.
//
// Usage:
//
//	GITHUB_TOKEN=... go run ./cmd/evidencepack            # the curated set
//	go run ./cmd/evidencepack owner/repo [owner/repo ...] # specific repos
//
// A token is optional but recommended; without one GitHub's anonymous rate
// limit (~60 req/hour) is easily exhausted.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/crawler"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

const outDir = "docs/gold-standard/evidence"

func main() {
	repos := os.Args[1:]
	if len(repos) == 0 {
		repos = []string{
			"excalidraw/excalidraw",
			"pocketbase/pocketbase",
			"Savio09/plantly",
			"Savio09/face-recognition",
		}
	}

	client := githubclient.New(os.Getenv("GITHUB_TOKEN"))
	ctx := context.Background()

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "mkdir:", err)
		os.Exit(1)
	}

	failed := 0
	for _, slug := range repos {
		owner, name, ok := splitSlug(slug)
		if !ok {
			fmt.Fprintf(os.Stderr, "skip %q: want owner/repo\n", slug)
			failed++
			continue
		}
		if err := buildPack(ctx, client, owner, name); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", slug, err)
			failed++
			continue
		}
		fmt.Printf("wrote %s/%s__%s.md\n", outDir, owner, name)
	}
	if failed > 0 {
		os.Exit(1)
	}
}

func splitSlug(s string) (owner, name string, ok bool) {
	parts := strings.SplitN(strings.TrimSpace(s), "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func buildPack(ctx context.Context, c *githubclient.Client, owner, name string) error {
	repo, err := c.GetRepo(ctx, owner, name)
	if err != nil {
		return fmt.Errorf("metadata: %w", err)
	}
	ref := repo.DefaultBranch
	if ref == "" {
		ref = "main"
	}

	tree, err := c.GetTree(ctx, owner, name, ref, githubclient.DefaultTreeLimits())
	if err != nil {
		return fmt.Errorf("tree: %w", err)
	}

	keyFiles := keyFilePaths(tree.Entries)
	fetched, err := c.GetFileContents(ctx, owner, name, ref, keyFiles, githubclient.DefaultContentLimits())
	if err != nil {
		return fmt.Errorf("contents: %w", err)
	}

	// Run the same classification the analysis path uses, over an Inspection
	// built from the real tree and fetch outcome — so the pack shows the verdict
	// and its limitations (a skipped key file becomes a limitation, not a silent
	// negative). The ref is a branch name, not a pinned commit, so Resolved=false.
	insp := crawler.NewInspection(repo.Language, crawler.Revision{Ref: ref}, tree, fetched)
	classification := crawler.Classify(insp)

	md := renderPack(repo, tree, keyFiles, fetched, classification)
	path := filepath.Join(outDir, owner+"__"+name+".md")
	return os.WriteFile(path, []byte(md), 0o644)
}

// keyFilePaths selects the files worth reading using the canonical crawler
// selector — the same ranking and budget the analysis path uses — so the pack
// and the analyzer never diverge. It returns just the paths, which is all the
// fetch step needs. The selector only decides what a human (and the classifier)
// should read, never what is true.
func keyFilePaths(entries []githubclient.TreeEntry) []string {
	selected := crawler.SelectFiles(entries, crawler.DefaultSelectionBudget())
	paths := make([]string, len(selected))
	for i, s := range selected {
		paths[i] = s.Path
	}
	return paths
}

func renderPack(
	repo githubclient.Repo,
	tree githubclient.FileTree,
	requested []string,
	fetched githubclient.FetchedContents,
	class crawler.Classification,
) string {
	var b strings.Builder

	title := repo.FullName
	if title == "" {
		title = repo.Owner + "/" + repo.Name
	}
	fmt.Fprintf(&b, "# Evidence pack — `%s`\n\n", title)
	b.WriteString("> Auto-generated raw material for the hand-made gold-standard preview (G13).\n")
	b.WriteString("> It reproduces what the ingestion client sees; it makes **no claims**. ")
	b.WriteString("Write the preview against this, citing the files below.\n\n")
	fmt.Fprintf(&b, "_Generated %s via `cmd/evidencepack`._\n\n", time.Now().UTC().Format("2006-01-02"))

	// Identity / metadata.
	b.WriteString("## Metadata\n\n")
	fmt.Fprintf(&b, "- **Full name:** %s\n", title)
	fmt.Fprintf(&b, "- **Description:** %s\n", orDash(repo.Description))
	fmt.Fprintf(&b, "- **Primary language:** %s\n", orDash(repo.Language))
	fmt.Fprintf(&b, "- **Stars:** %d\n", repo.Stars)
	fmt.Fprintf(&b, "- **License:** %s\n", orDash(repo.License))
	fmt.Fprintf(&b, "- **Default branch:** %s\n", orDash(repo.DefaultBranch))
	fmt.Fprintf(&b, "- **Last pushed:** %s\n", repo.PushedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "- **Archived:** %t · **Fork:** %t\n", repo.Archived, repo.Fork)
	if len(repo.Topics) > 0 {
		fmt.Fprintf(&b, "- **Topics:** %s\n", strings.Join(repo.Topics, ", "))
	}
	fmt.Fprintf(&b, "- **URL:** %s\n\n", repo.HTMLURL)

	// Tree overview (depth-limited so huge repos stay readable).
	b.WriteString("## Structure (top 2 levels)\n\n")
	if tree.Truncated {
		b.WriteString("_Tree was truncated by the ingestion limits; deeper/more entries exist._\n\n")
	}
	b.WriteString("```\n")
	b.WriteString(renderTree(tree.Entries, 1))
	b.WriteString("```\n\n")
	fmt.Fprintf(&b, "_Total entries ingested: %d._\n\n", len(tree.Entries))

	// Key files fetched.
	b.WriteString("## Key files\n\n")
	fmt.Fprintf(&b, "Selected %d candidate key files; %d fetched, %d skipped.\n\n",
		len(requested), len(fetched.Files), len(fetched.Skipped))

	for _, f := range fetched.Files {
		fmt.Fprintf(&b, "### `%s`", f.Path)
		if f.Truncated {
			fmt.Fprintf(&b, " _(truncated; original %d bytes)_", f.Size)
		}
		b.WriteString("\n\n")
		b.WriteString("```")
		b.WriteString(fenceLang(f.Path))
		b.WriteString("\n")
		b.WriteString(f.Content)
		if !strings.HasSuffix(f.Content, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("```\n\n")
	}

	if len(fetched.Skipped) > 0 {
		b.WriteString("### Skipped\n\n")
		for _, s := range fetched.Skipped {
			fmt.Fprintf(&b, "- `%s` — %s\n", s.Path, s.Reason)
		}
		b.WriteString("\n")
	}

	// Evidence index — the SOURCE INDEX the preview cites against.
	b.WriteString("## Source index (cite against these)\n\n")
	for _, f := range fetched.Files {
		fmt.Fprintf(&b, "- `%s`\n", f.Path)
	}
	b.WriteString("\n")

	// Crawler classification over the same inspection, shown for reference. The
	// pack still makes no claims of its own; this is the analyzer's verdict with
	// its evidence and the limits the bounded fetch imposed.
	b.WriteString("## Classification (crawler, for reference)\n\n")
	fmt.Fprintf(&b, "- **Kind:** %s\n- **Status:** %s\n\n", class.Kind, class.Status)
	if len(class.Observations) > 0 {
		b.WriteString("Observations:\n")
		for _, o := range class.Observations {
			if o.Path != "" {
				fmt.Fprintf(&b, "- `%s` — %s\n", o.Path, o.Detail)
			} else {
				fmt.Fprintf(&b, "- %s\n", o.Detail)
			}
		}
		b.WriteString("\n")
	}
	if len(class.Limitations) > 0 {
		b.WriteString("Limitations:\n")
		for _, l := range class.Limitations {
			fmt.Fprintf(&b, "- %s\n", l)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderTree prints entries up to maxDepth (0-based), directories first, as an
// indented listing. It keeps the overview readable for large repos.
func renderTree(entries []githubclient.TreeEntry, maxDepth int) string {
	kept := make([]githubclient.TreeEntry, 0, len(entries))
	for _, e := range entries {
		if strings.Count(e.Path, "/") <= maxDepth {
			kept = append(kept, e)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].Path < kept[j].Path })

	var b strings.Builder
	for _, e := range kept {
		depth := strings.Count(e.Path, "/")
		indent := strings.Repeat("  ", depth)
		name := filepath.Base(e.Path)
		if e.Type == "tree" {
			fmt.Fprintf(&b, "%s%s/\n", indent, name)
		} else {
			fmt.Fprintf(&b, "%s%s\n", indent, name)
		}
	}
	return b.String()
}

func fenceLang(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".js", ".mjs", ".cjs":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".json":
		return "json"
	case ".yml", ".yaml":
		return "yaml"
	case ".toml":
		return "toml"
	case ".md":
		return "markdown"
	case ".dart":
		return "dart"
	}
	if strings.EqualFold(filepath.Base(path), "dockerfile") {
		return "dockerfile"
	}
	if strings.EqualFold(filepath.Base(path), "makefile") {
		return "makefile"
	}
	return ""
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
