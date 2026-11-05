// Package analysis acquires bounded repository evidence for commands and HTTP.
package analysis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/crawler"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
	"github.com/Savio09/gittok-ios-backend/internal/preview"
)

// GitHub is the acquisition interface; selection and interpretation stay local.
type GitHub interface {
	GetRepo(context.Context, string, string) (githubclient.Repo, error)
	ResolveRef(context.Context, string, string, string) (string, error)
	GetTree(context.Context, string, string, string, githubclient.TreeLimits) (githubclient.FileTree, error)
	GetFileContents(context.Context, string, string, string, []string, githubclient.ContentLimits) (githubclient.FetchedContents, error)
}

// Outcome is the typed result of an acquisition, deciding whether it may be
// reused. A nil error is not enough: acquisition also returns usable partial
// results after transient failures or a withheld identity.
type Outcome int

const (
	// OutcomeUnknown is the zero value: an empty or failed Result is never
	// reusable by default.
	OutcomeUnknown Outcome = iota
	// OutcomeComplete: a bounded inspection ran to completion. Declared caps
	// (clipped files, a truncated tree) and unsupported layouts still count.
	OutcomeComplete
	// OutcomePartial: a transport, provider or deadline failure cut acquisition
	// short; the result is truthful but not reusable.
	OutcomePartial
	// OutcomeIdentityWithheld: the repository's identity could not be
	// reconfirmed, so file evidence was withheld.
	OutcomeIdentityWithheld
)

// Cacheable reports whether the result may be reused for later requests.
func (o Outcome) Cacheable() bool { return o == OutcomeComplete }

func (o Outcome) String() string {
	switch o {
	case OutcomeComplete:
		return "complete"
	case OutcomePartial:
		return "partial"
	case OutcomeIdentityWithheld:
		return "identity-withheld"
	}
	return "unknown"
}

// policyRejected reports a credential-policy error, which ends acquisition
// with no result rather than a partial one.
func policyRejected(err error) bool { return errors.Is(err, githubclient.ErrCredentialPolicy) }

// Result preserves exactly the evidence consumed by the classifier.
type Result struct {
	Repo           githubclient.Repo
	Tree           githubclient.FileTree
	Requested      []string
	Fetched        githubclient.FetchedContents
	Inspection     crawler.Inspection
	Classification crawler.Classification
	CapturedAt     time.Time
	Limitations    []string // internal diagnostics: paths and technical reasons
	Coverage       preview.Coverage
	Outcome        Outcome
	transient      bool // a selected file failed for a transient reason
}

// Service has no mutable request state and may be shared by HTTP handlers.
type Service struct {
	github     GitHub
	timeout    time.Duration
	publicOnly bool
	cache      *PreviewCache // public services only
	flights    *group        // coalescing and admission, when enabled

	afterCacheMiss func() // test hook: runs after a request's cache lookup misses
}

// UseCache enables reuse of completed public previews. It has no effect on a
// service that is not public-only, whose results are never stored.
func (s *Service) UseCache(c *PreviewCache) *Service {
	if s.publicOnly {
		s.cache = c
	}
	return s
}

func NewService(github GitHub) *Service { return &Service{github: github, timeout: 30 * time.Second} }

// NewPublicService cannot inherit the server's shared token. Anonymous requests
// cannot retrieve newly private evidence after an initially public metadata read.
func NewPublicService(options ...githubclient.Option) *Service {
	s := NewService(githubclient.New("", options...))
	s.publicOnly = true
	return s.UseAdmission(DefaultAdmissionLimits())
}

// NewPublicServiceFromClient serves public previews through c, which must be
// anonymous or vetted by githubclient.NewPublic. A general-purpose token
// client (such as the feed's) is refused: its broader access could expose
// data the public endpoint must never return.
func NewPublicServiceFromClient(c *githubclient.Client) (*Service, error) {
	if c == nil || (c.Mode() != "anonymous" && c.Mode() != "public-token") {
		return nil, errors.New("public previews need an anonymous or vetted public-only client")
	}
	s := NewService(c)
	s.publicOnly = true
	return s.UseAdmission(DefaultAdmissionLimits()), nil
}

// Acquire reads metadata and canonically selected files without executing code.
func (s *Service) Acquire(parent context.Context, owner, name string) (Result, error) {
	if err := parent.Err(); err != nil {
		return Result{}, err
	}
	repo, err := s.metadata(parent, owner, name)
	if err != nil {
		return Result{}, err
	}
	return s.acquireFrom(parent, repo, owner, name)
}

// metadata reads and gates current repository metadata: a public service
// accepts only a repository GitHub reports public with a valid identity.
func (s *Service) metadata(parent context.Context, owner, name string) (githubclient.Repo, error) {
	ctx, cancel := context.WithTimeout(parent, s.timeout)
	defer cancel()
	repo, err := s.github.GetRepo(ctx, owner, name)
	if err != nil {
		return githubclient.Repo{}, fmt.Errorf("metadata: %w", err)
	}
	if err := parent.Err(); err != nil {
		return githubclient.Repo{}, err
	}
	if s.publicOnly && !repo.Public {
		return githubclient.Repo{}, githubclient.ErrNotFound
	}
	if s.publicOnly && !preview.ValidIdentity(repo.Owner, repo.Name) {
		return githubclient.Repo{}, fmt.Errorf("invalid GitHub repository identity")
	}
	return repo, nil
}

