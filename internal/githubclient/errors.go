package githubclient

import (
	"errors"
	"fmt"
)

// ErrNotFound is returned when GitHub responds 404 — the repository (or other
// resource) does not exist or is not visible to the token.
var ErrNotFound = errors.New("githubclient: not found")

// ErrRateLimited is returned when GitHub rejects a request because the token's
// rate limit is exhausted.
var ErrRateLimited = errors.New("githubclient: rate limited")

// APIError is any other non-2xx response from the GitHub API. It carries enough
// to diagnose the failure without re-reading the response.
type APIError struct {
	StatusCode int
	URL        string
	// Message is GitHub's "message" field, when the body carried one.
	Message string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("githubclient: GitHub API %d for %s: %s", e.StatusCode, e.URL, e.Message)
	}
	return fmt.Sprintf("githubclient: GitHub API %d for %s", e.StatusCode, e.URL)
}
