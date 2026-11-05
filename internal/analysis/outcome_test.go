package analysis

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

var policyErr = &githubclient.PolicyError{Reason: "credential scope is not public-only"}

// recheckFailing wraps fakeGitHub so the post-acquisition identity read fails.
type recheckFailing struct {
	*fakeGitHub
	reads int
	err   error
}

func (r *recheckFailing) GetRepo(ctx context.Context, o, n string) (githubclient.Repo, error) {
	r.reads++
	if r.reads > 1 {
		return githubclient.Repo{}, r.err
	}
	return r.fakeGitHub.GetRepo(ctx, o, n)
}

func publicRepo() githubclient.Repo {
	return githubclient.Repo{ID: 7, Owner: "o", Name: "r", FullName: "o/r", Public: true, DefaultBranch: "main"}
}

// A credential-policy rejection at any stage after metadata yields no preview
// at all: never metadata-only success, never partial file evidence.
func TestCredentialPolicyRejectionAtAnyStageYieldsNoPreview(t *testing.T) {
	for name, gh := range map[string]GitHub{
		"ref":  &fakeGitHub{repo: publicRepo(), refErr: policyErr},
		"tree": &fakeGitHub{repo: publicRepo(), treeErr: policyErr},
		"contents": &fakeGitHub{repo: publicRepo(), contents: func(context.Context) (githubclient.FetchedContents, error) {
			return githubclient.FetchedContents{}, policyErr
		}},
		"identity": &recheckFailing{fakeGitHub: &fakeGitHub{repo: publicRepo()}, err: policyErr},
	} {
		t.Run(name, func(t *testing.T) {
			s := NewService(gh)
			s.publicOnly = true
			a, err := s.Acquire(context.Background(), "o", "r")
			if !errors.Is(err, githubclient.ErrCredentialPolicy) {
				t.Fatalf("Acquire: err=%v, want the credential-policy error", err)
			}
			if a.Repo.FullName != "" || len(a.Inspection.Files) != 0 || a.Outcome.Cacheable() {
				t.Fatalf("returned data after a policy rejection: %+v", a)
			}
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1/repos/o%2Fr/preview", nil)
			req.SetPathValue("id", "o/r")
			s.Handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("handler status %d for a credential-policy rejection: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// Only a completed bounded inspection is eligible for reuse; a nil error is
// not enough.
func TestAcquisitionOutcomeDecidesCacheEligibility(t *testing.T) {
	ok := func(fs ...githubclient.FileContent) func(context.Context) (githubclient.FetchedContents, error) {
		return func(context.Context) (githubclient.FetchedContents, error) {
			return githubclient.FetchedContents{Files: fs}, nil
		}
	}
	for name, tc := range map[string]struct {
		gh        GitHub
		cacheable bool
	}{
		"complete":                  {&fakeGitHub{repo: publicRepo()}, true},
		"unsupported but bounded":   {&fakeGitHub{repo: publicRepo(), contents: ok(githubclient.FileContent{Path: "package.json", Content: "{}"})}, true},
		"clipped by a declared cap": {&fakeGitHub{repo: publicRepo(), contents: ok(githubclient.FileContent{Path: "package.json", Content: "{", Size: 99, Truncated: true})}, true},
		"file not found": {&fakeGitHub{repo: publicRepo(), contents: func(context.Context) (githubclient.FetchedContents, error) {
			return githubclient.FetchedContents{Skipped: []githubclient.SkippedFile{{Path: "package.json", Reason: "not found"}}}, nil
		}}, true},
		"transient file failure": {&fakeGitHub{repo: publicRepo(), contents: func(context.Context) (githubclient.FetchedContents, error) {
			return githubclient.FetchedContents{Skipped: []githubclient.SkippedFile{{Path: "package.json", Reason: "fetch error: 502"}}}, nil
		}}, false},
		"contents failure": {&fakeGitHub{repo: publicRepo(), contents: func(context.Context) (githubclient.FetchedContents, error) {
			return githubclient.FetchedContents{}, errors.New("boom")
		}}, false},
		"tree failure":             {&fakeGitHub{repo: publicRepo(), treeErr: errors.New("boom")}, false},
		"unpinned ref":             {&fakeGitHub{repo: publicRepo(), refErr: errors.New("boom")}, false},
		"no default branch":        {&fakeGitHub{repo: githubclient.Repo{ID: 7, Owner: "o", Name: "r", Public: true}}, false},
		"identity not reconfirmed": {&recheckFailing{fakeGitHub: &fakeGitHub{repo: publicRepo()}, err: errors.New("transport")}, false},
	} {
		t.Run(name, func(t *testing.T) {
			s := NewService(tc.gh)
			s.publicOnly = true
			a, err := s.Acquire(context.Background(), "o", "r")
			if err != nil {
				t.Fatal(err)
			}
			if a.Outcome.Cacheable() != tc.cacheable {
				t.Errorf("cacheable=%v, want %v (outcome %v)", a.Outcome.Cacheable(), tc.cacheable, a.Outcome)
			}
		})
	}
}
