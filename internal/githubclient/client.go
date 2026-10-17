package githubclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.github.com"

// Client talks to the GitHub REST API. It is safe for concurrent use. Construct
// one with New and share it; it owns no per-request state.
type Client struct {
	httpClient *http.Client
	token      string
	baseURL    string
	userAgent  string
}

// Option configures a Client. Options exist mainly so tests can point the
// client at an httptest server and so callers can supply their own transport.
type Option func(*Client)

// WithBaseURL overrides the API base URL (default https://api.github.com).
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

// WithHTTPClient supplies the underlying HTTP client (default: 15s timeout).
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) {
		if h != nil {
			c.httpClient = h
		}
	}
}

// WithUserAgent sets the User-Agent header. GitHub requires a User-Agent.
func WithUserAgent(ua string) Option {
	return func(c *Client) { c.userAgent = ua }
}

// New returns a Client authenticated with token. The token may be empty for
// unauthenticated access, but GitHub's low anonymous rate limit makes that
// impractical, so the backend always supplies one.
func New(token string, opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		token:      token,
		baseURL:    defaultBaseURL,
		userAgent:  "gittok-backend",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// get issues a GET for path (joined to the base URL) and decodes a JSON body
// into v. It centralizes headers, auth, and the mapping of HTTP status codes
// onto this package's typed errors, so the typed methods stay small.
func (c *Client) get(ctx context.Context, path string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", c.userAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			return err
		}
		return nil
	case resp.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0":
		return ErrRateLimited
	case resp.StatusCode == http.StatusTooManyRequests:
		return ErrRateLimited
	default:
		return &APIError{
			StatusCode: resp.StatusCode,
			URL:        c.baseURL + path,
			Message:    messageFromBody(resp.Body),
		}
	}
}

// messageFromBody best-effort extracts GitHub's "message" field from an error
// body. It never fails: an unparseable body yields an empty message.
func messageFromBody(r io.Reader) string {
	var body struct {
		Message string `json:"message"`
	}
	_ = json.NewDecoder(io.LimitReader(r, 1<<20)).Decode(&body)
	return body.Message
}
