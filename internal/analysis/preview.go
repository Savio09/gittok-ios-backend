package analysis

import (
	"context"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/preview"
)

// Preview composes reusable acquisition and pure rendering. Only retained,
// validated evidence survives a partial read. No target code is executed.
//
// With a cache, current metadata is read and gated first on every request:
// only a repository GitHub reports public, with a positive stable ID and a
// valid canonical name, may be answered — from a stored snapshot for exactly
// that identity, or by acquiring from that same verified metadata. Only a
// completed bounded inspection is stored.
func (s *Service) Preview(ctx context.Context, owner, name string) (apimodel.RepoPreview, error) {
	if s.cache == nil {
		a, err := s.Acquire(ctx, owner, name)
		if err != nil {
			return apimodel.RepoPreview{}, err
		}
		return s.render(ctx, a)
	}
	repo, err := s.metadata(ctx, owner, name)
	if err != nil {
		return apimodel.RepoPreview{}, err
	}
	key := CacheKey{RepoID: repo.ID, Owner: repo.Owner, Name: repo.Name}
	cacheable := repo.ID > 0
	if cacheable {
		if p, ok := s.cache.Get(key); ok {
			return p, nil
		}
	}
	a, err := s.acquireFrom(ctx, repo, owner, name)
	if err != nil {
		return apimodel.RepoPreview{}, err
	}
	p, err := s.render(ctx, a)
	if err != nil {
		return apimodel.RepoPreview{}, err
	}
	if cacheable && a.Outcome.Cacheable() && ctx.Err() == nil {
		s.cache.Put(key, a.Inspection.Revision.Ref, a.CapturedAt, p)
	}
	return p, nil
}

func (s *Service) render(ctx context.Context, a Result) (apimodel.RepoPreview, error) {
	if err := ctx.Err(); err != nil {
		return apimodel.RepoPreview{}, err
	}
	p := preview.Build(preview.Input{Repo: a.Repo, Inspection: a.Inspection, CapturedAt: a.CapturedAt, Coverage: a.Coverage, Limitations: a.Limitations})
	if err := ctx.Err(); err != nil {
		return apimodel.RepoPreview{}, err
	}
	return p, nil
}
