package walkthrough

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/analysis"
	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
	"github.com/Savio09/gittok-ios-backend/internal/preview"
)

var (
	ErrJobsBusy       = errors.New("walkthrough capacity unavailable")
	ErrJobUnavailable = errors.New("walkthrough job unavailable")
)

const storyPolicy = "source-story-v1"

type Runner interface {
	Run(context.Context, string, *Investigation, func(string)) (Accepted, error)
}
type ServiceConfig struct {
	Limits      analysis.Limits
	JournalPath string
	MaxJobs     int
	TTL         time.Duration
}
type Job struct {
	ID          string                `json:"id"`
	State       string                `json:"state"`
	Stage       string                `json:"stage,omitempty"`
	PollAfterMS int                   `json:"poll_after_ms"`
	Preview     *apimodel.RepoPreview `json:"preview,omitempty"`
	Message     string                `json:"message,omitempty"`
}
type jobRecord struct {
	Job
	Identity        Identity  `json:"identity"`
	Created         time.Time `json:"created"`
	Completed       time.Time `json:"completed,omitempty"`
	key             string
	session         *Investigation
	reusable        bool
	visibilityEpoch uint64
}
type Service struct {
	mu      sync.Mutex
	github  PublicGitHub
	runner  Runner
	cfg     ServiceConfig
	journal *jobJournal
	records map[string]*jobRecord
	keys    map[string]string
	queue   chan string
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	pending int
	closed  bool
	broken  bool
}

func NewService(g PublicGitHub, runner Runner, cfg ServiceConfig) (*Service, error) {
	if g == nil || runner == nil || (g.Mode() != "anonymous" && g.Mode() != "public-token") {
		return nil, ErrJobUnavailable
	}
	if err := cfg.Limits.Validate(); err != nil {
		return nil, err
	}
	if cfg.MaxJobs < 1 || cfg.MaxJobs > 128 || cfg.TTL <= 0 || cfg.TTL > 24*time.Hour {
		return nil, ErrJobUnavailable
	}
	journal, records, err := openJobJournal(cfg.JournalPath, cfg.MaxJobs)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{github: g, runner: runner, cfg: cfg, journal: journal, records: records, keys: map[string]string{}, queue: make(chan string, cfg.MaxJobs), ctx: ctx, cancel: cancel, done: make(chan struct{})}
	// Bodies and target sessions are intentionally ephemeral. Restart never
	// replays work that may have charged the provider.
	for _, r := range records {
		r.Preview = nil
		r.Stage = ""
		r.PollAfterMS = 0
		if activeState(r.State) {
			r.State = "interrupted"
			r.Completed = time.Now().UTC()
			r.Message = "Generation was interrupted. Start a new walkthrough to try again."
		} else if r.State == "ready" {
			r.State = "unavailable"
			r.Message = "This walkthrough is no longer available. Start it again."
		}
	}
	if err := s.persistLocked(); err != nil {
		cancel()
		journal.close()
		return nil, err
	}
	go s.work()
	return s, nil
}
func activeState(state string) bool {
	return state == "queued" || state == "reading" || state == "writing" || state == "reviewing"
}
func jobKey(i Identity) string {
	return fmt.Sprintf("%s:%d:%s/%s@%s", storyPolicy, i.RepoID, i.Owner, i.Name, i.Commit)
}
func (s *Service) persistLocked() error {
	if err := s.journal.save(s.records); err != nil {
		s.broken = true
		s.cancel()
		return ErrJobUnavailable
	}
	return nil
}
func (s *Service) expireLocked() error {
	changed := false
	for id, r := range s.records {
		completed := r.Completed
		if completed.IsZero() {
			completed = r.Created
		}
		if !activeState(r.State) && time.Since(completed) > s.cfg.TTL {
			delete(s.records, id)
			changed = true
			if s.keys[r.key] == id {
				delete(s.keys, r.key)
			}
		}
	}
	if changed {
		return s.persistLocked()
	}
	return nil
}
func (s *Service) Start(caller context.Context, owner, name string) (Job, error) {
	if err := caller.Err(); err != nil {
		return Job{}, err
	}
	if !preview.ValidIdentity(owner, name) {
		return Job{}, ErrSourceUnavailable
	}
	s.mu.Lock()
	if s.closed || s.broken {
		s.mu.Unlock()
		return Job{}, ErrJobUnavailable
	}
	if err := s.expireLocked(); err != nil {
		s.mu.Unlock()
		return Job{}, err
	}
	// Canonical probes are separately bounded even when all job slots are in
	// use. A duplicate can still find its active receipt without a new worker.
	if s.pending >= s.cfg.MaxJobs {
		s.mu.Unlock()
		return Job{}, ErrJobsBusy
	}
	s.pending++
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.pending--; s.mu.Unlock() }()
	inv, err := beginInvestigation(s.ctx, s.github, owner, name, s.cfg.Limits, false)
	if err != nil {
		return Job{}, err
	}
	keep := false
	defer func() {
		if !keep {
			inv.Close()
		}
	}()
	job, created, err := s.admit(inv)
	keep = created
	if err == nil && job.State == "ready" {
		return s.Get(caller, job.ID)
	}
	return job, err
}

