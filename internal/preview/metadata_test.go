package preview

import (
	"testing"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

func TestFromMetadataMapsIdentityAndMetrics(t *testing.T) {
	r := githubclient.Repo{
		Owner: "pocketbase", Name: "pocketbase", FullName: "pocketbase/pocketbase",
		Description: "Open Source realtime backend in 1 file",
		Language:    "Go", Stars: 61456, License: "MIT",
		HTMLURL:  "https://github.com/pocketbase/pocketbase",
		PushedAt: time.Now().Add(-72 * time.Hour),
		Topics:   []string{"go", "backend", "realtime"},
	}
	r.Metadata = map[string]githubclient.MetadataField{
		"stargazers_count": {Value: "61456"},
		"license.spdx_id":  {Value: "MIT"},
		"pushed_at":        {Value: r.PushedAt.Format(time.RFC3339)},
	}
	p := FromMetadata(r)

	if p.ID != "pocketbase/pocketbase" || p.Slug != "pocketbase/pocketbase" || p.Owner != "pocketbase" {
		t.Errorf("identity wrong: %+v", p)
	}
	if p.Kind != apimodel.KindBackend {
		t.Errorf("kind = %q, want backend", p.Kind)
	}
	if p.License != "MIT" || p.GitHubURL == "" {
		t.Errorf("license/url wrong: %+v", p)
	}
	// Metadata-only: the evidence layer must stay empty.
	if len(p.Sections) != 0 || p.Fit != nil || p.Media != nil {
		t.Errorf("evidence layer should be empty for metadata-only preview: %+v", p)
	}
	if got := metricValue(p, "stars"); got != "61.5k" {
		t.Errorf("stars metric = %q, want 61.5k", got)
	}
	if got := metricValue(p, "license"); got != "MIT" {
		t.Errorf("license metric = %q, want MIT", got)
	}
	if metricValue(p, "last_commit") == "" {
		t.Errorf("last_commit metric missing")
	}
}

func metricValue(p apimodel.RepoPreview, key string) string {
	for _, m := range p.Metrics {
		if m.Key == key {
			return m.Value
		}
	}
	return ""
}

func TestInferKind(t *testing.T) {
	cases := []struct {
		name string
		repo githubclient.Repo
		want apimodel.RepoKind
	}{
		{"cli by topic", githubclient.Repo{Language: "Rust", Topics: []string{"rust", "cli", "terminal"}}, apimodel.KindCLI},
		{"cli via hyphenated prompt", githubclient.Repo{Language: "Rust", Topics: []string{"rust", "fish-prompt", "zsh-prompt"}}, apimodel.KindCLI},
		{"client topic is not cli", githubclient.Repo{Language: "Go", Topics: []string{"client", "api"}}, apimodel.KindBackend},
		{"frontend by lang", githubclient.Repo{Language: "TypeScript", Topics: []string{"canvas", "react"}}, apimodel.KindFrontend},
		{"backend by lang", githubclient.Repo{Language: "Go", Topics: []string{"backend"}}, apimodel.KindBackend},
		{"other", githubclient.Repo{Language: "Haskell"}, apimodel.KindOther},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := inferKind(c.repo); got != c.want {
				t.Errorf("inferKind = %q, want %q", got, c.want)
			}
		})
	}
}

func TestFormatCount(t *testing.T) {
	cases := map[int]string{0: "0", 42: "42", 999: "999", 1000: "1k", 18432: "18.4k", 134045: "134k", 1_250_000: "1.2m"}
	for n, want := range cases {
		if got := formatCount(n); got != want {
			t.Errorf("formatCount(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestHumanizeDuration(t *testing.T) {
	day := 24 * time.Hour
	cases := []struct {
		d    time.Duration
		want string
	}{
		{2 * time.Hour, "today"},
		{3 * day, "3d"},
		{20 * day, "2w"},
		{90 * day, "3mo"},
		{400 * day, "1y"},
	}
	for _, c := range cases {
		if got := humanizeDuration(c.d); got != c.want {
			t.Errorf("humanizeDuration(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}
