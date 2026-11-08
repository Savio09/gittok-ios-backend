package walkthrough

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/Savio09/gittok-ios-backend/internal/analysis"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

type investigationFake struct {
	mu        sync.Mutex
	repo      githubclient.Repo
	entries   []githubclient.TreeEntry
	files     map[string]string
	reads     []string
	metadata  int
	trees     int
	treeErr   error
	readErr   error
	afterRead func()
}

func (g *investigationFake) Mode() string { return "anonymous" }
func (g *investigationFake) GetRepo(context.Context, string, string) (githubclient.Repo, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.metadata++
	return g.repo, nil
}
func (g *investigationFake) ResolveRef(context.Context, string, string, string) (string, error) {
	return strings.Repeat("a", 40), nil
}
func (g *investigationFake) GetTree(context.Context, string, string, string, githubclient.TreeLimits) (githubclient.FileTree, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.trees++
	if g.treeErr != nil {
		return githubclient.FileTree{}, g.treeErr
	}
	return githubclient.FileTree{Entries: g.entries}, nil
}
func (g *investigationFake) GetFileContents(ctx context.Context, owner, name, ref string, paths []string, limits githubclient.ContentLimits) (githubclient.FetchedContents, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.readErr != nil {
		g.reads = append(g.reads, paths...)
		return githubclient.FetchedContents{}, g.readErr
	}
	var out githubclient.FetchedContents
	for _, p := range paths {
		g.reads = append(g.reads, p)
		if content, ok := g.files[p]; ok {
			limit := limits.MaxFileBytes
			if limits.MaxTotalBytes < limit {
				limit = limits.MaxTotalBytes
			}
			f := githubclient.FileContent{Path: p, Size: len(content), Content: content}
			if len(f.Content) > limit {
				f.Content = f.Content[:limit]
				f.Truncated = true
			}
			out.Files = append(out.Files, f)
		} else {
			out.Skipped = append(out.Skipped, githubclient.SkippedFile{Path: p, Reason: "404"})
		}
	}
	if g.afterRead != nil {
		g.afterRead()
	}
	return out, nil
}
func investigationFixture() *investigationFake {
	g := &investigationFake{repo: githubclient.Repo{ID: 12, Owner: "owner", Name: "project", FullName: "owner/project", Public: true, DefaultBranch: "main"}, files: map[string]string{"main.go": "package main\nfunc main() {}\n", "README.md": "A project that transforms input.\n", "helper.go": "package main\nfunc work() {}\n"}}
	for p := range g.files {
		g.entries = append(g.entries, githubclient.TreeEntry{Path: p, Type: "blob", Mode: "100644"})
	}
	return g
}
func TestInvestigationSharesFileAndByteCapsAcrossReads(t *testing.T) {
	g := investigationFixture()
	limits := analysis.DefaultLimits()
	limits.MaxFiles = 2
	limits.MaxFileBytes = 20
	limits.MaxTotalBytes = 30
	s, err := BeginInvestigation(context.Background(), g, "owner", "project", limits)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a, err := s.Read("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.ID) > 16 {
		t.Fatal("source handle is unnecessarily difficult to copy reliably")
	}
	again, err := s.Read("main.go")
	if err != nil || again.ID != a.ID || len(g.reads) != 1 {
		t.Fatal("duplicate read renewed a quota")
	}
	b, err := s.Read("README.md")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == b.ID {
		t.Fatal("distinct source handles collided")
	}
	if len(a.Content)+len(b.Content) > 30 {
		t.Fatal("cumulative byte cap reset")
	}
	if _, err := s.Read("helper.go"); !errors.Is(err, ErrInvestigationBudget) {
		t.Fatalf("third file escaped cap: %v", err)
	}
	if len(g.reads) != 2 {
		t.Fatalf("provider reads=%v", g.reads)
	}
}
func TestInvestigationNeverReadsUnlistedSymlinkOrSecrets(t *testing.T) {
	g := investigationFixture()
	g.entries = append(g.entries, githubclient.TreeEntry{Path: "linked.go", Type: "blob", Mode: "120000"}, githubclient.TreeEntry{Path: ".env", Type: "blob", Mode: "100644"}, githubclient.TreeEntry{Path: ".env.example", Type: "blob", Mode: "100644"})
	s, err := BeginInvestigation(context.Background(), g, "owner", "project", analysis.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, p := range []string{"../README.md", "/etc/passwd", ".env", ".env.example", "linked.go", "unlisted.go"} {
		if _, err := s.Read(p); err == nil {
			t.Errorf("unsafe path accepted: %s", p)
		}
	}
	if len(g.reads) != 0 {
		t.Fatalf("unsafe reads reached provider: %v", g.reads)
	}
}

func TestInvestigationCacheProbeDefersListingAndSeedDoesNotReloadIt(t *testing.T) {
	g := investigationFixture()
	s, err := beginInvestigation(context.Background(), g, "owner", "project", analysis.DefaultLimits(), false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if g.trees != 0 || len(g.reads) != 0 {
		t.Fatal("cache probe acquired source")
	}
	if err := s.Seed(); err != nil {
		t.Fatal(err)
	}
	if err := s.Seed(); err != nil {
		t.Fatal(err)
	}
	if g.trees != 1 || len(g.reads) == 0 {
		t.Fatal("listing or source read was repeated or skipped")
	}
}
func TestInvestigationMissingReadConsumesAttemptAndIdentityFailsClosed(t *testing.T) {
	g := investigationFixture()
	g.entries = append(g.entries, githubclient.TreeEntry{Path: "missing.go", Type: "blob", Mode: "100644"})
	limits := analysis.DefaultLimits()
	limits.MaxFiles = 1
	s, err := BeginInvestigation(context.Background(), g, "owner", "project", limits)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Read("missing.go"); err == nil {
		t.Fatal("missing file presented as evidence")
	}
	if _, err := s.Read("main.go"); !errors.Is(err, ErrInvestigationBudget) {
		t.Fatal("failed attempt did not consume file quota")
	}
	g.repo.ID = 99
	if err := s.Verify(); err == nil {
		t.Fatal("reused identity was accepted")
	}
	if len(s.Snapshot().Files) != 0 || s.Snapshot().Complete {
		t.Fatal("identity-withheld content retained")
	}
}
func TestInvestigationSameDeadlineAndCancelledReads(t *testing.T) {
	g := investigationFixture()
	ctx, cancel := context.WithCancel(context.Background())
	s, err := BeginInvestigation(ctx, g, "owner", "project", analysis.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	cancel()
	if _, err := s.Read("main.go"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled read: %v", err)
	}
	if len(g.reads) != 0 {
		t.Fatal("cancelled operation reached provider")
	}
}

func TestInvestigationRejectsBinaryRatherThanRepairingSource(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		cap           int
	}{
		{"NUL", "text\x00binary", 64 << 10},
		{"internal invalid UTF8", strings.Repeat("x", 128<<10) + "\xff" + strings.Repeat("y", (128<<10)-1), 256 << 10},
		{"incomplete original UTF8", "abc\xe2\x82", 64 << 10},
		{"truncated invalid byte", "abc\xffmore-source", 5},
		{"truncated internal invalid UTF8", "ab\xffcdef\xe2\x82\xac", 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := investigationFixture()
			g.files["main.go"] = tc.content
			limits := analysis.DefaultLimits()
			limits.MaxFileBytes = tc.cap
			s, err := BeginInvestigation(context.Background(), g, "owner", "project", limits)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if _, err := s.Read("main.go"); !errors.Is(err, ErrSourceUnavailable) {
				t.Fatalf("invalid source returned as evidence: %v", err)
			}
			if len(s.Snapshot().Files) != 0 {
				t.Fatal("binary source entered the evidence bundle")
			}
			if _, err := s.Read("README.md"); err != nil {
				t.Fatal("valid source unavailable after binary rejection", err)
			}
		})
	}
}

func TestInvestigationOnlyTrimsIncompleteRuneAtActualByteCap(t *testing.T) {
	for _, tc := range []struct {
		content string
		cap     int
	}{
		{"abc€", 4}, {"abc€", 5}, {"abc😀", 4}, {"abc😀", 5}, {"abc😀", 6},
	} {
		t.Run(strconv.Itoa(tc.cap)+tc.content, func(t *testing.T) {
			g := investigationFixture()
			g.files["main.go"] = tc.content
			limits := analysis.DefaultLimits()
			limits.MaxFileBytes = tc.cap
			s, err := BeginInvestigation(context.Background(), g, "owner", "project", limits)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			f, err := s.Read("main.go")
			if err != nil {
				t.Fatal(err)
			}
			if f.Content != "abc" || !f.Truncated || !utf8.ValidString(f.Content) {
				t.Fatalf("unexpected retained prefix: %+v", f)
			}
		})
	}
}

func TestInvestigationRejectsReadCompletedAfterCancellation(t *testing.T) {
	g := investigationFixture()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	g.afterRead = cancel
	s, err := BeginInvestigation(ctx, g, "owner", "project", analysis.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Read("main.go"); !errors.Is(err, context.Canceled) {
		t.Fatalf("late successful provider response bypassed cancellation: %v", err)
	}
	if s.Outcome() != analysis.OutcomePartial || len(s.Snapshot().Files) != 0 {
		t.Fatal("canceled read was retained or called complete")
	}
}

func TestInvestigationSeedReservesBytesAndFilesWithoutRenewingQuota(t *testing.T) {
	g := investigationFixture()
	limits := analysis.DefaultLimits()
	limits.MaxFiles = 8
	limits.MaxFileBytes = 20
	limits.MaxTotalBytes = 30
	s, err := BeginInvestigation(context.Background(), g, "owner", "project", limits)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Seed(); err != nil {
		t.Fatal(err)
	}
	seedReads := len(g.reads)
	if s.used > limits.MaxTotalBytes/2 || len(s.attempted) > limits.MaxFiles/2 {
		t.Fatalf("seed spent the follow-up reserve: bytes=%d files=%d", s.used, len(s.attempted))
	}
	if err := s.Seed(); err != nil {
		t.Fatal(err)
	}
	if len(g.reads) != seedReads {
		t.Fatal("repeated seed acquired another budget")
	}
	var followup string
	for _, candidate := range []string{"README.md", "main.go", "helper.go"} {
		if !s.attempted[candidate] {
			followup = candidate
			break
		}
	}
	if followup == "" {
		t.Fatal("seed consumed every candidate")
	}
	if _, err := s.Read(followup); err != nil {
		t.Fatalf("reserved follow-up failed: %v", err)
	}
	if s.used > limits.MaxTotalBytes || len(s.attempted) > limits.MaxFiles {
		t.Fatal("follow-up exceeded cumulative quotas")
	}
	if err := s.Seed(); err != nil {
		t.Fatal(err)
	}
	if len(g.reads) != seedReads+1 {
		t.Fatal("seed reset quota after follow-up")
	}
}

func TestInvestigationSeedPropagatesCredentialPolicyFailure(t *testing.T) {
	g := investigationFixture()
	g.readErr = githubclient.ErrCredentialPolicy
	s, err := BeginInvestigation(context.Background(), g, "owner", "project", analysis.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Seed(); !errors.Is(err, ErrIdentityWithheld) {
		t.Fatalf("seed hid terminal credential failure: %v", err)
	}
	if s.Outcome() != analysis.OutcomeIdentityWithheld || len(g.reads) != 1 || len(s.Snapshot().Files) != 0 {
		t.Fatal("credential rejection did not terminate acquisition")
	}
}

func TestInvestigationLazyListingCredentialFailureIsTerminal(t *testing.T) {
	g := investigationFixture()
	g.treeErr = githubclient.ErrCredentialPolicy
	s, err := beginInvestigation(context.Background(), g, "owner", "project", analysis.DefaultLimits(), false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Seed(); !errors.Is(err, ErrIdentityWithheld) {
		t.Fatalf("lazy listing hid terminal credential failure: %v", err)
	}
	if s.Outcome() != analysis.OutcomeIdentityWithheld || len(g.reads) != 0 {
		t.Fatal("failed credential check continued source acquisition")
	}
}

func TestInvestigationSeedPropagatesExhaustedRequestQuota(t *testing.T) {
	g := investigationFixture()
	g.readErr = githubclient.ErrCallBudget
	s, err := BeginInvestigation(context.Background(), g, "owner", "project", analysis.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Seed(); !errors.Is(err, githubclient.ErrCallBudget) {
		t.Fatalf("seed hid exhausted shared quota: %v", err)
	}
	if s.Outcome() != analysis.OutcomePartial || len(g.reads) != 1 {
		t.Fatal("exhausted HTTP quota did not stop acquisition")
	}
}

func TestInvestigationIdentityWithheldSurvivesCancellation(t *testing.T) {
	g := investigationFixture()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := BeginInvestigation(ctx, g, "owner", "project", analysis.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	g.repo.ID = 99
	if err := s.Verify(); !errors.Is(err, ErrIdentityWithheld) {
		t.Fatal(err)
	}
	cancel()
	if _, err := s.Read("main.go"); !errors.Is(err, ErrIdentityWithheld) {
		t.Fatalf("terminal outcome changed: %v", err)
	}
	if err := s.Seed(); !errors.Is(err, ErrIdentityWithheld) {
		t.Fatalf("seed ignored terminal identity failure: %v", err)
	}
	if s.Outcome() != analysis.OutcomeIdentityWithheld || len(s.Snapshot().Files) != 0 {
		t.Fatal("withheld outcome was downgraded")
	}
}

func TestInvestigationMaximumAllowedListingRemainsValidEvidence(t *testing.T) {
	g := investigationFixture()
	limits := analysis.DefaultLimits()
	limits.MaxTreeEntries = 10000
	if err := limits.Validate(); err != nil {
		t.Fatal(err)
	}
	for len(g.entries) < limits.MaxTreeEntries {
		g.entries = append(g.entries, githubclient.TreeEntry{Path: "source-" + strconv.Itoa(len(g.entries)) + ".go", Type: "blob", Mode: "100644"})
	}
	s, err := BeginInvestigation(context.Background(), g, "owner", "project", limits)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Read("main.go"); err != nil {
		t.Fatal(err)
	}
	if _, err := validateBundle(s.Snapshot()); err != nil {
		t.Fatalf("valid acquisition exceeded acceptance ceiling: %v", err)
	}
}

func TestInvestigationVerifyCannotUpgradePartialAcquisition(t *testing.T) {
	g := investigationFixture()
	g.entries = append(g.entries, githubclient.TreeEntry{Path: "missing.go", Type: "blob", Mode: "100644"})
	s, err := BeginInvestigation(context.Background(), g, "owner", "project", analysis.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Read("missing.go"); err == nil {
		t.Fatal("expected unavailable source")
	}
	if _, err := s.Read("main.go"); err != nil {
		t.Fatal(err)
	}
	if err := s.Verify(); err != nil {
		t.Fatal(err)
	}
	if s.Outcome() != analysis.OutcomePartial || s.Snapshot().Complete {
		t.Fatal("identity verification upgraded partial acquisition")
	}
}
