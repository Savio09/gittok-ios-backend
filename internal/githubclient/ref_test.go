package githubclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveRef(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/o/r/commits/main" {
			_, _ = w.Write([]byte(`{"sha":"0123456789abcdef0123456789abcdef01234567"}`))
			return
		}
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	}))
	defer srv.Close()
	c := New("", WithBaseURL(srv.URL), WithMaxRetries(0))

	sha, err := c.ResolveRef(context.Background(), "o", "r", "main")
	if err != nil || sha != "0123456789abcdef0123456789abcdef01234567" {
		t.Errorf("ResolveRef = %q, %v", sha, err)
	}
	if _, err := c.ResolveRef(context.Background(), "o", "r", "gone"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing ref: err = %v, want ErrNotFound", err)
	}
}
