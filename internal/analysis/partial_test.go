package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

func TestPreviewSanitizesPerFileProviderErrors(t *testing.T) {
	f := &fakeGitHub{repo: githubclient.Repo{Owner: "o", Name: "r", DefaultBranch: "main"}, contents: func(context.Context) (githubclient.FetchedContents, error) {
		return githubclient.FetchedContents{Skipped: []githubclient.SkippedFile{{Path: "package.json", Reason: "fetch error: sensitive provider details"}}}, nil
	}}
	p, err := NewService(f).Preview(context.Background(), "o", "r")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(p)
	if strings.Contains(string(b), "sensitive provider") {
		t.Fatalf("leaked upstream contents error: %s", b)
	}
	if !strings.Contains(string(b), "could not be read") {
		t.Fatal("hid unread file limitation")
	}
}

func TestNonRoleBearingSkippedAndTruncatedFilesRemainVisible(t *testing.T) {
	f := &fakeGitHub{repo: githubclient.Repo{Owner: "o", Name: "r", DefaultBranch: "main", Language: "Ruby"}, contents: func(context.Context) (githubclient.FetchedContents, error) {
		return githubclient.FetchedContents{Files: []githubclient.FileContent{{Path: "LICENSE", Content: "MIT", Size: 100, Truncated: true}}, Skipped: []githubclient.SkippedFile{{Path: "README.md", Reason: "not found"}}}, nil
	}}
	p, err := NewService(f).Preview(context.Background(), "o", "r")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(p)
	if !strings.Contains(string(b), "README.md") || !strings.Contains(string(b), "LICENSE") {
		t.Fatalf("hidden acquisition gap: %s", b)
	}
}

func TestOwnDeadlineRetainsFetchedFactsButCallerCancelDiscardsResult(t *testing.T) {
	for _, parentCanceled := range []bool{false, true} {
		t.Run(map[bool]string{false: "own-deadline", true: "caller-cancel"}[parentCanceled], func(t *testing.T) {
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			f := &fakeGitHub{repo: githubclient.Repo{Owner: "o", Name: "r", DefaultBranch: "main"}, contents: func(ctx context.Context) (githubclient.FetchedContents, error) {
				if parentCanceled {
					cancel()
				} else {
					<-ctx.Done()
				}
				return githubclient.FetchedContents{Files: []githubclient.FileContent{{Path: "package.json", Content: `{"dependencies":{"react":"19"}}`}}}, ctx.Err()
			}}
			s := NewService(f)
			s.timeout = 5 * time.Millisecond
			a, err := s.Acquire(parent, "o", "r")
			if parentCanceled {
				if !errors.Is(err, context.Canceled) || len(a.Inspection.Files) != 0 {
					t.Fatalf("cancellation result %+v, %v", a, err)
				}
			} else if err != nil || len(a.Inspection.Files) != 1 || len(a.Limitations) == 0 {
				t.Fatalf("lost retained facts %+v %v", a, err)
			}
		})
	}
}
