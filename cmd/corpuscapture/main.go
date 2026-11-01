// Command corpuscapture records sanitized, provenance-labelled snapshots of real
// repositories for the classifier's offline acceptance corpus. It performs only
// bounded read-only GitHub API reads, mirroring the analysis path exactly:
// metadata, the default branch resolved to a commit SHA, the bounded tree at
// that SHA, the canonical file selection, and a bounded contents fetch — every
// read at the same immutable revision. Files fetched beyond the selection (the
// decisive evidence a reference judgment needs) come only from extras.json,
// each with a written reason, and are recorded separately from the selection.
// Anything that looks like a credential is withheld, not stored.
//
// Usage:
//
//	GITHUB_TOKEN=... go run ./cmd/corpuscapture -out internal/crawler/testdata/corpus owner/repo ...
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/corpus"
	"github.com/Savio09/gittok-ios-backend/internal/crawler"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

func main() {
	out := flag.String("out", "internal/crawler/testdata/corpus", "directory to write snapshots and read extras.json from")
	flag.Parse()

	extras := map[string]map[string]string{}
	if b, err := os.ReadFile(filepath.Join(*out, "extras.json")); err == nil {
		if err := json.Unmarshal(b, &extras); err != nil {
			fail("extras.json: %v", err)
		}
	}

	client := githubclient.New(os.Getenv("GITHUB_TOKEN"))
	ctx := context.Background()
	for _, slug := range flag.Args() {
		s, err := capture(ctx, client, slug, extras[slug])
		if err != nil {
			fail("%s: %v", slug, err)
		}
		file := filepath.Join(*out, strings.ReplaceAll(slug, "/", "__")+".json")
		if err := s.Save(file); err != nil {
			fail("%s: %v", slug, err)
		}
		fmt.Printf("%s @ %s (%s): %d entries, %d selected, %d extras, %d read, %d skipped -> %s\n",
			slug, s.ResolvedSHA[:12], s.RequestedRef, len(s.Tree.Entries), len(s.Selected), len(s.Extras), len(s.Files), len(s.Skipped), file)
	}
}

func capture(ctx context.Context, c *githubclient.Client, slug string, extras map[string]string) (corpus.Snapshot, error) {
	owner, name, ok := strings.Cut(slug, "/")
	if !ok {
		return corpus.Snapshot{}, fmt.Errorf("want owner/repo")
	}
	repo, err := c.GetRepo(ctx, owner, name)
	if err != nil {
		return corpus.Snapshot{}, fmt.Errorf("metadata: %w", err)
	}
	sha, err := c.ResolveRef(ctx, owner, name, repo.DefaultBranch)
	if err != nil {
		return corpus.Snapshot{}, fmt.Errorf("resolve %s: %w", repo.DefaultBranch, err)
	}
	treeLimits, contentLimits := githubclient.DefaultTreeLimits(), githubclient.DefaultContentLimits()
	tree, err := c.GetTree(ctx, owner, name, sha, treeLimits)
	if err != nil {
		return corpus.Snapshot{}, fmt.Errorf("tree: %w", err)
	}

	s := corpus.Snapshot{
		Repo: slug, SourceURL: repo.HTMLURL, CapturedAt: time.Now().UTC().Format("2006-01-02"),
		RequestedRef: repo.DefaultBranch, ResolvedSHA: sha, Language: repo.Language, Archived: repo.Archived,
		TreeLimits: treeLimits, ContentLimits: contentLimits,
		Tree:   corpus.Tree{Truncated: tree.Truncated, DepthLimited: tree.DepthLimited, MaxDepth: tree.MaxDepth},
		Extras: map[string]string{}, Files: map[string]corpus.File{}, Skipped: map[string]string{},
	}
	for _, e := range tree.Entries {
		s.Tree.Entries = append(s.Tree.Entries, corpus.Entry{Path: e.Path, Type: e.Type, Size: e.Size})
	}
	for _, f := range crawler.SelectFiles(tree.Entries, crawler.DefaultSelectionBudget()) {
		s.Selected = append(s.Selected, f.Path)
	}
	if err := fetchInto(ctx, c, owner, name, sha, s.Selected, contentLimits, &s); err != nil {
		return s, err
	}

	var extraPaths []string
	for p, why := range extras {
		if _, done := s.Files[p]; !done {
			extraPaths = append(extraPaths, p)
		}
		s.Extras[p] = why
	}
	sort.Strings(extraPaths)
	if err := fetchInto(ctx, c, owner, name, sha, extraPaths, contentLimits, &s); err != nil {
		return s, err
	}
	return s, s.CheckSanitized()
}

// fetchInto reads paths at sha within limits and records every outcome,
// withholding anything that looks like a credential.
func fetchInto(ctx context.Context, c *githubclient.Client, owner, name, sha string, paths []string, limits githubclient.ContentLimits, s *corpus.Snapshot) error {
	if len(paths) == 0 {
		return nil
	}
	got, err := c.GetFileContents(ctx, owner, name, sha, paths, limits)
	if err != nil {
		return fmt.Errorf("contents: %w", err)
	}
	for _, f := range got.Files {
		if corpus.Secretish(f.Path, f.Content) {
			s.Skipped[f.Path] = "withheld from the corpus: looks like a credential"
			continue
		}
		s.Files[f.Path] = corpus.File{Content: f.Content, Size: f.Size, Truncated: f.Truncated}
	}
	for _, sk := range got.Skipped {
		s.Skipped[sk.Path] = sk.Reason
	}
	return nil
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "corpuscapture: "+format+"\n", args...)
	os.Exit(1)
}
