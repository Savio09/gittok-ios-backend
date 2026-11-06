package githubclient

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type testBudgetTransport func(*http.Request) (*http.Response, error)

func (f testBudgetTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRequestBudgetIncludesRetriesAndStopsBeforeBackoff(t *testing.T) {
	var calls atomic.Int32
	c := New("", WithMaxRetries(10), WithHTTPClient(&http.Client{Transport: testBudgetTransport(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: 500, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})}))
	var waits int
	c.sleep = func(time.Duration) { waits++ }
	ctx := NewRequestBudget(2, 1024).Context(context.Background())
	_, err := c.GetRepo(ctx, "o", "r")
	if !errors.Is(err, ErrCallBudget) || calls.Load() != 2 || waits != 1 {
		t.Fatalf("err=%v calls=%d waits=%d", err, calls.Load(), waits)
	}
}

func TestRequestBudgetIncludesRedirects(t *testing.T) {
	var calls int
	c := New("", WithHTTPClient(&http.Client{Transport: testBudgetTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": {"/redirect"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})}))
	ctx := NewRequestBudget(2, 1024).Context(context.Background())
	_, err := c.GetRepo(ctx, "o", "r")
	if !errors.Is(err, ErrCallBudget) || calls != 2 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

type countingBody struct{ remaining, read int }

func (r *countingBody) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := min(len(p), r.remaining)
	for i := range p[:n] {
		p[i] = ' '
	}
	r.remaining -= n
	r.read += n
	return n, nil
}
func (*countingBody) Close() error { return nil }

func TestSuccessfulBodyReadIsBounded(t *testing.T) {
	body := &countingBody{remaining: 100 << 20}
	c := New("", WithHTTPClient(&http.Client{Transport: testBudgetTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: body}, nil
	})}))
	ctx := NewRequestBudget(5, 1024).Context(context.Background())
	_, err := c.GetRepo(ctx, "o", "r")
	if !errors.Is(err, ErrResponseTooLarge) || body.read > 1025 {
		t.Fatalf("err=%v read=%d", err, body.read)
	}
}

func TestCallBudgetEndsFailedContentsBatch(t *testing.T) {
	var calls int
	c := New("", WithMaxRetries(0), WithHTTPClient(&http.Client{Transport: testBudgetTransport(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 404, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}))
	ctx := NewRequestBudget(2, 1024).Context(context.Background())
	got, err := c.GetFileContents(ctx, "o", "r", "main", []string{"a", "b", "c", "d"}, DefaultContentLimits())
	if !errors.Is(err, ErrCallBudget) || calls != 2 || len(got.Skipped) != 2 {
		t.Fatalf("err=%v calls=%d skipped=%v", err, calls, got.Skipped)
	}
}

func TestTreeEntryBudgetStopsBeforeHugeRemainder(t *testing.T) {
	body := &countingBody{remaining: 100 << 20}
	prefix := `{"tree":[{"path":"README.md","type":"blob"},{"path":"go.mod","type":"blob"},`
	c := New("", WithHTTPClient(&http.Client{Transport: testBudgetTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(io.MultiReader(strings.NewReader(prefix), body))}, nil
	})}))
	got, err := c.GetTree(context.Background(), "o", "r", "main", TreeLimits{MaxEntries: 1})
	if err != nil || len(got.Entries) != 1 || !got.Truncated || body.read > 512 {
		t.Fatalf("err=%v entries=%d truncated=%v remainder read=%d", err, len(got.Entries), got.Truncated, body.read)
	}
}

