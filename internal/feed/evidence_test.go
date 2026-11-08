package feed

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
	"github.com/Savio09/gittok-ios-backend/internal/selection"
)

// Exercise the actual HTTP feed path using recorded public metadata, including
// its intentionally coarse role heuristic. Rack's Ruby language must not be
// presented as direct evidence that this library is a backend service.
func TestEvidenceHTTPFeedFromCapturedMetadata(t *testing.T) {
	slugs := []selection.Slug{{Owner: "charmbracelet", Name: "glow"}, {Owner: "rack", Name: "rack"}}
	fetcher := &fakeFetcher{repos: map[string]githubclient.Repo{}}
	for _, slug := range slugs {
		b, err := os.ReadFile(filepath.Join("..", "analysis", "testdata", "g17", slug.Owner+"__"+slug.Name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var capture struct {
			Repo githubclient.Repo `json:"metadata"`
		}
		if err := json.Unmarshal(b, &capture); err != nil {
			t.Fatal(err)
		}
		fetcher.repos[slug.Name] = capture.Repo
	}
	svc := NewService(fetcher, slugs, time.Minute)
	rec := httptest.NewRecorder()
	svc.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/feed", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP %d: %s", rec.Code, rec.Body.String())
	}
	var result apimodel.Feed
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("items = %d", len(result.Items))
	}
	for i, p := range result.Items {
		if p.Slug != slugs[i].Owner+"/"+slugs[i].Name || p.EvidenceMode != apimodel.NotExecuted || p.KindEvidenceStatus != apimodel.EvidenceInferred {
			t.Errorf("HTTP output overstates support or reordered repos: %+v", p)
		}
		for _, m := range p.Metrics {
			if m.EvidenceStatus != apimodel.EvidenceSupported {
				t.Errorf("unmarked metadata metric: %+v", m)
			}
			if m.Source.Ref == "pushed_at" && (m.Key != "last_commit" || m.Label != "since last push") {
				t.Errorf("legacy key or truthful push label changed: %+v", m)
			}
		}
	}
	// Optional integration artifact: the exact response body, without edits.
	if dir := os.Getenv("GITTOK_G18_EXPORT_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "feed.captured.json"), rec.Body.Bytes(), 0644); err != nil {
			t.Fatal(err)
		}
	}
}
