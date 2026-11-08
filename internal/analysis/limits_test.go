package analysis

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

type limitSpy struct {
	*fakeGitHub
	deadlines     []time.Time
	treeLimits    githubclient.TreeLimits
	contentLimits githubclient.ContentLimits
}

func (g *limitSpy) record(ctx context.Context) {
	d, _ := ctx.Deadline()
	g.deadlines = append(g.deadlines, d)
}
func (g *limitSpy) GetRepo(ctx context.Context, _, _ string) (githubclient.Repo, error) {
	g.record(ctx)
	return g.repo, nil
}
func (g *limitSpy) ResolveRef(ctx context.Context, _, _, _ string) (string, error) {
	g.record(ctx)
	return "0123456789abcdef0123456789abcdef01234567", nil
}
func (g *limitSpy) GetTree(ctx context.Context, _, _, ref string, l githubclient.TreeLimits) (githubclient.FileTree, error) {
	g.record(ctx)
	g.treeLimits = l
	return g.fakeGitHub.GetTree(ctx, "o", "r", ref, l)
}
func (g *limitSpy) GetFileContents(ctx context.Context, _, _, ref string, p []string, l githubclient.ContentLimits) (githubclient.FetchedContents, error) {
	g.record(ctx)
	g.contentLimits = l
	return g.fakeGitHub.GetFileContents(ctx, "o", "r", ref, p, l)
}

func TestAnalysisSharesDeadlineAcrossMetadataAndAcquisition(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprint(shared), func(t *testing.T) {
			g := &limitSpy{fakeGitHub: &fakeGitHub{repo: publicRepo()}}
			s := NewService(g)
			if shared {
				s.publicOnly = true
				s.UseAdmission(DefaultAdmissionLimits())
			}
			l := DefaultLimits()
			l.Timeout = time.Second
			l.MaxTreeEntries = 7
			l.MaxDepth = 3
			l.MaxFiles = 2
			l.MaxFileBytes = 19
			l.MaxTotalBytes = 31
			if err := s.UseLimits(l); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Preview(context.Background(), "o", "r"); err != nil {
				t.Fatal(err)
			}
			if len(g.deadlines) != 5 || g.deadlines[0].IsZero() {
				t.Fatalf("deadlines=%v", g.deadlines)
			}
			for _, d := range g.deadlines {
				if !d.Equal(g.deadlines[0]) {
					t.Fatalf("renewed deadline: %v", g.deadlines)
				}
			}
			if g.treeLimits.MaxEntries != 7 || g.treeLimits.MaxDepth != 3 || g.contentLimits.MaxFiles != 2 || g.contentLimits.MaxFileBytes != 19 || g.contentLimits.MaxTotalBytes != 31 {
				t.Fatalf("tree=%+v content=%+v", g.treeLimits, g.contentLimits)
			}
		})
	}
}