// admit holds the registry lock only for state changes. Cached source still
// passes Get's fresh visibility gate after this lock has been released.
func (s *Service) admit(inv *Investigation) (Job, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.broken {
		return Job{}, false, ErrJobUnavailable
	}
	if err := s.expireLocked(); err != nil {
		return Job{}, false, err
	}
	key := jobKey(inv.Snapshot().Identity)
	if id := s.keys[key]; id != "" {
		if r := s.records[id]; r != nil && (activeState(r.State) || (r.State == "ready" && r.reusable && time.Since(r.Completed) < s.cfg.TTL)) {
			if r.State == "ready" {
				return Job{ID: id, State: "ready"}, false, nil
			}
			return cloneJob(r.Job), false, nil
		}
	}
	// Only terminal records may be removed to make room. Removing a receipt
	// never removes independent spending reservations.
	if len(s.records) >= s.cfg.MaxJobs {
		var oldest *jobRecord
		for _, r := range s.records {
			if !activeState(r.State) && (oldest == nil || r.Created.Before(oldest.Created)) {
				oldest = r
			}
		}
		if oldest == nil {
			return Job{}, false, ErrJobsBusy
		}
		delete(s.records, oldest.ID)
		if s.keys[oldest.key] == oldest.ID {
			delete(s.keys, oldest.key)
		}
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return Job{}, false, ErrJobUnavailable
	}
	id := hex.EncodeToString(raw[:])
	r := &jobRecord{Job: Job{ID: id, State: "queued", PollAfterMS: 1000}, Identity: inv.Snapshot().Identity, Created: time.Now().UTC(), key: key, session: inv}
	s.records[id] = r
	s.keys[key] = id
	if err := s.persistLocked(); err != nil {
		delete(s.records, id)
		delete(s.keys, key)
		return Job{}, false, err
	}
	s.queue <- id
	return cloneJob(r.Job), true, nil
}
func (s *Service) Get(ctx context.Context, id string) (Job, error) {
	if err := ctx.Err(); err != nil {
		return Job{}, err
	}
	if len(id) != 32 {
		return Job{}, ErrJobUnavailable
	}
	if _, err := hex.DecodeString(id); err != nil {
		return Job{}, ErrJobUnavailable
	}
	s.mu.Lock()
	if s.closed || s.broken {
		s.mu.Unlock()
		return Job{}, ErrJobUnavailable
	}
	if err := s.expireLocked(); err != nil {
		s.mu.Unlock()
		return Job{}, err
	}
	r := s.records[id]
	if r == nil || s.closed || s.broken {
		s.mu.Unlock()
		return Job{}, ErrJobUnavailable
	}
	j := cloneJob(r.Job)
	identity := r.Identity
	epoch := r.visibilityEpoch
	s.mu.Unlock()
	if j.State != "ready" {
		return j, nil
	}
	// Current visibility is checked before every response containing source
	// prose. A job identifier is an opaque receipt, not an access credential.
	gate, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	gate = githubclient.NewRequestBudget(4, 1<<20).Context(gate)
	repo, err := s.github.GetRepo(gate, identity.Owner, identity.Name)
	// A disconnected poller does not change a completed job or its cache.
	if callerErr := ctx.Err(); callerErr != nil {
		return Job{}, callerErr
	}
	identityFailed := err == nil && (!repo.Public || repo.ID != identity.RepoID || repo.Owner != identity.Owner || repo.Name != identity.Name)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.broken {
		return Job{}, ErrJobUnavailable
	}
	if expireErr := s.expireLocked(); expireErr != nil {
		return Job{}, expireErr
	}
	current := s.records[id]
	if current != r {
		return Job{}, ErrJobUnavailable
	}
	if err != nil || gate.Err() != nil || identityFailed {
		// Every failed gate fences older public checks. A transport failure
		// withholds this response but retains accepted content for a fresh gate;
		// a concrete identity/visibility or credential failure clears it.
		current.visibilityEpoch++
		if identityFailed || errors.Is(err, githubclient.ErrCredentialPolicy) {
			current.State = "unavailable"
			current.Preview = nil
			current.PollAfterMS = 0
			current.Message = visibilityMessage
			current.reusable = false
			if persistErr := s.persistLocked(); persistErr != nil {
				return Job{}, persistErr
			}
		}
		return hiddenJob(id), nil
	}
	if current.visibilityEpoch != epoch || current.State != "ready" || current.Preview == nil {
		return hiddenJob(id), nil
	}
	return cloneJob(current.Job), nil
}

