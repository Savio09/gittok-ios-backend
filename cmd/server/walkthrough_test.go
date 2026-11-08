package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/config"
)

func TestWalkthroughRoutesExplicitlyUnavailableWithoutProvider(t *testing.T) {
	handler := newRouter(nil, nil)
	for _, request := range []*http.Request{httptest.NewRequest("POST", "/v1/repos/arbitrary/project/walkthrough", strings.NewReader("{}")), httptest.NewRequest("GET", "/v1/walkthroughs/0123456789abcdef0123456789abcdef", nil)} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, request)
		if w.Code != 503 || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "secret") {
			t.Fatal("internal configuration exposed")
		}
	}
}
func TestWalkthroughDisabledBootDoesNotInitializeAccounting(t *testing.T) {
	svc, cleanup, err := walkthroughService(context.Background(), config.Config{})
	if err != nil || svc != nil || cleanup == nil {
		t.Fatal("disabled service failed", err)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
}
