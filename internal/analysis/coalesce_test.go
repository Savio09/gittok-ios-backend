package analysis

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

// gatedGitHub blocks every tree read until release is closed (or the read's
// context ends), so tests control exactly when acquisition finishes.
type gatedGitHub struct {
	release   chan struct{}
	started   chan string   // one send per tree read, naming the repository
	cancelled chan struct{} // one send per tree read ended by its context
	trees     atomic.Int32
}

func newGated() *gatedGitHub {
	return &gatedGitHub{release: make(chan struct{}), started: make(chan string, 64), cancelled: make(chan struct{}, 64)}
}

func (g *gatedGitHub) GetRepo(_ context.Context, _, name string) (githubclient.Repo, error) {
	id, _ := strconv.Atoi(name[1:]) // names are r1, r2, ...
	return githubclient.Repo{ID: int64(id), Owner: "o", Name: name, FullName: "o/" + name, Public: true, DefaultBranch: "main"}, nil
}
func (g *gatedGitHub) ResolveRef(context.Context, string, string, string) (string, error) {
	return "0123456789abcdef0123456789abcdef01234567", nil
}
func (g *gatedGitHub) GetTree(ctx context.Context, _, name, _ string, _ githubclient.TreeLimits) (githubclient.FileTree, error) {
	g.trees.Add(1)
	g.started <- name
	select {
	case <-g.release:
		return githubclient.FileTree{Entries: []githubclient.TreeEntry{{Path: "package.json", Type: "blob"}}}, nil
	case <-ctx.Done():
		g.cancelled <- struct{}{}
		return githubclient.FileTree{}, ctx.Err()
	}
}
func (g *gatedGitHub) GetFileContents(context.Context, string, string, string, []string, githubclient.ContentLimits) (githubclient.FetchedContents, error) {
	return githubclient.FetchedContents{Files: []githubclient.FileContent{{Path: "package.json", Content: `{"name":"x","exports":"./i.js"}`}}}, nil
}

type harness struct {
	s      *Service
	gh     *gatedGitHub
	joined chan CacheKey
}

func newHarness(t *testing.T, l AdmissionLimits) *harness {
	t.Helper()
	gh := newGated()
	s := NewService(gh)
	s.publicOnly = true
	s.UseCache(openTestCache(t, t.TempDir(), testLimits(), &fakeClock{time.Now()}))
	s.UseAdmission(l)
	h := &harness{s: s, gh: gh, joined: make(chan CacheKey, 64)}
	s.flights.onJoin = func(k CacheKey) { h.joined <- k }
	return h
}

type outcome struct {
	p   apimodel.RepoPreview
	err error
}

func (h *harness) start(ctx context.Context, name string) chan outcome {
	out := make(chan outcome, 1)
	go func() {
		p, err := h.s.Preview(ctx, "o", name)
		out <- outcome{p, err}
	}()
	return out
}

func wideLimits() AdmissionLimits {
	return AdmissionLimits{MaxActive: 2, MaxPerKey: 32, MaxWaiters: 64, Deadline: 30 * time.Second}
}

func TestSimultaneousIdenticalMissesShareOneAcquisition(t *testing.T) {
	h := newHarness(t, wideLimits())
	var outs []chan outcome
	for i := 0; i < 5; i++ {
		outs = append(outs, h.start(context.Background(), "r1"))
	}
	for i := 0; i < 5; i++ {
		<-h.joined
	}
	<-h.gh.started
	close(h.gh.release)
	var first apimodel.RepoPreview
	for i, o := range outs {
		got := <-o
		if got.err != nil {
			t.Fatal(got.err)
		}
		if i == 0 {
			first = got.p
		} else if got.p.ID != first.ID || len(got.p.Sections) != len(first.Sections) {
			t.Fatal("joined callers received different results")
		}
	}
	if n := h.gh.trees.Load(); n != 1 {
		t.Fatalf("%d acquisitions for one repository", n)
	}
}

func TestAFollowerSurvivesItsLeadersCancellation(t *testing.T) {
	h := newHarness(t, wideLimits())
	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	leader := h.start(leaderCtx, "r1")
	<-h.joined
	<-h.gh.started
	follower := h.start(context.Background(), "r1")
	<-h.joined
	cancelLeader()
	if got := <-leader; !errors.Is(got.err, context.Canceled) {
		t.Fatalf("leader: %v", got.err)
	}
	close(h.gh.release)
	if got := <-follower; got.err != nil || got.p.ID != "o/r1" {
		t.Fatalf("follower lost its result when the leader left: %+v %v", got.p, got.err)
	}
	select {
	case <-h.gh.cancelled:
		t.Fatal("work was cancelled while a caller still waited")
	default:
	}
}

func TestTheLastCallerLeavingCancelsWorkAndRetiresItsGeneration(t *testing.T) {
	h := newHarness(t, wideLimits())
	ctx, cancel := context.WithCancel(context.Background())
	only := h.start(ctx, "r1")
	<-h.joined
	<-h.gh.started
	cancel()
	if got := <-only; !errors.Is(got.err, context.Canceled) {
		t.Fatalf("err = %v", got.err)
	}
	<-h.gh.cancelled // the abandoned acquisition observed cancellation
	next := h.start(context.Background(), "r1")
	<-h.joined
	<-h.gh.started // a fresh generation began its own acquisition
	close(h.gh.release)
	if got := <-next; got.err != nil {
		t.Fatal(got.err)
	}
	if n := h.gh.trees.Load(); n != 2 {
		t.Fatalf("acquisitions = %d, want a fresh second generation", n)
	}
}

