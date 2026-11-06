package walkthrough

import (
	"context"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
	"testing"
)

func TestAcceptedPresentationPreservesPublicCardIdentityAndSourcedMetadata(t *testing.T) {
	g := investigationFixture()
	g.repo.Stars = 7
	g.repo.Language = "Go"
	g.repo.License = "MIT"
	g.repo.Metadata = map[string]githubclient.MetadataField{"stargazers_count": {Value: "7"}, "language": {Value: "Go"}, "license.spdx_id": {Value: "MIT"}}
	s, err := NewService(g, &jobRunner{}, jobConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a, err := s.Start(context.Background(), "owner", "project")
	if err != nil {
		t.Fatal(err)
	}
	job := waitJob(t, s, a.ID)
	if job.State != "ready" || job.Preview == nil {
		t.Fatalf("%+v", job)
	}
	p := job.Preview
	if p.ID != "owner/project" || p.Slug != "owner/project" {
		t.Fatal("story changed card identity", p.ID)
	}
	if p.Language != "Go" || p.License != "MIT" {
		t.Fatal("story dropped public metadata")
	}
	found := false
	for _, m := range p.Metrics {
		if m.Key == "stars" && m.Value == "7" && m.Source != nil && m.Source.Ref == "stargazers_count" {
			found = true
		}
	}
	if !found {
		t.Fatal("story dropped sourced activity metadata")
	}
}