const visibilityMessage = "The repository could not be verified as publicly available."

func hiddenJob(id string) Job { return Job{ID: id, State: "unavailable", Message: visibilityMessage} }
func cloneJob(j Job) Job {
	if j.Preview != nil {
		data, _ := json.Marshal(j.Preview)
		var p apimodel.RepoPreview
		_ = json.Unmarshal(data, &p)
		j.Preview = &p
	}
	return j
}
func (s *Service) work() {
	defer close(s.done)
	for {
		select {
		case <-s.ctx.Done():
			return
		case id := <-s.queue:
			s.execute(id)
		}
	}
}
func (s *Service) execute(id string) {
	s.mu.Lock()
	r := s.records[id]
	if r == nil || s.closed || s.broken || r.State != "queued" {
		s.mu.Unlock()
		return
	}
	inv := r.session
	r.State = "reading"
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		inv.Close()
		return
	}
	s.mu.Unlock()
	defer inv.Close()
	stage := func(state string) {
		if state != "reading" && state != "writing" && state != "reviewing" {
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.records[id] != r || s.closed || s.broken || !activeState(r.State) {
			return
		}
		r.State = state
		r.Stage = state
		_ = s.persistLocked()
	}
	err := inv.Seed()
	var accepted Accepted
	if err == nil {
		accepted, err = s.runner.Run(inv.Context(), id, inv, stage)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Pointer ownership and active state fence late completion after shutdown,
	// eviction or an interrupted generation. No worker can overwrite its successor.
	if s.records[id] != r || s.closed || s.broken || !activeState(r.State) {
		return
	}
	r.session = nil
	lastStage := r.State
	r.Stage = ""
	r.PollAfterMS = 0
	r.Completed = time.Now().UTC()
	if err != nil || accepted.ID() == "" || accepted.Identity() != r.Identity || inv.Outcome() == analysis.OutcomeIdentityWithheld || inv.Context().Err() != nil {
		r.State = "failed"
		r.Message = "A source-grounded walkthrough could not be completed. The source preview is still available."
		r.Preview = nil
	} else {
		p := storyPreview(inv.Repo(), accepted, inv.Snapshot().CapturedAt)
		r.Preview = &p
		r.State = "ready"
		r.reusable = inv.Outcome().Cacheable() && inv.Snapshot().Complete
	}
	code := "accepted"
	if r.State != "ready" {
		code = "generation_failed"
		var status *ProviderStatusError
		if errors.As(err, &status) {
			code = fmt.Sprintf("provider_http_%d", status.Status)
		} else {
			switch {
			case errors.Is(err, context.DeadlineExceeded):
				code = "deadline"
			case errors.Is(err, context.Canceled):
				code = "cancelled"
			case errors.Is(err, ErrStoryRejected):
				code = "story_rejected"
			case errors.Is(err, ErrGenerationBudget):
				code = "generation_limit"
			case errors.Is(err, ErrIdentityWithheld):
				code = "identity_withheld"
			}
		}
	}
	log.Printf("gittok: walkthrough job=%s state=%s stage=%s code=%s duration_ms=%d", id, r.State, lastStage, code, time.Since(r.Created).Milliseconds())
	_ = s.persistLocked()
}
func storyPreview(repo githubclient.Repo, a Accepted, captured time.Time) apimodel.RepoPreview {
	sections := a.Sections()
	description := ""
	for _, section := range sections {
		if section.Kind == apimodel.SectionIdea {
			description = section.Body
			break
		}
	}
	if description == "" && len(sections) > 0 {
		description = sections[0].Body
	}
	// Reuse the existing provider-value checks for card metadata. Stable GitHub
	// numeric identity belongs to job ownership; the public card keeps its
	// existing owner/name ID across source and story presentations.
	p := preview.Build(preview.Input{Repo: repo, CapturedAt: captured})
	p.Presentation = "walkthrough"
	p.Description = description
	p.Sections = sections
	return p
}
func (s *Service) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.cancel()
	for _, r := range s.records {
		if activeState(r.State) {
			r.State = "interrupted"
			r.Stage = ""
			r.Preview = nil
			r.PollAfterMS = 0
			r.Completed = time.Now().UTC()
			if r.session != nil {
				r.session.Close()
			}
		}
	}
	err := s.persistLocked()
	s.mu.Unlock()
	<-s.done
	closeErr := s.journal.close()
	if err != nil {
		return err
	}
	return closeErr
}

