package githubclient

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

// Public-repository credential policy.
//
// GitHub documents a classic personal access token with no OAuth scopes as
// granting "read-only access to public information", and reports the scopes a
// token has authorized in the X-OAuth-Scopes response header
// (https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/scopes-for-oauth-apps).
// X-Accepted-OAuth-Scopes only names what an endpoint checks for, so it is
// never evidence of a grant. Fine-grained, OAuth-app and GitHub App tokens
// carry permissions this policy cannot verify from that header, so they are
// rejected rather than guessed at.

const classicPATPrefix = "ghp_"

// NewPublic returns a client for reading public repositories. An empty token
// selects explicit anonymous mode. A non-empty token must be a classic personal
// access token, and every response is vetted before its body is decoded: a
// success must carry an X-OAuth-Scopes header whose every value is empty, and
// a 401 ends the read. Either failure is a terminal *PolicyError — a rejected
// credential is never downgraded to anonymous access.
func NewPublic(token string, opts ...Option) (*Client, error) {
	if token != "" && !strings.HasPrefix(token, classicPATPrefix) {
		return nil, &PolicyError{Reason: "only a classic personal access token with no OAuth scopes is supported for public reads"}
	}
	c := New(token, opts...)
	if token != "" {
		c.mode = "public-token"
		c.check = publicScopeCheck
	}
	return c, nil
}

// Mode names how the client authenticates: "anonymous", "token" (an
// unvetted credential) or "public-token" (the vetted no-scope policy).
func (c *Client) Mode() string { return c.mode }

// VerifyCredential makes one request that does not count against the REST
// rate limit, so a misconfigured public credential fails at startup. Every
// later response is still vetted: scopes can change after startup.
func (c *Client) VerifyCredential(ctx context.Context) error {
	if c.check == nil {
		return nil
	}
	var discard struct{}
	return c.get(ctx, "/rate_limit", &discard)
}

func publicScopeCheck(resp *http.Response) error {
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return &PolicyError{Reason: "the configured credential was rejected"}
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return nil // no data will be decoded from a non-success response
	}
	values, present := resp.Header[http.CanonicalHeaderKey("X-OAuth-Scopes")]
	if !present {
		return &PolicyError{Reason: "the response carried no granted-scope evidence"}
	}
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return &PolicyError{Reason: "the credential has OAuth scopes; only a credential with none is accepted"}
		}
	}
	return nil
}

// restrictRedirects copies the HTTP client (never mutating a caller's shared
// one) so a credential-bearing request follows redirects only within origin.
func (c *Client) restrictRedirects() {
	origin, err := url.Parse(c.baseURL)
	if err != nil {
		return
	}
	copied := *c.httpClient
	prev := copied.CheckRedirect
	copied.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != origin.Scheme || req.URL.Host != origin.Host {
			return &PolicyError{Reason: "a redirect left the GitHub API origin"}
		}
		if prev != nil {
			return prev(req, via)
		}
		if len(via) >= 10 {
			return http.ErrUseLastResponse
		}
		return nil
	}
	c.httpClient = &copied
}
