// Package analysis acquires bounded repository evidence for commands and HTTP.
package analysis

import (
	"context"
	"fmt"

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
}

// Service has no mutable request state and may be shared by HTTP handlers.
type Service struct{ github GitHub }

func NewService(github GitHub) *Service { return &Service{github: github} }

// Acquire reads metadata and canonically selected files without executing code.
func (s *Service) Acquire(ctx context.Context, owner, name string) (Result, error) {
	repo, err := s.github.GetRepo(ctx, owner, name)
	if err != nil {
		return Result{}, fmt.Errorf("metadata: %w", err)
	}
	ref := repo.DefaultBranch
	if ref == "" {
		ref = "main"
	}
	tree, err := s.github.GetTree(ctx, owner, name, ref, githubclient.DefaultTreeLimits())
	if err != nil {
		return Result{}, fmt.Errorf("tree: %w", err)
	}
	selected := crawler.SelectFiles(tree.Entries, crawler.DefaultSelectionBudget())
	paths := make([]string, len(selected))
	for i, f := range selected {
		paths[i] = f.Path
	}
	fetched, err := s.github.GetFileContents(ctx, owner, name, ref, paths, githubclient.DefaultContentLimits())
	if err != nil {
		return Result{}, fmt.Errorf("contents: %w", err)
	}
	insp := crawler.NewInspection(repo.Language, crawler.Revision{Ref: ref}, tree, fetched)
	return Result{Repo: repo, Tree: tree, Requested: paths, Fetched: fetched, Inspection: insp, Classification: crawler.Classify(insp)}, nil
}
