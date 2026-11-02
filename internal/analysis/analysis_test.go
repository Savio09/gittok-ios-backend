package analysis

import (
	"context"
	"errors"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

type fakeGitHub struct {
	repo     githubclient.Repo
	treeErr  error
	refs     []string
	ref      string
	refErr   error
	contents func(context.Context) (githubclient.FetchedContents, error)
}

func (f *fakeGitHub) GetRepo(context.Context, string, string) (githubclient.Repo, error) {
	return f.repo, nil
}
func (f *fakeGitHub) ResolveRef(context.Context, string, string, string) (string, error) {
	if f.refErr != nil {
		return "", f.refErr
	}
	if f.ref != "" {
		return f.ref, nil
	}
	return "0123456789abcdef0123456789abcdef01234567", nil
}
func (f *fakeGitHub) GetTree(_ context.Context, _, _, ref string, _ githubclient.TreeLimits) (githubclient.FileTree, error) {
	f.refs = append(f.refs, ref)
	return githubclient.FileTree{Entries: []githubclient.TreeEntry{{Path: "package.json", Type: "blob"}}}, f.treeErr
}
func (f *fakeGitHub) GetFileContents(ctx context.Context, _, _, ref string, paths []string, _ githubclient.ContentLimits) (githubclient.FetchedContents, error) {
	f.refs = append(f.refs, ref)
	if f.contents != nil {
		return f.contents(ctx)
	}
	return githubclient.FetchedContents{Files: []githubclient.FileContent{{Path: "package.json", Content: `{"name":"renamable","exports":"./index.js"}`}}}, nil
}

func TestAcquireArbitraryRepository(t *testing.T) {
	f := &fakeGitHub{repo: githubclient.Repo{Owner: "unlisted", Name: "anything", FullName: "unlisted/anything", DefaultBranch: "trunk"}}
	got, err := NewService(f).Acquire(context.Background(), "unlisted", "anything")
	if err != nil {
		t.Fatal(err)
	}
	if got.Repo.FullName != "unlisted/anything" || len(got.Requested) != 1 || len(got.Inspection.Files) != 1 || got.Classification.Kind != apimodel.KindLibrary {
		t.Fatalf("incomplete shared pipeline: %+v", got)
	}
}

func TestAcquireTreeFailurePreservesMetadata(t *testing.T) {
	f := &fakeGitHub{treeErr: errors.New("transport unavailable"), repo: githubclient.Repo{DefaultBranch: "trunk"}}
	got, err := NewService(f).Acquire(context.Background(), "o", "r")
	if err != nil || len(got.Limitations) == 0 || len(got.Inspection.Files) != 0 {
		t.Fatalf("want truthful partial result: %+v, %v", got, err)
	}
}

func TestAcquirePinsAllFileReads(t *testing.T) {
	f := &fakeGitHub{repo: githubclient.Repo{DefaultBranch: "moving"}}
	got, err := NewService(f).Acquire(context.Background(), "o", "r")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Inspection.Revision.Resolved {
		t.Fatal("inspection not pinned")
	}
	for _, ref := range f.refs {
		if ref != "0123456789abcdef0123456789abcdef01234567" {
			t.Fatalf("mutable read %q", ref)
		}
	}
}

func TestAcquireUnresolvedRefCannotCiteFiles(t *testing.T) {
	for _, ref := range []string{"main", "abc", "0123456789abcdef0123456789abcdef0123456z"} {
		t.Run(ref, func(t *testing.T) {
			f := &fakeGitHub{repo: githubclient.Repo{DefaultBranch: "main"}, ref: ref}
			got, err := NewService(f).Acquire(context.Background(), "o", "r")
			if err != nil || len(f.refs) > 0 || len(got.Limitations) == 0 || got.Inspection.Revision.Resolved {
				t.Fatalf("unsafe revision: %+v %v", got, err)
			}
		})
	}
}

func TestAcquireMissingDefaultBranchDoesNotGuessMain(t *testing.T) {
	f := &fakeGitHub{}
	got, err := NewService(f).Acquire(context.Background(), "o", "r")
	if err != nil || len(f.refs) > 0 || len(got.Limitations) == 0 {
		t.Fatalf("guessed branch: %+v %v", got, err)
	}
}
