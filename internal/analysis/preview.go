package analysis

import (
	"context"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/preview"
)

// Preview composes reusable acquisition and pure rendering. Only retained,
// validated evidence survives a partial read. No target code is executed.
func (s *Service) Preview(ctx context.Context, owner, name string) (apimodel.RepoPreview, error) {
	a, err := s.Acquire(ctx, owner, name)
	if err != nil {
		return apimodel.RepoPreview{}, err
	}
	if err := ctx.Err(); err != nil {
		return apimodel.RepoPreview{}, err
	}
	p := preview.Build(preview.Input{Repo: a.Repo, Inspection: a.Inspection, CapturedAt: a.CapturedAt, Coverage: a.Coverage, Limitations: a.Limitations})
	if err := ctx.Err(); err != nil {
		return apimodel.RepoPreview{}, err
	}
	return p, nil
}