func TestFileLimitCountsMissingFiles(t *testing.T) {
	var calls int
	c := New("", WithHTTPClient(&http.Client{Transport: testBudgetTransport(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 404, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}))
	got, err := c.GetFileContents(context.Background(), "o", "r", "main", []string{"a", "b", "c", "d"}, ContentLimits{MaxFiles: 2})
	if err != nil || calls != 2 || len(got.Skipped) != 4 {
		t.Fatalf("err=%v calls=%d skipped=%v", err, calls, got.Skipped)
	}
}

func TestContentsDecodeOnlyRetainedPrefix(t *testing.T) {
	// A corrupt, huge tail cannot force decoding beyond the retained prefix.
	encoded := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("a", 1<<20))) + "!"
	c := New("", WithHTTPClient(&http.Client{Transport: testBudgetTransport(func(*http.Request) (*http.Response, error) {
		body := `{"type":"file","encoding":"base64","size":1048576,"content":"` + encoded + `"}`
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}))
	got, err := c.GetFileContents(context.Background(), "o", "r", "main", []string{"large.txt"}, ContentLimits{MaxFileBytes: 64})
	if err != nil || len(got.Files) != 1 || len(got.Files[0].Content) != 64 || !got.Files[0].Truncated {
		t.Fatalf("err=%v files=%v skipped=%v", err, got.Files, got.Skipped)
	}
}

func TestResponseLimitDoesNotExposeDetectionByteToJSON(t *testing.T) {
	c := New("", WithHTTPClient(&http.Client{Transport: testBudgetTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"id":1}`))}, nil
	})}))
	ctx := NewRequestBudget(1, 7).Context(context.Background())
	if _, err := c.GetRepo(ctx, "o", "r"); !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("err=%v", err)
	}
}

func TestExcessiveLowLevelLimitsCannotOverflow(t *testing.T) {
	b := NewRequestBudget(math.MaxInt, math.MaxInt)
	if b.remaining > 128 || b.responseBytes > 32<<20 {
		t.Fatalf("unsafe budget: %+v", b)
	}
	c := New("", WithHTTPClient(&http.Client{Transport: testBudgetTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"type":"file","encoding":"base64","size":3,"content":"YWJj"}`))}, nil
	})}))
	got, err := c.GetFileContents(b.Context(context.Background()), "o", "r", "main", []string{"f"}, ContentLimits{MaxFileBytes: math.MaxInt, MaxTotalBytes: math.MaxInt})
	if err != nil || len(got.Files) != 1 || got.Files[0].Content != "abc" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestStandardTransportCannotRetryInternally(t *testing.T) {
	original := http.DefaultTransport.(*http.Transport)
	c := New("", WithHTTPClient(&http.Client{Transport: original}))
	isolated := c.httpClient.Transport.(budgetTransport).base.(*http.Transport)
	if isolated == original || !isolated.DisableKeepAlives || isolated.ForceAttemptHTTP2 || isolated.Protocols.HTTP2() || !isolated.Protocols.HTTP1() {
		t.Fatal("standard transport can reuse or retry connections internally")
	}
}

func TestBudgetedTLSRequestsUseFreshHTTP1Connections(t *testing.T) {
	var calls atomic.Int32
	connections := make(chan string, 2)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.ProtoMajor != 1 {
			t.Errorf("protocol=%s: transport retries can bypass quota", r.Proto)
		}
		connections <- r.RemoteAddr
		io.WriteString(w, `{"id":7}`)
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()
	transport := srv.Client().Transport.(*http.Transport)
	transport.TLSClientConfig.NextProtos = []string{"h2", "http/1.1"}
	transport.ForceAttemptHTTP2 = true
	c := New("", WithBaseURL(srv.URL), WithHTTPClient(srv.Client()), WithMaxRetries(0))
	ctx := NewRequestBudget(2, 1024).Context(context.Background())
	for i := 0; i < 2; i++ {
		if _, err := c.GetRepo(ctx, "o", "r"); err != nil {
			t.Fatal(err)
		}
	}
	if first, second := <-connections, <-connections; first == second {
		t.Fatal("transport reused a connection with hidden retry potential")
	}
	if _, err := c.GetRepo(ctx, "o", "r"); !errors.Is(err, ErrCallBudget) || calls.Load() != 2 {
		t.Fatalf("calls=%d err=%v", calls.Load(), err)
	}
}
