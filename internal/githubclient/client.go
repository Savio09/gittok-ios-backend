package githubclient

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.github.com"

const (
	defaultMaxRetries  = 3
	defaultBackoffBase = 500 * time.Millisecond
	maxBackoff         = 30 * time.Second
)

// Client talks to the GitHub REST API. It is safe for concurrent use. Construct
// one with New and share it; it owns no per-request state.
type Client struct {
	httpClient  *http.Client
	token       string
	baseURL     string
	userAgent   string
	maxRetries  int
	backoffBase time.Duration
	sleep       func(time.Duration) // optional test hook; production uses a cancellable timer
	// check, when set, vets every response before any body is decoded; a
	// non-nil result (normally a *PolicyError) is terminal.
	check func(*http.Response) error
	mode  string
}

// Option configures a Client. Options exist mainly so tests can point the
// client at an httptest server and so callers can supply their own transport.
type Option func(*Client)

// WithBaseURL overrides the API base URL (default https://api.github.com).
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

// WithHTTPClient supplies the underlying HTTP client (default: 15s timeout).
// Standard transports are cloned and use fresh HTTP/1 connections so hidden
// transport retries cannot evade the quota. A custom RoundTripper must perform
// at most one network attempt per RoundTrip and honor request cancellation.
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

// WithMaxRetries sets how many times a transient failure (5xx, 429, secondary
// rate limit, transport error) is retried before giving up. Zero disables
// retries.
func WithMaxRetries(n int) Option {
	return func(c *Client) {
		if n >= 0 {
			c.maxRetries = n
		}
	}
}

// WithBackoffBase sets the base delay for exponential backoff between retries.
func WithBackoffBase(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.backoffBase = d
		}
	}
}

// New returns a Client authenticated with token. The token may be empty for
// unauthenticated access, but GitHub's low anonymous rate limit makes that
// impractical, so the backend always supplies one.
func New(token string, opts ...Option) *Client {
	c := &Client{
		httpClient:  &http.Client{Timeout: 15 * time.Second},
		token:       token,
		baseURL:     defaultBaseURL,
		userAgent:   "gittok-backend",
		maxRetries:  defaultMaxRetries,
		backoffBase: defaultBackoffBase,
	}
	for _, opt := range opts {
		opt(c)
	}
	// Copy the caller's client before wrapping its transport; a shared HTTP
	// client must never acquire another service's mutable configuration.
	copied := *c.httpClient
	transport := copied.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	if standard, ok := transport.(*http.Transport); ok {
		// A reused connection or HTTP/2 stream can retry inside RoundTrip,
		// invisible to our attempt counter. Fresh HTTP/1 connections keep all
		// retries in get(), where the analysis quota can stop them.
		isolated := standard.Clone()
		isolated.DisableKeepAlives = true
		isolated.ForceAttemptHTTP2 = false
		isolated.Protocols = new(http.Protocols)
		isolated.Protocols.SetHTTP1(true)
		// Clone initializes HTTP/2 defaults on the original transport. Clear
		// their ALPN offers and handlers too, including explicitly supplied H2.
		if isolated.TLSClientConfig == nil {
			isolated.TLSClientConfig = &tls.Config{}
		}
		isolated.TLSClientConfig.NextProtos = []string{"http/1.1"}
		isolated.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
		transport = isolated
	}
	copied.Transport = budgetTransport{base: transport}
	c.httpClient = &copied
	c.mode = "anonymous"
	if token != "" {
		c.mode = "token"
		c.restrictRedirects() // a credential never follows a redirect off-origin
	}
	return c
}

