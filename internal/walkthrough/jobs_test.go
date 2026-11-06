package walkthrough

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/analysis"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

type jobRunner struct {
	mu      sync.Mutex
	calls   int
	entered chan struct{}
	release chan struct{}
}

type distinctRepoGitHub struct{ *investigationFake }

func (g distinctRepoGitHub) GetRepo(ctx context.Context, owner, name string) (githubclient.Repo, error) {
	r, err := g.investigationFake.GetRepo(ctx, owner, name)
	if owner != "owner" {
		r.Owner = owner
		r.ID++
	}
	return r, err
}

func (r *jobRunner) Run(ctx context.Context, id string, inv *Investigation, stage func(string)) (Accepted, error) {
	r.mu.Lock()
	r.calls++
	r.mu.Unlock()
	if r.entered != nil {
		select {
		case r.entered <- struct{}{}:
		default:
		}
	}
	if r.release != nil {
		select {
		case <-r.release:
		case <-ctx.Done():
			return Accepted{}, ctx.Err()
		}
	}
	stage("writing")
	f, err := inv.Read("main.go")
	if err != nil {
		return Accepted{}, err
	}
	c := Citation{FileID: f.ID, StartLine: 1, EndLine: 1, Quote: "package main"}
	d := Draft{Scenes: []Scene{{ID: "s", Role: "workflow", StatementIDs: []string{"p", "a", "m", "r"}}}}
	review := Review{Accepted: true, Reason: "Scripted contract fixture only."}
	for i, k := range []string{"purpose", "action", "mechanism", "result"} {
		id := []string{"p", "a", "m", "r"}[i]
		d.Statements = append(d.Statements, Statement{ID: id, Kind: k, Text: "Fixture " + k, Sources: []Citation{c}})
		review.Decisions = append(review.Decisions, Decision{StatementID: id, Accepted: true, Reason: "Fixture."})
	}
	if err := inv.Verify(); err != nil {
		return Accepted{}, err
	}
	return Accept(inv.Snapshot(), d, review)
}
func jobConfig(t *testing.T) ServiceConfig {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return ServiceConfig{Limits: analysis.DefaultLimits(), JournalPath: filepath.Join(dir, "jobs.json"), MaxJobs: 8, TTL: time.Minute}
}
func waitJob(t *testing.T, s *Service, id string) Job {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		j, err := s.Get(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if j.State == "ready" || j.State == "failed" || j.State == "interrupted" {
			return j
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("job did not finish")
	return Job{}
}

func TestJobsCoalesceAndReuseOnlySameVerifiedCommit(t *testing.T) {
	g := investigationFixture()
	r := &jobRunner{entered: make(chan struct{}, 1), release: make(chan struct{})}
	s, err := NewService(g, r, jobConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a, err := s.Start(context.Background(), "owner", "project")
	if err != nil {
		t.Fatal(err)
	}
	<-r.entered
	b, err := s.Start(context.Background(), "owner", "project")
	if err != nil || a.ID != b.ID {
		t.Fatalf("duplicate paid job: %+v %v", b, err)
	}
	close(r.release)
	ready := waitJob(t, s, a.ID)
	if ready.State != "ready" || ready.Preview == nil || ready.Preview.Presentation != "walkthrough" {
		t.Fatalf("not ready: %+v", ready)
	}
	again, err := s.Start(context.Background(), "owner", "project")
	if err != nil || again.ID != a.ID {
		t.Fatal("completed commit not reused", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.calls != 1 {
		t.Fatalf("calls=%d", r.calls)
	}
}
func TestJobsRestartInterruptsWithoutReplayingAndFencesOldWorker(t *testing.T) {
	g := investigationFixture()
	r := &jobRunner{entered: make(chan struct{}, 1), release: make(chan struct{})}
	cfg := jobConfig(t)
	s, err := NewService(g, r, cfg)
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.Start(context.Background(), "owner", "project")
	if err != nil {
		t.Fatal(err)
	}
	<-r.entered
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	replacement := &jobRunner{}
	next, err := NewService(g, replacement, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	j, err := next.Get(context.Background(), a.ID)
	if err != nil || j.State != "interrupted" || j.Preview != nil {
		t.Fatalf("restart replayed/lost state: %+v %v", j, err)
	}
	replacement.mu.Lock()
	defer replacement.mu.Unlock()
	if replacement.calls != 0 {
		t.Fatal("paid work replayed")
	}
}
func TestJobsWithholdPreviouslyReadySourceWhenVisibilityChanges(t *testing.T) {
	g := investigationFixture()
	s, err := NewService(g, &jobRunner{}, jobConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a, err := s.Start(context.Background(), "owner", "project")
	if err != nil {
		t.Fatal(err)
	}
	if j := waitJob(t, s, a.ID); j.State != "ready" {
		t.Fatalf("%+v", j)
	}
	g.repo.Public = false
	j, err := s.Get(context.Background(), a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if j.State != "unavailable" || j.Preview != nil {
		t.Fatal("private source exposed")
	}
	if _, err := s.Start(context.Background(), "owner", "project"); err == nil {
		t.Fatal("private repository generated")
	}
}
func TestJobsRejectUnavailableJournalAndBoundAdmission(t *testing.T) {
	g := investigationFixture()
	r := &jobRunner{entered: make(chan struct{}, 1), release: make(chan struct{})}
	cfg := jobConfig(t)
	cfg.MaxJobs = 1
	s, err := NewService(distinctRepoGitHub{g}, r, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Start(context.Background(), "owner", "project"); err != nil {
		t.Fatal(err)
	}
	<-r.entered
	if _, err := s.Start(context.Background(), "another", "project"); !errors.Is(err, ErrJobsBusy) {
		t.Fatalf("unbounded admission: %v", err)
	}
	if _, err := s.Get(context.Background(), "missing"); !errors.Is(err, ErrJobUnavailable) {
		t.Fatal(err)
	}
}

type visibilityGitHub struct {
	*investigationFake
	mu          sync.Mutex
	deny        bool
	flipResolve bool
	repoErr     error
	entered     chan struct{}
	release     chan struct{}
}

func (g *visibilityGitHub) GetRepo(ctx context.Context, owner, name string) (githubclient.Repo, error) {
	g.mu.Lock()
	r := g.investigationFake.repo
	r.Public = !g.deny
	repoErr := g.repoErr
	entered, release := g.entered, g.release
	g.entered = nil
	g.release = nil
	g.mu.Unlock()
	if entered != nil {
		close(entered)
		select {
		case <-release:
		case <-ctx.Done():
			return githubclient.Repo{}, ctx.Err()
		}
	}
	return r, repoErr
}
func (g *visibilityGitHub) ResolveRef(context.Context, string, string, string) (string, error) {
	g.mu.Lock()
	if g.flipResolve {
		g.deny = true
	}
	g.mu.Unlock()
	return strings.Repeat("a", 40), nil
}
func TestJobsStartCacheHitMustRevalidateVisibility(t *testing.T) {
	g := &visibilityGitHub{investigationFake: investigationFixture()}
	s, err := NewService(g, &jobRunner{}, jobConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	first, err := s.Start(context.Background(), "owner", "project")
	if err != nil {
		t.Fatal(err)
	}
	if waitJob(t, s, first.ID).State != "ready" {
		t.Fatal("fixture not ready")
	}
	g.mu.Lock()
	g.flipResolve = true
	g.mu.Unlock()
	result, err := s.Start(context.Background(), "owner", "project")
	if err == nil && result.Preview != nil {
		t.Fatal("POST returned cached source after visibility changed during commit resolution")
	}
}
func TestJobsFailedVisibilityFencesConcurrentReadyResponse(t *testing.T) {
	g := &visibilityGitHub{investigationFake: investigationFixture()}
	s, err := NewService(g, &jobRunner{}, jobConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	first, err := s.Start(context.Background(), "owner", "project")
	if err != nil {
		t.Fatal(err)
	}
	if waitJob(t, s, first.ID).State != "ready" {
		t.Fatal("fixture not ready")
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	g.mu.Lock()
	g.entered = entered
	g.release = release
	g.mu.Unlock()
	result := make(chan Job, 1)
	go func() { j, _ := s.Get(context.Background(), first.ID); result <- j }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first gate did not start")
	}
	g.mu.Lock()
	g.deny = true
	g.mu.Unlock()
	denied, err := s.Get(context.Background(), first.ID)
	if err != nil || denied.Preview != nil || denied.State != "unavailable" {
		t.Fatalf("failed control: %+v %v", denied, err)
	}
	unblock()
	j := <-result
	if j.Preview != nil {
		t.Fatal("older public check published a cloned source body after job was withheld")
	}
}
func TestJobsGetEnforcesTerminalTTL(t *testing.T) {
	g := investigationFixture()
	cfg := jobConfig(t)
	s, err := NewService(g, &jobRunner{}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	first, err := s.Start(context.Background(), "owner", "project")
	if err != nil {
		t.Fatal(err)
	}
	if waitJob(t, s, first.ID).State != "ready" {
		t.Fatal("fixture not ready")
	}
	s.mu.Lock()
	s.records[first.ID].Completed = time.Now().Add(-2 * cfg.TTL)
	s.mu.Unlock()
	result, err := s.Get(context.Background(), first.ID)
	if err == nil && result.Preview != nil {
		t.Fatal("expired job remains a permanent source endpoint until unrelated POST")
	}
}
func TestJobsDuplicateCoalescesAtFullCapacity(t *testing.T) {
	g := investigationFixture()
	cfg := jobConfig(t)
	cfg.MaxJobs = 1
	r := &jobRunner{entered: make(chan struct{}, 1), release: make(chan struct{})}
	s, err := NewService(g, r, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	first, err := s.Start(context.Background(), "owner", "project")
	if err != nil {
		t.Fatal(err)
	}
	<-r.entered
	result, err := s.Start(context.Background(), "owner", "project")
	if err != nil || result.ID != first.ID {
		t.Fatalf("duplicate active intent rejected instead of coalesced: %+v %v", result, err)
	}
}
func TestJobsCanceledPollDoesNotDestroyCompletedJob(t *testing.T) {
	g := investigationFixture()
	s, err := NewService(g, &jobRunner{}, jobConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	first, err := s.Start(context.Background(), "owner", "project")
	if err != nil {
		t.Fatal(err)
	}
	if waitJob(t, s, first.ID).State != "ready" {
		t.Fatal("fixture not ready")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _ = s.Get(ctx, first.ID)
	result, err := s.Get(context.Background(), first.ID)
	if err != nil || result.State != "ready" || result.Preview == nil {
		t.Fatalf("disconnected poller destroyed paid completed job: %+v %v", result, err)
	}
}

func TestJobsTransientGateFailureReusesAcceptedBodyAfterFreshGate(t *testing.T) {
	g := &visibilityGitHub{investigationFake: investigationFixture()}
	runner := &jobRunner{}
	s, err := NewService(g, runner, jobConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	first, err := s.Start(context.Background(), "owner", "project")
	if err != nil {
		t.Fatal(err)
	}
	if waitJob(t, s, first.ID).State != "ready" {
		t.Fatal("fixture not ready")
	}
	g.mu.Lock()
	g.repoErr = errors.New("temporary provider failure")
	g.mu.Unlock()
	failed, err := s.Get(context.Background(), first.ID)
	if err != nil || failed.Preview != nil {
		t.Fatalf("failed gate published source: %+v %v", failed, err)
	}
	g.mu.Lock()
	g.repoErr = nil
	g.mu.Unlock()
	retry, err := s.Start(context.Background(), "owner", "project")
	if err != nil || retry.ID != first.ID || retry.Preview == nil {
		t.Fatalf("fresh successful gate did not reuse accepted body: %+v %v", retry, err)
	}
	runner.mu.Lock()
	defer runner.mu.Unlock()
	if runner.calls != 1 {
		t.Fatal("transient metadata error regenerated paid content")
	}
}

func TestJobsGetExpiryPersistsReceiptRemoval(t *testing.T) {
	g := investigationFixture()
	cfg := jobConfig(t)
	s, err := NewService(g, &jobRunner{}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	first, err := s.Start(context.Background(), "owner", "project")
	if err != nil {
		t.Fatal(err)
	}
	if waitJob(t, s, first.ID).State != "ready" {
		t.Fatal("fixture not ready")
	}
	s.mu.Lock()
	s.records[first.ID].Completed = time.Now().Add(-2 * cfg.TTL)
	s.mu.Unlock()
	if _, err := s.Get(context.Background(), first.ID); !errors.Is(err, ErrJobUnavailable) {
		t.Fatalf("expired receipt remained available: %v", err)
	}
	data, err := os.ReadFile(cfg.JournalPath)
	if err != nil {
		t.Fatal(err)
	}
	var journal journalSnapshot
	if err := json.Unmarshal(data, &journal); err != nil {
		t.Fatal(err)
	}
	if len(journal.Records) != 0 {
		t.Fatal("expired receipt removal was not durable")
	}
}

func TestJobsClosedServiceCannotRewriteJournalDuringExpiry(t *testing.T) {
	g := investigationFixture()
	cfg := jobConfig(t)
	s, err := NewService(g, &jobRunner{}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.Start(context.Background(), "owner", "project")
	if err != nil {
		t.Fatal(err)
	}
	if waitJob(t, s, first.ID).State != "ready" {
		t.Fatal("fixture not ready")
	}
	s.mu.Lock()
	s.records[first.ID].Completed = time.Now().Add(-2 * cfg.TTL)
	s.mu.Unlock()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(cfg.JournalPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Start(context.Background(), "owner", "project"); !errors.Is(err, ErrJobUnavailable) {
		t.Fatal(err)
	}
	after, err := os.ReadFile(cfg.JournalPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("closed service wrote journal after releasing exclusive process lock")
	}
}

func TestJobsJournalRejectsCorruptOrIndirectControlState(t *testing.T) {
	for _, kind := range []string{"malformed", "trailing", "oversized", "nonhex ID", "invalid identity", "duplicate ID", "public permissions", "symlink", "lock symlink", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			cfg := jobConfig(t)
			record := journalRecord{ID: strings.Repeat("a", 32), State: "queued", Created: time.Now().UTC(), Identity: Identity{RepoID: 12, Owner: "owner", Name: "project", Commit: strings.Repeat("a", 40)}}
			snapshot := journalSnapshot{Version: 1, Records: []journalRecord{record}}
			switch kind {
			case "nonhex ID":
				snapshot.Records[0].ID = strings.Repeat("z", 32)
			case "invalid identity":
				snapshot.Records[0].Identity.Commit = "main"
			case "duplicate ID":
				snapshot.Records = append(snapshot.Records, record)
			}
			data, err := json.Marshal(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "malformed":
				data = []byte("{")
			case "trailing":
				data = append(data, []byte("{}")...)
			case "oversized":
				data = append(data, bytes.Repeat([]byte(" "), 1<<20)...)
			}
			if err := os.WriteFile(cfg.JournalPath, data, 0600); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "public permissions":
				if err := os.Chmod(cfg.JournalPath, 0644); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				target := cfg.JournalPath + ".target"
				if err := os.Rename(cfg.JournalPath, target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, cfg.JournalPath); err != nil {
					t.Fatal(err)
				}
			case "lock symlink":
				if err := os.Symlink(cfg.JournalPath, cfg.JournalPath+".lock"); err != nil {
					t.Fatal(err)
				}
			case "hardlink":
				if err := os.Link(cfg.JournalPath, cfg.JournalPath+".other"); err != nil {
					t.Fatal(err)
				}
			}
			s, err := NewService(investigationFixture(), &jobRunner{}, cfg)
			if err == nil {
				s.Close()
				t.Fatal("corrupt/indirect journal accepted")
			}
		})
	}
}

func TestJobsRecoveredActiveReceiptHasBoundedLifetimeAndNeverReplays(t *testing.T) {
	cfg := jobConfig(t)
	id := strings.Repeat("a", 32)
	snapshot := journalSnapshot{Version: 1, Records: []journalRecord{{ID: id, State: "writing", Created: time.Now().Add(-time.Hour), Identity: Identity{RepoID: 12, Owner: "owner", Name: "project", Commit: strings.Repeat("a", 40)}}}}
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.JournalPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	runner := &jobRunner{}
	s, err := NewService(investigationFixture(), runner, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	j, err := s.Get(context.Background(), id)
	if err != nil || j.State != "interrupted" || j.Preview != nil {
		t.Fatalf("incorrect recovery: %+v %v", j, err)
	}
	s.mu.Lock()
	completed := s.records[id].Completed
	s.records[id].Completed = time.Now().Add(-2 * cfg.TTL)
	s.mu.Unlock()
	if completed.IsZero() {
		t.Fatal("recovered receipt cannot expire")
	}
	if _, err := s.Get(context.Background(), id); !errors.Is(err, ErrJobUnavailable) {
		t.Fatal("recovered receipt outlived TTL")
	}
	runner.mu.Lock()
	defer runner.mu.Unlock()
	if runner.calls != 0 {
		t.Fatal("recovery replayed paid work")
	}
}

func TestJobsJournalIsExclusiveAndMissingReceiptsAreExplicitlyUnavailable(t *testing.T) {
	cfg := jobConfig(t)
	g := investigationFixture()
	s, err := NewService(g, &jobRunner{}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if next, err := NewService(g, &jobRunner{}, cfg); err == nil {
		next.Close()
		s.Close()
		t.Fatal("two services shared a journal")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(cfg.JournalPath); err != nil {
		t.Fatal(err)
	}
	runner := &jobRunner{}
	next, err := NewService(g, runner, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	if _, err := next.Get(context.Background(), strings.Repeat("a", 32)); !errors.Is(err, ErrJobUnavailable) {
		t.Fatal("missing receipt was replayed or invented")
	}
	runner.mu.Lock()
	defer runner.mu.Unlock()
	if runner.calls != 0 {
		t.Fatal("missing journal replayed work")
	}
}
