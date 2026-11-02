package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
	"github.com/Savio09/gittok-ios-backend/internal/preview"
)

// Handler mounts on GET /v1/repos/{id}/preview. Only the anonymous public
// service can be exposed; a trusted token service fails closed.
func (s *Service) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Context().Err() != nil {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		fail := func(status int, message string) {
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
		}
		if !s.publicOnly {
			fail(503, "preview service unavailable")
			return
		}
		owner, name, ok := strings.Cut(r.PathValue("id"), "/")
		if !ok || !preview.ValidIdentity(owner, name) {
			fail(400, "invalid repository id; expected owner/name")
			return
		}
		p, err := s.Preview(r.Context(), owner, name)
		if r.Context().Err() != nil || errors.Is(err, context.Canceled) {
			return
		}
		if err != nil {
			switch {
			case errors.Is(err, githubclient.ErrNotFound):
				fail(404, "repository unavailable")
			case errors.Is(err, context.DeadlineExceeded):
				fail(504, "preview deadline exceeded")
			case errors.Is(err, githubclient.ErrRateLimited):
				fail(503, "GitHub temporarily unavailable")
			default:
				var api *githubclient.APIError
				if errors.As(err, &api) && (api.StatusCode == 401 || api.StatusCode == 403 || api.StatusCode == 429) {
					fail(503, "GitHub temporarily unavailable")
				} else {
					fail(502, "GitHub acquisition failed")
				}
			}
			return
		}
		_ = json.NewEncoder(w).Encode(p)
	})
}