// get issues a GET for path (joined to the base URL) and decodes a JSON body
// into v. It centralizes headers, auth, status-to-typed-error mapping, and
// retry with exponential backoff for transient failures. Terminal outcomes
// (success, 404, primary rate limit, other 4xx) return immediately; transient
// ones (5xx, 429, secondary rate limit, transport errors) are retried up to
// maxRetries, honoring a Retry-After header when present.
func (c *Client) get(ctx context.Context, path string, v any) error {
	var lastErr error
	var retryAfter time.Duration

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := requestBudget(ctx).check(false); err != nil {
			return err
		}
		if attempt > 0 {
			if err := c.waitBackoff(ctx, backoffDelay(attempt, retryAfter, c.backoffBase)); err != nil {
				return err
			}
			retryAfter = 0
		}

		resp, err := c.doOnce(ctx, path)
		if err != nil {
			var policy *PolicyError
			if errors.As(err, &policy) {
				return policy // a refused redirect is terminal, not transient
			}
			if errors.Is(err, ErrCallBudget) || ctx.Err() != nil {
				return err
			}
			lastErr = err // transport error — transient, retry
			continue
		}

		if c.check != nil {
			if err := c.check(resp); err != nil {
				resp.Body.Close()
				return err // terminal: never decode or retry a rejected response
			}
		}
		resp.Body = capResponse(ctx, resp.Body)
		done, ra, err := classify(resp, c.baseURL+path, v)
		resp.Body.Close()
		if done {
			return err
		}
		lastErr = err
		retryAfter = ra
	}
	return lastErr
}

// waitBackoff cancels its production timer when the work ends; even an enormous
// Retry-After cannot leave sleeping goroutines behind after the deadline.
func (c *Client) waitBackoff(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.sleep == nil {
		timer := time.NewTimer(d)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		}
	}
	// The injected hook lets tests observe/advance backoff without wall time.
	done := make(chan struct{})
	go func() {
		c.sleep(d)
		close(done)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

// doOnce performs a single GET. The caller is responsible for closing the
// response body.
func (c *Client) doOnce(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", c.userAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return c.httpClient.Do(req)
}

// classify interprets one response. It returns done=true for a terminal outcome
// (with err nil on success or the typed error otherwise) and done=false for a
// transient one that should be retried (with the suggested Retry-After delay).
func classify(resp *http.Response, url string, v any) (done bool, retryAfter time.Duration, err error) {
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		dec := json.NewDecoder(resp.Body)
		var decErr error
		if streaming, ok := v.(interface{ decodeJSON(*json.Decoder) error }); ok {
			decErr = streaming.decodeJSON(dec)
		} else {
			decErr = dec.Decode(v)
		}
		if decErr != nil {
			return true, 0, decErr
		}
		return true, 0, nil

	case resp.StatusCode == http.StatusNotFound:
		return true, 0, ErrNotFound

	case resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0":
		// Primary rate limit: the quota is spent and resets far in the future,
		// so retrying now is pointless — fail fast with a distinct error.
		return true, 0, ErrRateLimited

	case resp.StatusCode == http.StatusTooManyRequests ||
		(resp.StatusCode == http.StatusForbidden && resp.Header.Get("Retry-After") != ""):
		// Secondary / abuse rate limit: transient, honor Retry-After and retry.
		return false, parseRetryAfter(resp.Header.Get("Retry-After")), ErrRateLimited

	case resp.StatusCode >= 500:
		return false, 0, &APIError{StatusCode: resp.StatusCode, URL: url, Message: messageFromBody(resp.Body)}

	default:
		return true, 0, &APIError{StatusCode: resp.StatusCode, URL: url, Message: messageFromBody(resp.Body)}
	}
}

// backoffDelay returns the wait before attempt n (1-based). It uses exponential
// backoff from base, capped, and never less than a Retry-After hint.
func backoffDelay(attempt int, retryAfter, base time.Duration) time.Duration {
	exp := time.Duration(float64(base) * math.Pow(2, float64(attempt-1)))
	if exp > maxBackoff {
		exp = maxBackoff
	}
	if retryAfter > exp {
		return retryAfter
	}
	return exp
}

// parseRetryAfter reads a Retry-After header expressed as a number of seconds.
func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	return 0
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
