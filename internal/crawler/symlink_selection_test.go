package crawler

import (
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
	"testing"
)

func TestSelectFilesDoesNotDereferenceSymlinks(t *testing.T) {
	entries := []githubclient.TreeEntry{{Path: "README.md", Type: "blob", Mode: "120000"}, {Path: "package.json", Type: "blob", Mode: "120000"}, {Path: "go.mod", Type: "blob", Mode: "100644"}}
	got := SelectFiles(entries, DefaultSelectionBudget())
	if len(got) != 1 || got[0].Path != "go.mod" {
		t.Fatalf("selected symlink evidence: %+v", got)
	}
}
