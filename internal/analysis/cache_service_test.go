package analysis

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

// countingGitHub counts acquisition calls and serves mutable metadata, so a
// test can change visibility, identity or name between requests.
type countingGitHub struct {
	mu                               sync.Mutex
	repo                             githubclient.Repo
	metaErr                          error
	contents                         func(context.Context) (githubclient.FetchedContents, error)
	metadata, refs, trees, contentsN int
}

func (c *countingGitHub) GetRepo(context.Context, string, string) (githubclient.Repo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metadata++
	return c.repo, c.metaErr
}
func (c *countingGitHub) ResolveRef(context.Context, string, string, string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.refs++
	return "0123456789abcdef0123456789abcdef01234567", nil
}
func (c *countingGitHub) GetTree(context.Context, string, string, string, githubclient.TreeLimits) (githubclient.FileTree, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.trees++
	return githubclient.FileTree{Entries: []githubclient.TreeEntry{{Path: "package.json", Type: "blob"}}}, nil
}
func (c *countingGitHub) GetFileContents(ctx context.Context, _, _, _ string, _ []string, _ githubclient.ContentLimits) (githubclient.FetchedContents, error) {
	c.mu.Lock()
	c.contentsN++
	f := c.contents
	c.mu.Unlock()
	if f != nil {
		return f(ctx)
	}
	return githubclient.FetchedContents{Files: []githubclient.FileContent{{Path: "package.json", Content: `{"name":"x","exports":"./i.js"}`}}}, nil
}

func (c *countingGitHub) set(f func(*countingGitHub)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	f(c)
}

func cachedService(t *testing.T, gh GitHub, clock *fakeClock) *Service {
	t.Helper()
	s := NewService(gh)
	s.publicOnly = true
	return s.UseCache(openTestCache(t, t.TempDir(), testLimits(), clock))
}

func publicMeta() githubclient.Repo {
	return githubclient.Repo{ID: 7, Owner: "o", Name: "r", FullName: "o/r", Public: true, DefaultBranch: "main"}
}

// R6 (archived control): a repeat preview is served from the captured
// snapshot after one visibility lookup, with no ref, tree or contents reads.
func TestReviewRepeatPreviewReusesTheCapturedSnapshot(t *testing.T) {
	gh := &countingGitHub{repo: publicMeta()}
	s := cachedService(t, gh, &fakeClock{time.Now()})
	first, err := s.Preview(context.Background(), "o", "r")
	if err != nil {
		t.Fatal(err)
	}
	coldMetadata := gh.metadata
	second, err := s.Preview(context.Background(), "o", "r")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Error("the warm preview differs from the captured snapshot")
	}
	if gh.metadata-coldMetadata != 1 || gh.refs != 1 || gh.trees != 1 || gh.contentsN != 1 {
		t.Errorf("warm request reads: metadata +%d, refs %d, trees %d, contents %d", gh.metadata-coldMetadata, gh.refs, gh.trees, gh.contentsN)
	}
}

// Every hit is gated by current metadata: a repository that became private,
// changed identity, was renamed, or whose visibility cannot be established is
// never answered from the cache.
func TestCachedPreviewsAreGatedByCurrentMetadata(t *testing.T) {
	for name, tc := range map[string]struct {
		change  func(*countingGitHub)
		wantErr error // nil: a fresh acquisition is expected instead
	}{
		"became private":      {func(c *countingGitHub) { c.repo.Public = false }, githubclient.ErrNotFound},
		"visibility unknown":  {func(c *countingGitHub) { c.metaErr = errors.New("transport") }, errAny},
		"credential rejected": {func(c *countingGitHub) { c.metaErr = &githubclient.PolicyError{Reason: "scopes"} }, githubclient.ErrCredentialPolicy},
		"name reused":         {func(c *countingGitHub) { c.repo.ID = 8 }, nil},
		"renamed":             {func(c *countingGitHub) { c.repo.Name, c.repo.FullName = "r2", "o/r2" }, nil},
		"transferred":         {func(c *countingGitHub) { c.repo.Owner, c.repo.FullName = "new", "new/r" }, nil},
	} {
		t.Run(name, func(t *testing.T) {
			gh := &countingGitHub{repo: publicMeta()}
			s := cachedService(t, gh, &fakeClock{time.Now()})
			if _, err := s.Preview(context.Background(), "o", "r"); err != nil {
				t.Fatal(err)
			}
			gh.set(tc.change)
			p, err := s.Preview(context.Background(), "o", "r")
			switch {
			case tc.wantErr == errAny:
				if err == nil {
					t.Fatalf("served %+v without established visibility", p)
				}
			case tc.wantErr != nil:
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
			default:
				if err != nil {
					t.Fatal(err)
				}
				if gh.trees != 2 {
					t.Fatalf("served the old identity's snapshot (trees read: %d)", gh.trees)
				}
				if p.Owner != gh.repo.Owner || p.Name != gh.repo.Name {
					t.Fatalf("preview identity %s/%s, metadata %s/%s", p.Owner, p.Name, gh.repo.Owner, gh.repo.Name)
				}
			}
		})
	}
}

var errAny = errors.New("any error")

func TestOnlyCacheableOutcomesAreStored(t *testing.T) {
	transient := func(context.Context) (githubclient.FetchedContents, error) {
		return githubclient.FetchedContents{Skipped: []githubclient.SkippedFile{{Path: "package.json", Reason: "fetch error: 502"}}}, nil
	}
	unsupported := func(context.Context) (githubclient.FetchedContents, error) {
		return githubclient.FetchedContents{Files: []githubclient.FileContent{{Path: "package.json", Content: "{}"}}}, nil
	}
	for name, tc := range map[string]struct {
		contents  func(context.Context) (githubclient.FetchedContents, error)
		wantTrees int
	}{
		"transient file failure is reacquired": {transient, 2},
		"unsupported but complete is reused":   {unsupported, 1},
	} {
		t.Run(name, func(t *testing.T) {
			gh := &countingGitHub{repo: publicMeta(), contents: tc.contents}
			s := cachedService(t, gh, &fakeClock{time.Now()})
			for i := 0; i < 2; i++ {
				if _, err := s.Preview(context.Background(), "o", "r"); err != nil {
					t.Fatal(err)
				}
			}
			if gh.trees != tc.wantTrees {
				t.Errorf("tree reads %d, want %d", gh.trees, tc.wantTrees)
			}
		})
	}
}

func TestExpiredPreviewsAreReacquiredAtTheCurrentRevision(t *testing.T) {
	gh := &countingGitHub{repo: publicMeta()}
	clock := &fakeClock{time.Now()}
	s := cachedService(t, gh, clock)
	if _, err := s.Preview(context.Background(), "o", "r"); err != nil {
		t.Fatal(err)
	}
	clock.t = clock.t.Add(31 * time.Minute)
	if _, err := s.Preview(context.Background(), "o", "r"); err != nil {
		t.Fatal(err)
	}
	if gh.refs != 2 || gh.trees != 2 {
		t.Errorf("expired entry not reacquired: refs %d, trees %d", gh.refs, gh.trees)
	}
}
