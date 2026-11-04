package githubclient

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
)

// A credential-policy rejection is terminal: it is never retried, it is not
// downgraded to a skipped file, and no data decoded after it is returned.
func TestCredentialPolicyErrorsAreTerminal(t *testing.T) {
	var requests atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(`{"type":"file","encoding":"base64","content":"c2VjcmV0"}`))
	})
	c.backoffBase = 1
	c.check = func(*http.Response) error { return &PolicyError{Reason: "credential scope is not public-only"} }

	_, err := c.GetRepo(context.Background(), "o", "r")
	if !errors.Is(err, ErrCredentialPolicy) || requests.Load() != 1 {
		t.Fatalf("GetRepo: err=%v after %d requests (want a terminal policy error after 1)", err, requests.Load())
	}
	got, err := c.GetFileContents(context.Background(), "o", "r", "0123456789abcdef0123456789abcdef01234567", []string{"a", "b"}, ContentLimits{})
	if !errors.Is(err, ErrCredentialPolicy) {
		t.Fatalf("GetFileContents: err=%v, want the policy error", err)
	}
	if len(got.Files) != 0 || len(got.Skipped) != 0 {
		t.Fatalf("policy error became partial contents: %+v", got)
	}
	var pe *PolicyError
	if !errors.As(err, &pe) || pe.Error() == "" {
		t.Fatalf("policy error lost its type: %v", err)
	}
}

// A policy error after earlier successful reads still ends the batch with no data.
func TestCredentialPolicyErrorMidBatchDiscardsEarlierContents(t *testing.T) {
	var n atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"type":"file","encoding":"base64","content":"c2VjcmV0"}`))
	})
	c.check = func(*http.Response) error {
		if n.Add(1) > 1 {
			return &PolicyError{Reason: "scopes changed"}
		}
		return nil
	}
	got, err := c.GetFileContents(context.Background(), "o", "r", "0123456789abcdef0123456789abcdef01234567", []string{"a", "b", "c"}, ContentLimits{})
	if !errors.Is(err, ErrCredentialPolicy) || len(got.Files) != 0 || len(got.Skipped) != 0 {
		t.Fatalf("mid-batch policy failure returned data: %+v, %v", got, err)
	}
}
