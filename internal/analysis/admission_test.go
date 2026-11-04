package analysis

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

// Ported from the swarm review's cache/concurrency controls (SW5, SW6). All
// synchronization is by channels; nothing depends on timing.

type gatedHTTP struct {
	metadata, refs, trees            atomic.Int32
	treeStarted, treeRelease         chan struct{}
	metadataStarted, metadataRelease chan struct{}
	gateMetadata                     atomic.Bool
}

func gatedServer(t *testing.T) (*gatedHTTP, *Service) {
	t.Helper()
	h := &gatedHTTP{treeStarted: make(chan struct{}), treeRelease: make(chan struct{}), metadataStarted: make(chan struct{}, 128), metadataRelease: make(chan struct{})}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/commits/"):
			h.refs.Add(1)
			fmt.Fprint(w, `{"sha":"0123456789abcdef0123456789abcdef01234567"}`)
		case strings.Contains(r.URL.Path, "/git/trees/"):
			if h.trees.Add(1) == 1 {
				close(h.treeStarted)
				<-h.treeRelease
			}
			fmt.Fprint(w, `{"tree":[],"truncated":false}`)
		case r.URL.Path == "/repos/o/r":
			h.metadata.Add(1)
			if h.gateMetadata.Load() {
				h.metadataStarted <- struct{}{}
				<-h.metadataRelease
			}
			fmt.Fprint(w, `{"id":7,"owner":{"login":"o"},"name":"r","full_name":"o/r","private":false,"default_branch":"main"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	s := NewPublicService(githubclient.WithBaseURL(srv.URL), githubclient.WithMaxRetries(0))
	c, err := OpenPreviewCache(t.TempDir(), DefaultCacheLimits(), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	s.UseCache(c)
	return h, s
}

func startPreview(s *Service) <-chan error {
	done := make(chan error, 1)
	go func() { _, err := s.Preview(context.Background(), "o", "r"); done <- err }()
	return done
}

// SW5: callers beyond the admission bound are refused before any GitHub
// request, not after paying for a visibility lookup.
func TestAdmissionRefusesExcessCallersBeforeContactingGitHub(t *testing.T) {
	h, s := gatedServer(t)
	s.UseAdmission(AdmissionLimits{MaxActive: 1, MaxPerKey: 1, MaxWaiters: 1, Deadline: 30 * time.Second})
	first := startPreview(s)
	<-h.treeStarted
	before := h.metadata.Load()
	h.gateMetadata.Store(true)
	for i := 0; i < 80; i++ {
		if err := <-startPreview(s); err != ErrOverloaded {
			t.Fatalf("excess caller %d: err=%v, want ErrOverloaded", i, err)
		}
	}
	if extra := h.metadata.Load() - before; extra != 0 {
		t.Errorf("refused callers made %d upstream visibility requests", extra)
	}
	h.gateMetadata.Store(false)
	close(h.metadataRelease)
	close(h.treeRelease)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
}

// SW6: a caller that missed the cache and then leads a new flight after the
// previous leader stored its result is served that stored result.
func TestALeaderRechecksTheCacheBeforeAcquiring(t *testing.T) {
	h, s := gatedServer(t)
	first := startPreview(s)
	<-h.treeStarted
	missed, proceed := make(chan struct{}), make(chan struct{})
	s.afterCacheMiss = func() { close(missed); <-proceed }
	second := startPreview(s)
	<-missed
	close(h.treeRelease)
	if err := <-first; err != nil {
		t.Fatal(err) // the first caller has stored its result and retired its flight
	}
	close(proceed)
	if err := <-second; err != nil {
		t.Fatal(err)
	}
	if h.trees.Load() != 1 || h.refs.Load() != 1 {
		t.Errorf("the stored result was reacquired: refs %d, trees %d", h.refs.Load(), h.trees.Load())
	}
}
