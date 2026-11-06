package walkthrough

import (
	"context"
	"errors"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Savio09/gittok-ios-backend/internal/analysis"
	"github.com/Savio09/gittok-ios-backend/internal/corpus"
	"github.com/Savio09/gittok-ios-backend/internal/crawler"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
	"github.com/Savio09/gittok-ios-backend/internal/preview"
)

var (
	ErrInvestigationBudget = errors.New("walkthrough investigation limit reached")
	ErrSourceUnavailable   = errors.New("walkthrough source unavailable")
	ErrIdentityWithheld    = errors.New("walkthrough identity could not be verified")
)

// PublicGitHub cannot inherit the feed's unrestricted credential. Mode is checked
// before any request; real clients also enforce granted scopes on every response.
type PublicGitHub interface {
	analysis.GitHub
	Mode() string
}

// Investigation owns one absolute deadline and cumulative quotas. It is used by
// a single job worker; all tool calls, including follow-ups, use this same owner.
type Investigation struct {
	github     PublicGitHub
	ctx        context.Context
	cancel     context.CancelFunc
	limits     analysis.Limits
	repo       githubclient.Repo
	tree       githubclient.FileTree
	bundle     Bundle
	eligible   map[string]bool
	sourceIDs  map[string]string
	components map[string]bool
	attempted  map[string]bool
	read       map[string]EvidenceFile
	used       int
	verified   bool
	listed     bool
	outcome    analysis.Outcome
}

func BeginInvestigation(parent context.Context, github PublicGitHub, owner, name string, limits analysis.Limits) (*Investigation, error) {
	return beginInvestigation(parent, github, owner, name, limits, true)
}

// Cache probes acquire only current public identity and commit. The worker
// later acquires the listing under this same deadline and request budget.
func beginInvestigation(parent context.Context, github PublicGitHub, owner, name string, limits analysis.Limits, listing bool) (*Investigation, error) {
	if !preview.ValidIdentity(owner, name) || github == nil || (github.Mode() != "anonymous" && github.Mode() != "public-token") {
		return nil, ErrSourceUnavailable
	}
	if err := limits.Validate(); err != nil {
		return nil, err
	}
	bounded, cancel := context.WithTimeout(parent, limits.Timeout)
	s := &Investigation{github: github, cancel: cancel, limits: limits, eligible: map[string]bool{}, components: map[string]bool{}, attempted: map[string]bool{}, read: map[string]EvidenceFile{}, outcome: analysis.OutcomeComplete}
	s.ctx = githubclient.NewRequestBudget(limits.MaxAPICalls, limits.MaxResponseBytes).Context(bounded)
	fail := func(err error) (*Investigation, error) { cancel(); return nil, err }
	r, err := github.GetRepo(s.ctx, owner, name)
	if err != nil {
		return fail(err)
	}
	if !r.Public || r.ID <= 0 || !preview.ValidIdentity(r.Owner, r.Name) || r.DefaultBranch == "" {
		return fail(ErrSourceUnavailable)
	}
	s.repo = r
	commit, err := github.ResolveRef(s.ctx, r.Owner, r.Name, r.DefaultBranch)
	if err != nil {
		return fail(err)
	}
	if !githubclient.ValidCommitID(commit) {
		return fail(ErrSourceUnavailable)
	}
	s.bundle = Bundle{Identity: Identity{RepoID: r.ID, Owner: r.Owner, Name: r.Name, Commit: commit}, CapturedAt: time.Now().UTC()}
	if listing {
		if err := s.loadListing(); err != nil {
			return fail(err)
		}
	}
	return s, nil
}