// A detached generation that finishes late must neither remove the newer
// generation nor store its own result over it.
func TestALateOldGenerationCannotDisplaceNewerWork(t *testing.T) {
	gh := newGated()
	s := NewService(gh)
	s.publicOnly = true
	s.UseCache(openTestCache(t, t.TempDir(), testLimits(), &fakeClock{time.Now()}))
	s.UseAdmission(wideLimits())
	joined := make(chan CacheKey, 8)
	finished := make(chan uint64, 8)
	var storedGens sync.Map
	s.flights.onJoin = func(k CacheKey) { joined <- k }
	s.flights.onStore = func(_ CacheKey, gen uint64) { storedGens.Store(gen, true) }
	s.flights.onDone = func(_ CacheKey, gen uint64) { finished <- gen }
	// Generation 1 ignores cancellation until released, so it completes late.
	stubborn := &stubbornGitHub{gatedGitHub: gh, hold: make(chan struct{})}
	s.github = stubborn
	ctx, cancel := context.WithCancel(context.Background())
	old := make(chan error, 1)
	go func() { _, err := s.Preview(ctx, "o", "r1"); old <- err }()
	<-joined
	<-gh.started
	cancel()
	<-old
	newer := make(chan error, 1)
	go func() { _, err := s.Preview(context.Background(), "o", "r1"); newer <- err }()
	<-joined
	<-gh.started
	close(stubborn.hold) // generation 1 completes now, after being retired
	if gen := <-finished; gen != 1 {
		t.Fatalf("generation %d finished first, want the retired generation 1", gen)
	}
	third := make(chan error, 1)
	go func() { _, err := s.Preview(context.Background(), "o", "r1"); third <- err }()
	<-joined
	close(gh.release)
	if err := <-newer; err != nil {
		t.Fatal(err)
	}
	if err := <-third; err != nil {
		t.Fatal(err)
	}
	if n := gh.trees.Load(); n != 2 {
		t.Fatalf("acquisitions = %d: the old generation displaced the newer one", n)
	}
	if _, ok := storedGens.Load(uint64(1)); ok {
		t.Fatal("the retired generation stored its result")
	}
	if _, ok := storedGens.Load(uint64(2)); !ok {
		t.Fatal("the current generation's complete result was not stored")
	}
}

// stubbornGitHub lets the first tree read finish only when hold is closed,
// ignoring cancellation, to simulate an acquisition that completes late.
type stubbornGitHub struct {
	*gatedGitHub
	hold  chan struct{}
	first atomic.Bool
}

func (s *stubbornGitHub) GetTree(ctx context.Context, o, n, ref string, l githubclient.TreeLimits) (githubclient.FileTree, error) {
	if s.first.CompareAndSwap(false, true) {
		s.trees.Add(1)
		s.started <- n
		<-s.hold
		return githubclient.FileTree{Entries: []githubclient.TreeEntry{{Path: "package.json", Type: "blob"}}}, nil
	}
	return s.gatedGitHub.GetTree(ctx, o, n, ref, l)
}

func TestAdmissionIsBoundedWithoutAQueue(t *testing.T) {
	l := AdmissionLimits{MaxActive: 2, MaxPerKey: 2, MaxWaiters: 3, Deadline: 30 * time.Second}
	h := newHarness(t, l)
	a := h.start(context.Background(), "r1")
	b := h.start(context.Background(), "r2")
	<-h.joined
	<-h.joined
	if _, err := h.s.Preview(context.Background(), "o", "r3"); !errors.Is(err, ErrOverloaded) {
		t.Fatalf("a third distinct acquisition was admitted: %v", err)
	}
	a2 := h.start(context.Background(), "r1")
	<-h.joined
	if _, err := h.s.Preview(context.Background(), "o", "r1"); !errors.Is(err, ErrOverloaded) {
		t.Fatalf("waiter bounds not enforced: %v", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/repos/o%2Fr4/preview", nil)
	req.SetPathValue("id", "o/r4")
	h.s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("overload response %d, Retry-After %q", rec.Code, rec.Header().Get("Retry-After"))
	}
	close(h.gh.release)
	for _, o := range []chan outcome{a, b, a2} {
		if got := <-o; got.err != nil {
			t.Fatal(got.err)
		}
	}
	// Capacity is released when work finishes.
	if _, err := h.s.Preview(context.Background(), "o", "r3"); err != nil {
		t.Fatalf("capacity not released: %v", err)
	}
}

func TestSharedWorkHasItsOwnDeadline(t *testing.T) {
	l := wideLimits()
	l.Deadline = time.Millisecond
	h := newHarness(t, l)
	got := <-h.start(context.Background(), "r1")
	<-h.gh.cancelled
	if got.err == nil {
		t.Fatal("work outlived the shared deadline")
	}
}
