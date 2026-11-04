package githubclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// fakeClassicPAT is built at runtime so no token-shaped literal is stored.
func fakeClassicPAT() string { return "ghp_" + strings.Repeat("0", 36) }

const repoBody = `{"id":1,"owner":{"login":"o"},"name":"r","full_name":"o/r","private":false,"default_branch":"main"}`

func publicServer(t *testing.T, scopes func(n int32) []string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := n.Add(1)
		if r.Header.Get("Authorization") != "Bearer "+fakeClassicPAT() {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if v := scopes(i); v != nil {
			w.Header()["X-Oauth-Scopes"] = v
		}
		w.Header().Set("X-Accepted-OAuth-Scopes", "repo") // endpoint requirement, not a grant
		_, _ = w.Write([]byte(repoBody))
	}))
	t.Cleanup(srv.Close)
	return srv, &n
}

func TestPublicCredentialAcceptsOnlyAnEmptyGrantedScopeList(t *testing.T) {
	for name, tc := range map[string]struct {
		scopes []string
		ok     bool
	}{
		"present and empty":        {[]string{""}, true},
		"duplicate empty values":   {[]string{"", ""}, true},
		"absent header":            {nil, false},
		"private repository scope": {[]string{"repo"}, false},
		"public write scope":       {[]string{"public_repo"}, false},
		"several scopes":           {[]string{"read:org, gist"}, false},
		"empty then privileged":    {[]string{"", "repo"}, false},
	} {
		t.Run(name, func(t *testing.T) {
			srv, _ := publicServer(t, func(int32) []string { return tc.scopes })
			c, err := NewPublic(fakeClassicPAT(), WithBaseURL(srv.URL), WithMaxRetries(0))
			if err != nil {
				t.Fatal(err)
			}
			repo, err := c.GetRepo(context.Background(), "o", "r")
			if tc.ok && (err != nil || repo.FullName != "o/r") {
				t.Fatalf("supported credential failed: %+v %v", repo, err)
			}
			if !tc.ok {
				if !errors.Is(err, ErrCredentialPolicy) {
					t.Fatalf("err = %v, want a credential-policy rejection", err)
				}
				if repo.FullName != "" {
					t.Fatalf("decoded repository data from a rejected response: %+v", repo)
				}
			}
		})
	}
}

func TestPublicCredentialScopeChangesAreCaughtOnLaterReads(t *testing.T) {
	srv, _ := publicServer(t, func(n int32) []string {
		if n == 1 {
			return []string{""}
		}
		return []string{"repo"} // the token gained a scope after startup
	})
	c, err := NewPublic(fakeClassicPAT(), WithBaseURL(srv.URL), WithMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.VerifyCredential(context.Background()); err != nil {
		t.Fatalf("startup verification: %v", err)
	}
	if _, err := c.GetRepo(context.Background(), "o", "r"); !errors.Is(err, ErrCredentialPolicy) {
		t.Fatalf("expanded scopes accepted after startup: %v", err)
	}
}

func TestPublicCredentialRevocationIsTerminal(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
	}))
	defer srv.Close()
	c, err := NewPublic(fakeClassicPAT(), WithBaseURL(srv.URL), WithBackoffBase(1))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetRepo(context.Background(), "o", "r"); !errors.Is(err, ErrCredentialPolicy) || n.Load() != 1 {
		t.Fatalf("rejected credential: err=%v after %d requests (want terminal after 1)", err, n.Load())
	}
}

func TestPublicCredentialRejectsUnsupportedTokenFamilies(t *testing.T) {
	for _, prefix := range []string{"github_pat_", "gho_", "ghu_", "ghs_", "ghr_", "not-a-token-"} {
		token := prefix + strings.Repeat("0", 36)
		_, err := NewPublic(token)
		if !errors.Is(err, ErrCredentialPolicy) {
			t.Errorf("%s token accepted: %v", prefix, err)
		}
		if err != nil && strings.Contains(err.Error(), token) {
			t.Errorf("error leaks the credential: %v", err)
		}
	}
	c, err := NewPublic("")
	if err != nil || c.Mode() != "anonymous" {
		t.Fatalf("empty credential must select explicit anonymous mode: %v %q", err, c.Mode())
	}
	if c, _ := NewPublic(fakeClassicPAT()); c.Mode() != "public-token" {
		t.Fatalf("mode = %q", c.Mode())
	}
}

// A credential-bearing request never follows a redirect off the API origin,
// and the caller's HTTP client is left unmodified.
func TestPublicCredentialRedirectsStayOnTheAPIOrigin(t *testing.T) {
	var leaked atomic.Bool
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			leaked.Store(true)
		}
		w.Header()["X-Oauth-Scopes"] = []string{""}
		_, _ = w.Write([]byte(repoBody))
	}))
	defer other.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/o/moved" {
			http.Redirect(w, r, "/repos/o/r", http.StatusMovedPermanently)
			return
		}
		if r.URL.Path == "/repos/o/away" {
			http.Redirect(w, r, other.URL+"/repos/o/r", http.StatusMovedPermanently)
			return
		}
		w.Header()["X-Oauth-Scopes"] = []string{""}
		_, _ = w.Write([]byte(repoBody))
	}))
	defer api.Close()
	shared := &http.Client{}
	c, err := NewPublic(fakeClassicPAT(), WithBaseURL(api.URL), WithHTTPClient(shared), WithMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}
	if repo, err := c.GetRepo(context.Background(), "o", "moved"); err != nil || repo.FullName != "o/r" {
		t.Fatalf("same-origin redirect: %+v %v", repo, err)
	}
	if _, err := c.GetRepo(context.Background(), "o", "away"); !errors.Is(err, ErrCredentialPolicy) {
		t.Fatalf("cross-origin redirect: err = %v", err)
	}
	if leaked.Load() {
		t.Fatal("credential sent to another origin")
	}
	if shared.CheckRedirect != nil {
		t.Fatal("the caller's shared HTTP client was mutated")
	}
}