func (s *Investigation) loadListing() error {
	if s.outcome == analysis.OutcomeIdentityWithheld {
		return ErrIdentityWithheld
	}
	if s.listed {
		return nil
	}
	if err := s.ctx.Err(); err != nil {
		s.outcome = analysis.OutcomePartial
		return err
	}
	var err error
	s.tree, err = s.github.GetTree(s.ctx, s.repo.Owner, s.repo.Name, s.bundle.Identity.Commit, githubclient.TreeLimits{MaxEntries: s.limits.MaxTreeEntries, MaxDepth: s.limits.MaxDepth})
	if errors.Is(err, githubclient.ErrCredentialPolicy) {
		s.withhold()
		return ErrIdentityWithheld
	}
	if ctxErr := s.ctx.Err(); ctxErr != nil {
		s.outcome = analysis.OutcomePartial
		return ctxErr
	}
	if err != nil {
		s.outcome = analysis.OutcomePartial
		return err
	}
	s.listed = true
	if s.tree.Truncated || s.tree.DepthLimited {
		s.note("The bounded listing does not cover every repository path.")
	}
	for _, entry := range s.tree.Entries {
		if entry.Type != "blob" || (entry.Mode != "" && entry.Mode != "100644" && entry.Mode != "100755") || !safeReadPath(entry.Path) {
			continue
		}
		s.eligible[entry.Path] = true
		switch strings.ToLower(path.Base(entry.Path)) {
		case "package.json", "go.mod", "cargo.toml", "pyproject.toml", "setup.py", "pubspec.yaml":
			s.components[path.Dir(entry.Path)] = true
		}
	}
	// Short opaque handles identify acquired source within this pinned session.
	// They do not establish trust: retained content, exact spans and independent
	// review do. Sorted paths keep handles independent of acquisition order.
	paths := make([]string, 0, len(s.eligible))
	for p := range s.eligible {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	s.sourceIDs = make(map[string]string, len(paths))
	for i, p := range paths {
		s.sourceIDs[p] = "source-" + strconv.Itoa(i+1)
	}
	return nil
}

func safeReadPath(p string) bool {
	return safeSourcePath(p) && !corpus.Secretish(p, "")
}

func (s *Investigation) Close()                    { s.cancel() }
func (s *Investigation) Context() context.Context  { return s.ctx }
func (s *Investigation) Outcome() analysis.Outcome { return s.outcome }
func (s *Investigation) Repo() githubclient.Repo {
	r := s.repo
	r.Topics = append([]string(nil), r.Topics...)
	r.Metadata = map[string]githubclient.MetadataField{}
	for k, v := range s.repo.Metadata {
		r.Metadata[k] = v
	}
	return r
}

func (s *Investigation) note(message string) {
	for _, n := range s.bundle.Limitations {
		if n == message {
			return
		}
	}
	s.bundle.Limitations = append(s.bundle.Limitations, message)
}

func (s *Investigation) Seed() error {
	if s.outcome == analysis.OutcomeIdentityWithheld {
		return ErrIdentityWithheld
	}
	if err := s.ctx.Err(); err != nil {
		s.outcome = analysis.OutcomePartial
		return err
	}
	if err := s.loadListing(); err != nil {
		return err
	}
	budget := crawler.DefaultSelectionBudget()
	budget.MaxFiles = min(6, max(1, s.limits.MaxFiles/2))
	budget.MaxEntry = 2
	// These are absolute session caps, so repeated Seed calls cannot renew the
	// initial allocation. Follow-up reads retain at least half the byte/file
	// budget whenever that dimension has room for more than one unit.
	seedBytes := max(1, s.limits.MaxTotalBytes/2)
	for _, selected := range crawler.SelectFiles(s.tree.Entries, budget) {
		if len(s.attempted) >= budget.MaxFiles || s.used >= seedBytes {
			break
		}
		if _, err := s.readSource(selected.Path, seedBytes-s.used); err != nil {
			if errors.Is(err, ErrIdentityWithheld) || errors.Is(err, githubclient.ErrCallBudget) {
				return err
			}
			if ctxErr := s.ctx.Err(); ctxErr != nil {
				return ctxErr
			}
		}
	}
	if err := s.ctx.Err(); err != nil {
		s.outcome = analysis.OutcomePartial
		return err
	}
	return nil
}

func (s *Investigation) Read(p string) (EvidenceFile, error) {
	return s.readSource(p, s.limits.MaxFileBytes)
}

func (s *Investigation) readSource(p string, maxBytes int) (EvidenceFile, error) {
	if s.outcome == analysis.OutcomeIdentityWithheld {
		return EvidenceFile{}, ErrIdentityWithheld
	}
	if err := s.ctx.Err(); err != nil {
		s.outcome = analysis.OutcomePartial
		return EvidenceFile{}, err
	}
	if !s.eligible[p] || !safeReadPath(p) {
		return EvidenceFile{}, ErrSourceUnavailable
	}
	if f, ok := s.read[p]; ok {
		return f, nil
	}
	if s.attempted[p] {
		return EvidenceFile{}, ErrSourceUnavailable
	}
	remaining := min(s.limits.MaxTotalBytes-s.used, maxBytes)
	if len(s.attempted) >= s.limits.MaxFiles || remaining <= 0 {
		s.note("The source-reading budget was reached.")
		return EvidenceFile{}, ErrInvestigationBudget
	}
	s.attempted[p] = true
	s.verified = false
	fetched, err := s.github.GetFileContents(s.ctx, s.repo.Owner, s.repo.Name, s.bundle.Identity.Commit, []string{p}, githubclient.ContentLimits{MaxFiles: 1, MaxFileBytes: min(s.limits.MaxFileBytes, remaining), MaxTotalBytes: remaining})
	if errors.Is(err, githubclient.ErrCredentialPolicy) {
		s.withhold()
		return EvidenceFile{}, ErrIdentityWithheld
	}
	if ctxErr := s.ctx.Err(); ctxErr != nil {
		s.outcome = analysis.OutcomePartial
		return EvidenceFile{}, ctxErr
	}
	if err != nil {
		s.outcome = analysis.OutcomePartial
		return EvidenceFile{}, err
	}
	if len(fetched.Files) != 1 || fetched.Files[0].Path != p {
		s.outcome = analysis.OutcomePartial
		s.note("Some listed source could not be acquired.")
		return EvidenceFile{}, ErrSourceUnavailable
	}
	f := fetched.Files[0]
	cap := min(s.limits.MaxFileBytes, remaining)
	if len(f.Content) > cap {
		f.Content = f.Content[:cap]
		f.Truncated = true
	}
	s.used += len(f.Content)
	text, valid := retainedUTF8(f.Content, f.Truncated && len(f.Content) == cap)
	unavailable := !valid || text == "" || strings.ContainsRune(text, 0) || corpus.Secretish(p, text)
	if err := s.ctx.Err(); err != nil {
		s.outcome = analysis.OutcomePartial
		return EvidenceFile{}, err
	}
	if unavailable {
		s.note("Some source was withheld by the content policy.")
		return EvidenceFile{}, ErrSourceUnavailable
	}
	f.Content = text
	component := ""
	for dir := path.Dir(p); dir != "." && dir != "/"; dir = path.Dir(dir) {
		if s.components[dir] {
			component = dir
			break
		}
	}
	e := EvidenceFile{ID: s.sourceIDs[p], Path: p, Component: component, Content: f.Content, Truncated: f.Truncated}
	if e.Truncated {
		s.note("Some inspected source was clipped to the configured byte limit.")
	}
	if err := s.ctx.Err(); err != nil {
		s.outcome = analysis.OutcomePartial
		return EvidenceFile{}, err
	}
	s.read[p] = e
	return e, nil
}

// retainedUTF8 never repairs arbitrary invalid source. A true byte cap may cut
// through one final rune; examine at most its four bytes, then validate the
// retained prefix once. This keeps work linear even for hostile binary input.
func retainedUTF8(content string, clipped bool) (string, bool) {
	if utf8.ValidString(content) {
		return content, true
	}
	if !clipped {
		return "", false
	}
	start := len(content) - 1
	for start > 0 && !utf8.RuneStart(content[start]) && len(content)-start < utf8.UTFMax {
		start--
	}
	if utf8.FullRuneInString(content[start:]) || !utf8.ValidString(content[:start]) {
		return "", false
	}
	return content[:start], true
}

func (s *Investigation) FindPaths(query string, limit int) []string {
	if len(query) > 160 {
		return nil
	}
	limit = min(max(limit, 1), 40)
	var found []string
	for p := range s.eligible {
		if strings.Contains(strings.ToLower(p), strings.ToLower(query)) {
			found = append(found, p)
		}
	}
	sort.Slice(found, func(i, j int) bool {
		if len(found[i]) != len(found[j]) {
			return len(found[i]) < len(found[j])
		}
		return found[i] < found[j]
	})
	return found[:min(len(found), limit)]
}

func (s *Investigation) Snapshot() Bundle {
	b := s.bundle
	b.Complete = s.outcome == analysis.OutcomeComplete && s.verified
	b.Limitations = append([]string(nil), b.Limitations...)
	b.Files = nil
	b.Paths = nil
	if s.outcome == analysis.OutcomeIdentityWithheld {
		return b
	}
	for p := range s.eligible {
		b.Paths = append(b.Paths, p)
	}
	sort.Strings(b.Paths)
	for _, p := range b.Paths {
		if f, ok := s.read[p]; ok {
			b.Files = append(b.Files, f)
		}
	}
	return b
}

func (s *Investigation) withhold() {
	s.outcome = analysis.OutcomeIdentityWithheld
	s.verified = false
	s.read = map[string]EvidenceFile{}
	s.eligible = map[string]bool{}
	s.sourceIDs = map[string]string{}
}

// Verify is part of the same deadline/quota. It runs after model review and
// before publication; failed public identity checks withhold every file artifact.
func (s *Investigation) Verify() error {
	if s.outcome == analysis.OutcomeIdentityWithheld {
		return ErrIdentityWithheld
	}
	r, err := s.github.GetRepo(s.ctx, s.repo.Owner, s.repo.Name)
	if err != nil || s.ctx.Err() != nil || !r.Public || r.ID != s.repo.ID || r.Owner != s.repo.Owner || r.Name != s.repo.Name {
		s.withhold()
		return ErrIdentityWithheld
	}
	s.verified = true
	return nil
}
