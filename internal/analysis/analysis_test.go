package analysis

import (
	"context"
	"errors"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

type fakeGitHub struct {
	repo    githubclient.Repo
	treeErr error
	refs    []string
}

func (f *fakeGitHub) GetRepo(context.Context, string, string) (githubclient.Repo, error) {
	return f.repo, nil
}
func (f *fakeGitHub) ResolveRef(context.Context, string, string, string) (string, error) {
	return "0123456789abcdef0123456789abcdef01234567", nil
}
func (f *fakeGitHub) GetTree(_ context.Context, _, _, ref string, _ githubclient.TreeLimits) (githubclient.FileTree, error) {
	f.refs = append(f.refs, ref)
	return githubclient.FileTree{Entries: []githubclient.TreeEntry{{Path: "package.json", Type: "blob"}}}, f.treeErr
}
func (f *fakeGitHub) GetFileContents(_ context.Context, _, _, ref string, paths []string, _ githubclient.ContentLimits) (githubclient.FetchedContents, error) {
	f.refs = append(f.refs, ref)
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

func TestAcquireMetadataFailure(t *testing.T) {
	f := &fakeGitHub{treeErr: errors.New("transport unavailable"), repo: githubclient.Repo{DefaultBranch: "trunk"}}
	if _, err := NewService(f).Acquire(context.Background(), "o", "r"); err == nil {
		t.Fatal("expected acquisition error")
	}
}
