package walkthrough

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

func TestJobHandlerRejectsInputOverridesAndTrailingBodiesBeforeAcquisition(t *testing.T) {
	g := investigationFixture()
	s, err := NewService(g, &jobRunner{}, jobConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	mux := http.NewServeMux()
	mux.Handle("POST /v1/repos/{owner}/{name}/walkthrough", s.Handler())
	for _, body := range []string{`{"model":"other"}`, `{"url":"http://internal"}`, `null`, `[]`, `{} {}`, `{}` + strings.Repeat(" ", 1024), `{broken`} {
		request := httptest.NewRequest("POST", "/v1/repos/owner/project/walkthrough", strings.NewReader(body))
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != 400 || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("body length%d: status%d %s", len(body), response.Code, response.Body.String())
		}
	}
	if g.metadata != 0 {
		t.Fatal("invalid input reached GitHub or paid work")
	}
}

func TestJobHTTPPublishesReviewedStoryboardWithHandoffAndPinnedEvidence(t *testing.T) {
	_, g, draft, review := engineInvestigation(t)
	calls := 0
	cfg := engineConfig()
	cfg.MaxCalls = 4
	engine, err := NewEngine(cfg, engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(*http.Request) (*http.Response, error) {
		calls++
		switch calls {
		case 1:
			return engineHTTP(t, engineCall(t, "read_file", map[string]any{"path": "src/report.rs", "start_line": 1, "line_count": 4}, "read")), nil
		case 2:
			return engineHTTP(t, engineCall(t, "finish_reading", map[string]any{}, "finish")), nil
		case 3:
			return engineHTTP(t, engineCall(t, "submit_story", draft, "submit")), nil
		case 4:
			return engineHTTP(t, engineMessage(t, review)), nil
		default:
			return nil, ErrProviderUnavailable
		}
	})}))
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(g, engine, jobConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	mux := http.NewServeMux()
	mux.Handle("POST /v1/repos/{owner}/{name}/walkthrough", service.Handler())
	mux.Handle("GET /v1/walkthroughs/{id}", service.Handler())
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest("POST", "/v1/repos/owner/project/walkthrough", strings.NewReader("{}")))
	if response.Code != http.StatusAccepted {
		t.Fatalf("start: %d %s", response.Code, response.Body.String())
	}
	var started Job
	if err := json.Unmarshal(response.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	job := waitJob(t, service, started.ID)
	if job.State != "ready" {
		t.Fatalf("story failed: %+v", job)
	}
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest("GET", "/v1/walkthroughs/"+started.ID, nil))
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("poll: %d", response.Code)
	}
	var published Job
	if err := json.Unmarshal(response.Body.Bytes(), &published); err != nil {
		t.Fatal(err)
	}
	if published.Preview == nil || len(published.Preview.Sections) != 3 || published.Preview.Sections[2].Kind != apimodel.SectionHandoff {
		t.Fatal("HTTP lost visual film or handoff")
	}
	for _, section := range published.Preview.Sections {
		if section.Storyboard == nil || section.Storyboard.Version != 1 || len(section.Storyboard.Beats) != len(section.Claims) || section.EvidenceMode != apimodel.NotExecuted {
			t.Fatal("HTTP lost ordered source-only storyboard")
		}
		for i, claim := range section.Claims {
			if claim.Visual == nil || section.Storyboard.Beats[i].ClaimID != claim.ID || claim.EvidenceStatus != apimodel.EvidenceInferred || !strings.Contains(claim.Source.URL, "/blob/") {
				t.Fatal("HTTP lost accepted visual fact, reference or pinned provenance")
			}
		}
	}
}
