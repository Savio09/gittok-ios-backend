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
	// Admission comes first: a refused caller makes no GitHub request at all.
	if s.flights != nil {
		if !s.flights.enter() {
			return apimodel.RepoPreview{}, ErrOverloaded
		}
		defer s.flights.exit()
	}
	workCtx, cancel, budget, deadline := s.workContext(ctx)
	defer cancel()
	repo, err := s.metadata(workCtx, owner, name)
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
	if s.afterCacheMiss != nil {
		s.afterCacheMiss()
	}
	var commit string
	var captured time.Time
	run := func(ctx context.Context) (apimodel.RepoPreview, bool, error) {
		// A previous leader may have stored this identity's result after this
		// caller's lookup missed: serve it as the hit it is, without restoring
		// it (its provenance and expiry stay those of the original capture).
		if cacheable {
			if p, ok := s.cache.Get(key); ok {
				return p, false, nil
			}
		}
		bounded, stop := context.WithDeadline(ctx, deadline)
		defer stop()
		a, err := s.acquireFrom(ctx, budget.Context(bounded), repo, owner, name)
		if err != nil {
			return apimodel.RepoPreview{}, false, err
		}
		p, err := s.render(ctx, a)
		if err != nil {
			return apimodel.RepoPreview{}, false, err
		}
		commit, captured = a.Inspection.Revision.Ref, a.CapturedAt
		return p, cacheable && a.Outcome.Cacheable() && bounded.Err() == nil, nil
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
	bounded := ctx
	if !a.deadline.IsZero() {
		var cancel context.CancelFunc
		bounded, cancel = context.WithDeadline(ctx, a.deadline)
		defer cancel()
	}
	p := preview.BuildContext(bounded, preview.Input{Repo: a.Repo, Inspection: a.Inspection, CapturedAt: a.CapturedAt, Coverage: a.Coverage, Limitations: a.Limitations})
	if err := ctx.Err(); err != nil {
		return apimodel.RepoPreview{}, err
	}
	return p, nil
}
