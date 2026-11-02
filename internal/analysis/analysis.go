// Package analysis acquires bounded repository evidence for commands and HTTP.
package analysis

import (
	"context"
	"fmt"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/crawler"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

// GitHub is the acquisition interface; selection and interpretation stay local.
type GitHub interface {
	GetRepo(context.Context, string, string) (githubclient.Repo, error)
	ResolveRef(context.Context, string, string, string) (string, error)
	GetTree(context.Context, string, string, string, githubclient.TreeLimits) (githubclient.FileTree, error)
	GetFileContents(context.Context, string, string, string, []string, githubclient.ContentLimits) (githubclient.FetchedContents, error)
}

// Result preserves exactly the evidence consumed by the classifier.
type Result struct {
	Repo           githubclient.Repo
	Tree           githubclient.FileTree
	Requested      []string
	Fetched        githubclient.FetchedContents
	Inspection     crawler.Inspection
	Classification crawler.Classification
	CapturedAt     time.Time
	Limitations    []string
}

// Service has no mutable request state and may be shared by HTTP handlers.
type Service struct {
	github  GitHub
	timeout time.Duration
}

func NewService(github GitHub) *Service { return &Service{github: github, timeout: 30 * time.Second} }

// Acquire reads metadata and canonically selected files without executing code.
func (s *Service) Acquire(parent context.Context, owner, name string) (Result, error) {
	if err := parent.Err(); err != nil {
		return Result{}, err
	}
	ctx, cancel := context.WithTimeout(parent, s.timeout)
	defer cancel()
	repo, err := s.github.GetRepo(ctx, owner, name)
	if err != nil {
		return Result{}, fmt.Errorf("metadata: %w", err)
	}
	a := Result{Repo: repo, CapturedAt: time.Now().UTC()}
	finish := func(stage string) (Result, error) {
		if err := parent.Err(); err != nil {
			return Result{}, err
		}
		if stage != "" {
			a.Limitations = append(a.Limitations, stage)
		}
		a.Inspection = crawler.NewInspection(repo.Language, a.Inspection.Revision, a.Tree, a.Fetched)
		a.Classification = crawler.Classify(a.Inspection)
		return a, nil
	}
	a.Tree.Truncated = true // until a successful tree establishes listing scope
	ref := repo.DefaultBranch
	if ref == "" {
		return finish("GitHub did not report a default branch; file evidence was not acquired.")
	}
	if ctx.Err() != nil {
		return finish("Analysis deadline reached after metadata acquisition.")
	}
	ref, err = s.github.ResolveRef(ctx, owner, name, ref)
	if err != nil || !githubclient.ValidCommitID(ref) {
		return finish("The default branch could not be pinned to a commit; file evidence was not acquired.")
	}
	a.Inspection.Revision = crawler.Revision{Ref: ref, Resolved: true}
	if ctx.Err() != nil {
		return finish("Analysis stopped before the tree could be acquired.")
	}
	tree, err := s.github.GetTree(ctx, owner, name, ref, githubclient.DefaultTreeLimits())
	if err != nil {
		return finish("The file tree could not be acquired; only metadata evidence is available.")
	}
	a.Tree = tree
	for _, entry := range tree.Entries {
		if entry.Mode == "120000" {
			a.Limitations = append(a.Limitations, "Symlink contents were excluded from inspection.")
			break
		}
	}
	selected := crawler.SelectFiles(tree.Entries, crawler.DefaultSelectionBudget())
	paths := make([]string, len(selected))
	for i, f := range selected {
		paths[i] = f.Path
	}
	a.Requested = paths
	if ctx.Err() != nil {
		return finish("Analysis stopped before selected contents could be acquired.")
	}
	fetched, err := s.github.GetFileContents(ctx, owner, name, ref, paths, githubclient.DefaultContentLimits())
	a.Fetched = fetched
	if err != nil {
		return finish("Selected contents could not all be acquired; only retained evidence was inspected.")
	}
	return finish("")
}
