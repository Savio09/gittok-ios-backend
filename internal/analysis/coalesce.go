package analysis

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

// AdmissionLimits bound concurrent preview work in one process. There is no
// queue: a request beyond these bounds fails immediately with ErrOverloaded.
type AdmissionLimits struct {
	MaxActive int // distinct acquisitions running at once
	MaxPerKey int // callers joined to one acquisition
	// MaxWaiters bounds the callers inside the service at once — checking
	// visibility or joined to an acquisition. A caller beyond it is refused
	// before any GitHub request is made.
	MaxWaiters int
	Deadline   time.Duration // shared deadline for one acquisition
}

// DefaultAdmissionLimits are the operating defaults.
func DefaultAdmissionLimits() AdmissionLimits {
	return AdmissionLimits{MaxActive: 2, MaxPerKey: 32, MaxWaiters: 64, Deadline: 30 * time.Second}
}

func (l AdmissionLimits) valid() bool {
	return l.MaxActive > 0 && l.MaxPerKey > 0 && l.MaxWaiters > 0 && l.Deadline > 0
}

// ErrOverloaded reports that admission bounds are exhausted; retry shortly.
var ErrOverloaded = errors.New("analysis: preview capacity is exhausted")

// UseAdmission coalesces identical preview misses and bounds concurrent work.
// Invalid limits fall back to the defaults.
func (s *Service) UseAdmission(l AdmissionLimits) *Service {
	if !l.valid() {
		l = DefaultAdmissionLimits()
	}
	s.flights = &group{limits: l, flights: map[CacheKey]*flight{}}
	return s
}

// flight is one generation of shared work for a key.
type flight struct {
	gen     uint64
	cancel  context.CancelFunc
	waiters int
	done    chan struct{}
	p       apimodel.RepoPreview
	err     error
}

// group coalesces callers by verified preview identity. Work runs under its
// own deadline, not any caller's: a departing caller only leaves, and when the
// last one leaves the work is cancelled and its generation retired before
// another caller can join it. A retired generation that completes late never
// removes or stores over newer work.
type group struct {
	limits   AdmissionLimits
	mu       sync.Mutex
	flights  map[CacheKey]*flight
	active   int // running acquisitions, including retired ones still finishing
	waiters  int
	requests int // admitted callers, from entry to exit
	gen      uint64

	// Test hooks.
	onJoin  func(CacheKey)
	onStore func(CacheKey, uint64)
	onDone  func(CacheKey, uint64)
}

// enter admits one caller before it does any GitHub work, or refuses it when
// the bound is reached. There is no queue. Each admitted caller must exit.
func (g *group) enter() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.requests >= g.limits.MaxWaiters {
		return false
	}
	g.requests++
	return true
}

func (g *group) exit() {
	g.mu.Lock()
	g.requests--
	g.mu.Unlock()
}

// work produces a preview and whether it may be stored.
type work func(context.Context) (apimodel.RepoPreview, bool, error)

func (g *group) do(ctx context.Context, k CacheKey, run work, store func(apimodel.RepoPreview)) (apimodel.RepoPreview, error) {
	g.mu.Lock()
	f := g.flights[k]
	switch {
	case f != nil && (f.waiters >= g.limits.MaxPerKey || g.waiters >= g.limits.MaxWaiters),
		f == nil && (g.active >= g.limits.MaxActive || g.waiters >= g.limits.MaxWaiters):
		g.mu.Unlock()
		return apimodel.RepoPreview{}, ErrOverloaded
	case f == nil:
		g.gen++
		shared, cancel := context.WithTimeout(context.Background(), g.limits.Deadline)
		f = &flight{gen: g.gen, cancel: cancel, done: make(chan struct{})}
		g.flights[k] = f
		g.active++
		go g.run(shared, k, f, run, store)
	}
	f.waiters++
	g.waiters++
	onJoin := g.onJoin
	g.mu.Unlock()
	if onJoin != nil {
		onJoin(k)
	}
	select {
	case <-f.done:
		g.leave(k, f, false)
		return f.p, f.err
	case <-ctx.Done():
		g.leave(k, f, true)
		return apimodel.RepoPreview{}, ctx.Err()
	}
}

func (g *group) run(ctx context.Context, k CacheKey, f *flight, run work, store func(apimodel.RepoPreview)) {
	p, storable, err := run(ctx)
	g.mu.Lock()
	if g.flights[k] == f { // still the current generation
		delete(g.flights, k)
		if err == nil && storable && store != nil && ctx.Err() == nil {
			store(p)
			if g.onStore != nil {
				g.onStore(k, f.gen)
			}
		}
	}
	f.p, f.err = p, err
	g.active--
	onDone := g.onDone
	g.mu.Unlock()
	f.cancel()
	close(f.done)
	if onDone != nil {
		onDone(k, f.gen)
	}
}

func (g *group) leave(k CacheKey, f *flight, abandoned bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	f.waiters--
	g.waiters--
	if abandoned && f.waiters == 0 {
		f.cancel()
		if g.flights[k] == f {
			delete(g.flights, k) // retire: the next caller starts a fresh generation
		}
	}
}