// Handler supports the POST repository route and GET job route. Nil services
// explicitly report unconfigured generation without changing source previews.
func (s *Service) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		fail := func(status int, message string) {
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
		}
		if s == nil {
			fail(503, "Walkthrough generation is not configured.")
			return
		}
		var job Job
		var err error
		if r.Method == http.MethodPost {
			if !preview.ValidIdentity(r.PathValue("owner"), r.PathValue("name")) {
				fail(400, "Invalid repository identity.")
				return
			}
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, 1024)
				var body map[string]json.RawMessage
				decoder := json.NewDecoder(r.Body)
				decodeErr := decoder.Decode(&body)
				if decodeErr != nil && !errors.Is(decodeErr, io.EOF) {
					fail(400, "Expected an empty JSON object.")
					return
				}
				if decodeErr == nil && body == nil {
					fail(400, "Expected an empty JSON object.")
					return
				}
				if len(body) != 0 {
					fail(400, "No generation options are accepted.")
					return
				}
				if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
					fail(400, "Expected one empty JSON object.")
					return
				}
			}
			job, err = s.Start(r.Context(), r.PathValue("owner"), r.PathValue("name"))
		} else {
			job, err = s.Get(r.Context(), r.PathValue("id"))
		}
		if r.Context().Err() != nil {
			return
		}
		if err != nil {
			switch {
			case errors.Is(err, ErrJobsBusy):
				w.Header().Set("Retry-After", "2")
				fail(503, "Walkthrough capacity is busy.")
			case errors.Is(err, ErrJobUnavailable), errors.Is(err, githubclient.ErrNotFound), errors.Is(err, ErrSourceUnavailable):
				fail(404, "Walkthrough unavailable.")
			default:
				fail(503, "Walkthrough generation is temporarily unavailable.")
			}
			return
		}
		if r.Method == http.MethodPost && job.State != "ready" {
			w.WriteHeader(http.StatusAccepted)
		}
		_ = json.NewEncoder(w).Encode(job)
	})
}
