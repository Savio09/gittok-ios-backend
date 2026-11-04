package analysis

import (
	"context"
	"time"

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
//
// With admission, identical misses for one verified identity share a single
// acquisition under its own deadline, and excess work fails fast with
// ErrOverloaded instead of queueing.
func (s *Service) Preview(ctx context.Context, owner, name string) (apimodel.RepoPreview, error) {
	if s.cache == nil && s.flights == nil {
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
	cacheable := s.cache != nil && repo.ID > 0
	if cacheable {
		if p, ok := s.cache.Get(key); ok {
			return p, nil
		}
	}
	var commit string
	var captured time.Time
	run := func(ctx context.Context) (apimodel.RepoPreview, bool, error) {
		a, err := s.acquireFrom(ctx, repo, owner, name)
		if err != nil {
			return apimodel.RepoPreview{}, false, err
		}
		p, err := s.render(ctx, a)
		if err != nil {
			return apimodel.RepoPreview{}, false, err
		}
		commit, captured = a.Inspection.Revision.Ref, a.CapturedAt
		return p, cacheable && a.Outcome.Cacheable(), nil
	}
	store := func(p apimodel.RepoPreview) { s.cache.Put(key, commit, captured, p) }
	if s.flights != nil {
		return s.flights.do(ctx, key, run, store)
	}
	p, storable, err := run(ctx)
	if err == nil && storable && ctx.Err() == nil {
		store(p)
	}
	return p, err
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
