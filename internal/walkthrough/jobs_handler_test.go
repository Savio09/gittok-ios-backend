package walkthrough

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
