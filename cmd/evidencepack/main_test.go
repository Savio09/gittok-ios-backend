package main

import (
	"strings"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/crawler"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

// Slice 8a: the command selects key files through the canonical crawler
// selector, so the pack and the analysis path never diverge.
func TestKeyFilePathsUseCanonicalSelector(t *testing.T) {
	entries := []githubclient.TreeEntry{
		{Path: "go.mod", Type: "blob"},
		{Path: "README.md", Type: "blob"},
		{Path: "cmd/app/main.go", Type: "blob"},
		{Path: "internal/foo/foo.go", Type: "blob"},
		{Path: "Dockerfile", Type: "blob"},
		{Path: "docs", Type: "tree"},
	}
	got := keyFilePaths(entries)

	selected := crawler.SelectFiles(entries, crawler.DefaultSelectionBudget())
	want := make([]string, len(selected))
	for i, s := range selected {
		want[i] = s.Path
	}

	if len(got) != len(want) {
		t.Fatalf("keyFilePaths = %v, canonical = %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("path[%d] = %q, canonical = %q", i, got[i], want[i])
		}
	}
}

// Slice 8b: the pack surfaces the crawler's verdict and, crucially, the
// limitations a bounded fetch imposed — a skipped key file is incompleteness,
// not a silent negative.
func TestRenderPackShowsClassificationAndLimitations(t *testing.T) {
	repo := githubclient.Repo{Owner: "o", Name: "r", FullName: "o/r"}
	tree := githubclient.FileTree{Entries: []githubclient.TreeEntry{{Path: "go.mod", Type: "blob"}}}
	fetched := githubclient.FetchedContents{}
	class := crawler.Classification{
		Kind:   "other",
		Status: crawler.VerdictInconclusive,
		Observations: []crawler.Observation{{Path: "go.mod", Detail: "present",
			Witness: crawler.Witness{Kind: crawler.WitnessField, Path: "go.mod", Field: []string{"module"}}}},
		Secondary:   []crawler.Observation{{Path: "ui/package.json", Detail: "depends on react", Component: "ui", Role: "frontend"}},
		Limitations: []string{"cmd/service/main.go was not inspected"},
	}

	md := renderPack(repo, tree, []string{"go.mod"}, fetched, class)

	for _, want := range []string{"Classification", "inconclusive", "cmd/service/main.go was not inspected", "`ui` (frontend): `ui/package.json`", "witness: go.mod field module"} {
		if !strings.Contains(md, want) {
			t.Errorf("rendered pack missing %q", want)
		}
	}
}