func TestHugeRepositoryRetainsOnlyConfiguredEvidence(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	encoded := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("a", 1<<20)))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		switch {
		case r.URL.Path == "/repos/o/r":
			fmt.Fprint(w, `{"id":7,"owner":{"login":"o"},"name":"r","private":false,"default_branch":"main"}`)
		case strings.Contains(r.URL.Path, "/commits/"):
			fmt.Fprint(w, `{"sha":"0123456789abcdef0123456789abcdef01234567"}`)
		case strings.Contains(r.URL.Path, "/git/trees/"):
			fmt.Fprint(w, `{"tree":[`)
			for i := 0; i < 100000; i++ {
				if i > 0 {
					fmt.Fprint(w, ",")
				}
				if _, err := fmt.Fprintf(w, `{"path":"docs/%06d/README.md","type":"blob","mode":"100644"}`, i); err != nil {
					return
				}
			}
			fmt.Fprint(w, `],"truncated":false}`)
		case strings.Contains(r.URL.Path, "/contents/"):
			json.NewEncoder(w).Encode(map[string]any{"type": "file", "encoding": "base64", "size": 1 << 20, "content": encoded})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	s := NewPublicService(githubclient.WithBaseURL(srv.URL), githubclient.WithMaxRetries(0))
	l := DefaultLimits()
	l.MaxTreeEntries = 20
	l.MaxFiles = 2
	l.MaxFileBytes = 32
	l.MaxTotalBytes = 48
	l.MaxAPICalls = 8
	if err := s.UseLimits(l); err != nil {
		t.Fatal(err)
	}
	a, err := s.Acquire(context.Background(), "o", "r")
	if err != nil {
		t.Fatal(err)
	}
	used := 0
	for _, f := range a.Fetched.Files {
		used += len(f.Content)
		if len(f.Content) > 32 || !f.Truncated {
			t.Fatalf("uncapped file: %+v", f)
		}
	}
	mu.Lock()
	n := calls
	mu.Unlock()
	if len(a.Tree.Entries) != 20 || !a.Tree.Truncated || len(a.Requested) != 2 || len(a.Fetched.Files) != 2 || used != 48 || n != 6 || !a.Outcome.Cacheable() {
		t.Fatalf("tree=%d truncated=%v requested=%d files=%d bytes=%d calls=%d outcome=%v", len(a.Tree.Entries), a.Tree.Truncated, len(a.Requested), len(a.Fetched.Files), used, n, a.Outcome)
	}
}

func TestConfiguredCallBudgetIncludesIdentityAndDoesNotCacheWithheldEvidence(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		switch {
		case r.URL.Path == "/repos/o/r":
			fmt.Fprint(w, `{"id":7,"owner":{"login":"o"},"name":"r","private":false,"default_branch":"main"}`)
		case strings.Contains(r.URL.Path, "/commits/"):
			fmt.Fprint(w, `{"sha":"0123456789abcdef0123456789abcdef01234567"}`)
		case strings.Contains(r.URL.Path, "/git/trees/"):
			fmt.Fprint(w, `{"tree":[{"path":"package.json","type":"blob"}]}`)
		default:
			fmt.Fprint(w, `{"type":"file","encoding":"base64","size":2,"content":"e30="}`)
		}
	}))
	defer srv.Close()
	s := NewPublicService(githubclient.WithBaseURL(srv.URL), githubclient.WithMaxRetries(0)).UseCache(openTestCache(t, t.TempDir(), testLimits(), &fakeClock{time.Now()}))
	l := DefaultLimits()
	l.MaxAPICalls = 4
	if err := s.UseLimits(l); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		p, err := s.Preview(context.Background(), "o", "r")
		if err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(p)
		if !strings.Contains(string(b), "identity") || strings.Contains(string(b), "/blob/") {
			t.Fatalf("unsafe preview: %s", b)
		}
	}
	if calls.Load() != 8 {
		t.Fatalf("calls=%d: budget renewed or withheld result cached", calls.Load())
	}
}

func TestLimitsRejectUnboundedOrExcessiveWork(t *testing.T) {
	for _, change := range []func(*Limits){
		func(l *Limits) { l.Timeout = 0 }, func(l *Limits) { l.MaxTreeEntries = -1 }, func(l *Limits) { l.MaxFiles = 1000000 },
		func(l *Limits) { l.MaxFileBytes = 1 << 30 }, func(l *Limits) { l.MaxTotalBytes = 1 << 30 }, func(l *Limits) { l.MaxAPICalls = 1 << 30 },
		func(l *Limits) { l.MaxResponseBytes = 1 << 30 }, func(l *Limits) { l.MaxDepth = 1 << 30 }, func(l *Limits) { l.Timeout = 24 * time.Hour },
	} {
		l := DefaultLimits()
		change(&l)
		if err := NewService(nil).UseLimits(l); err == nil {
			t.Fatalf("accepted %+v", l)
		}
	}
}