// acquireFrom reads canonically selected files for already gated metadata.
func (s *Service) acquireFrom(parent context.Context, repo githubclient.Repo, owner, name string) (Result, error) {
	ctx, cancel := context.WithTimeout(parent, s.timeout)
	defer cancel()
	var err error
	a := Result{Repo: repo, CapturedAt: time.Now().UTC(), Outcome: OutcomeComplete}
	// Follow canonical identity after a redirected metadata lookup. A reused
	// alias must never select another repository's commits for this metadata.
	if preview.ValidIdentity(repo.Owner, repo.Name) {
		owner, name = repo.Owner, repo.Name
	}
	finish := func(stage string) (Result, error) {
		if err := parent.Err(); err != nil {
			return Result{}, err
		}
		if stage != "" {
			a.Limitations = append(a.Limitations, stage)
		}
		// An owner/name can be renamed and reused while acquisition is in flight.
		// Verify stable identity before publishing public file citations. On a
		// failed check, retain only the already captured provider metadata.
		if (s.publicOnly || repo.ID > 0) && a.Inspection.Revision.Resolved {
			verified, err := s.github.GetRepo(ctx, owner, name)
			if policyRejected(err) {
				return Result{}, err
			}
			if err != nil || repo.ID <= 0 || verified.ID != repo.ID || (s.publicOnly && !verified.Public) || verified.Owner != repo.Owner || verified.Name != repo.Name {
				a.Fetched = githubclient.FetchedContents{}
				a.Tree = githubclient.FileTree{Truncated: true}
				a.Requested = nil
				a.Inspection.Revision = crawler.Revision{}
				a.Coverage = preview.Coverage{IdentityUnverified: true}
				a.Outcome = OutcomeIdentityWithheld
				a.Limitations = append(a.Limitations, "Repository identity could not be revalidated after acquisition; file evidence was withheld.")
			}
			if err := parent.Err(); err != nil {
				return Result{}, err
			}
		}
		for _, skipped := range a.Fetched.Skipped {
			a.Limitations = append(a.Limitations, fmt.Sprintf("Selected file %q could not be read (%s).", skipped.Path, skipped.Reason))
		}
		if a.Coverage.FilesNotAcquired || a.Coverage.ContentsIncomplete || a.transient {
			if a.Outcome == OutcomeComplete {
				a.Outcome = OutcomePartial
			}
		}
		for _, file := range a.Fetched.Files {
			if file.Truncated {
				a.Limitations = append(a.Limitations, fmt.Sprintf("Selected file %q was clipped to %d of %d bytes.", file.Path, len(file.Content), file.Size))
			}
		}
		a.Inspection = crawler.NewInspection(repo.Language, a.Inspection.Revision, a.Tree, a.Fetched)
		a.Classification = crawler.Classify(a.Inspection)
		return a, nil
	}
	a.Tree.Truncated = true // until a successful tree establishes listing scope
	ref := repo.DefaultBranch
	if ref == "" {
		a.Coverage.FilesNotAcquired = true
		return finish("GitHub did not report a default branch; file evidence was not acquired.")
	}
	if ctx.Err() != nil {
		a.Coverage.FilesNotAcquired = true
		return finish("Analysis deadline reached after metadata acquisition.")
	}
	ref, err = s.github.ResolveRef(ctx, owner, name, ref)
	if policyRejected(err) {
		return Result{}, err
	}
	if err != nil || !githubclient.ValidCommitID(ref) {
		a.Coverage.FilesNotAcquired = true
		return finish("The default branch could not be pinned to a commit; file evidence was not acquired.")
	}
	a.Inspection.Revision = crawler.Revision{Ref: ref, Resolved: true}
	if ctx.Err() != nil {
		a.Coverage.FilesNotAcquired = true
		return finish("Analysis stopped before the tree could be acquired.")
	}
	tree, err := s.github.GetTree(ctx, owner, name, ref, githubclient.DefaultTreeLimits())
	if policyRejected(err) {
		return Result{}, err
	}
	if err != nil {
		a.Coverage.FilesNotAcquired = true
		return finish("The file tree could not be acquired; only metadata evidence is available.")
	}
	a.Tree = tree
	for _, entry := range tree.Entries {
		if entry.Mode == "120000" {
			a.Coverage.SymlinksExcluded = true
			a.Limitations = append(a.Limitations, "Symlink contents were excluded from inspection.")
			break
		}
	}
	selected := crawler.SelectFiles(tree.Entries, crawler.DefaultSelectionBudget())
	paths := make([]string, len(selected))
	for i, f := range selected {
		paths[i] = f.Path
	}
	a.Requested = paths
	if ctx.Err() != nil {
		a.Coverage.FilesNotAcquired = true
		return finish("Analysis stopped before selected contents could be acquired.")
	}
	fetched, err := s.github.GetFileContents(ctx, owner, name, ref, paths, githubclient.DefaultContentLimits())
	if policyRejected(err) {
		return Result{}, err
	}
	// Raw provider messages are diagnostic data, not public inspection limits.
	// The shared result feeds HTTP as well as the command renderer.
	for i := range fetched.Skipped {
		if strings.HasPrefix(fetched.Skipped[i].Reason, "fetch error:") {
			fetched.Skipped[i].Reason = "GitHub acquisition failed"
			a.transient = true // a provider/transport failure, not a bounded absence
		}
	}
	a.Fetched = fetched
	if err != nil {
		a.Coverage.ContentsIncomplete = true
		return finish("Selected contents could not all be acquired; only retained evidence was inspected.")
	}
	return finish("")
}
