package preview

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/corpus"
	"github.com/Savio09/gittok-ios-backend/internal/crawler"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

// Regression controls from the November 4 product-readiness review. They use
// the captured corpus with its production selection and only the metadata the
// snapshot recorded; nothing is fetched or injected.

func corpusInput(t *testing.T, file string) Input {
	t.Helper()
	s, err := corpus.Load(filepath.Join("..", "crawler", "testdata", "corpus", file))
	if err != nil {
		t.Fatal(err)
	}
	owner, name, _ := strings.Cut(s.Repo, "/")
	repo := githubclient.Repo{Owner: owner, Name: name, FullName: s.Repo, Language: s.Language, Metadata: map[string]githubclient.MetadataField{"full_name": {Value: s.Repo}, "language": {Value: s.Language}}}
	return Input{Repo: repo, Inspection: crawler.NewInspection(s.Language, crawler.Revision{Ref: s.ResolvedSHA, Resolved: true}, s.FileTree(), s.Fetched(s.Selected))}
}

// consumedLines returns lines a..b (one-based, inclusive) of a consumed file,
// computed directly from its bytes for an audit independent of the renderer.
func consumedLines(t *testing.T, in Input, ref, lines string) string {
	t.Helper()
	a, b, ok := strings.Cut(lines, "-")
	start, err1 := strconv.Atoi(a)
	end, err2 := strconv.Atoi(b)
	if !ok || err1 != nil || err2 != nil || start < 1 || end < start {
		t.Fatalf("%s: malformed line range %q", ref, lines)
	}
	for _, f := range in.Inspection.Files {
		if f.Path == ref {
			all := strings.Split(f.Content, "\n")
			if end > len(all) {
				t.Fatalf("%s: lines %s exceed the file's %d lines", ref, lines, len(all))
			}
			return strings.Join(all[start-1:end], "\n")
		}
	}
	t.Fatalf("%s was not consumed", ref)
	return ""
}

// R4: declarations in go.mod cite their exact lines and bytes.
func TestReviewGoDeclarationsCiteExactLocations(t *testing.T) {
	in := corpusInput(t, "pocketbase__pocketbase.json")
	checked := 0
	for _, c := range claimsIn(Build(in)) {
		if c.Source.Kind != apimodel.SourceFile || c.Source.Ref != "go.mod" || !strings.HasPrefix(c.Label, "Declares ") {
			continue
		}
		checked++
		module := strings.TrimPrefix(c.Label, "Declares ")
		if c.Source.Lines == "" || c.Excerpt == "" {
			t.Errorf("%s: whole-file citation (lines %q, excerpt %q)", module, c.Source.Lines, c.Excerpt)
			continue
		}
		text := consumedLines(t, in, c.Source.Ref, c.Source.Lines)
		if !strings.Contains(text, c.Excerpt) || !strings.Contains(c.Excerpt, module) {
			t.Errorf("%s: excerpt %q is not the declaration on lines %s (%q)", module, c.Excerpt, c.Source.Lines, text)
		}
		if !strings.HasSuffix(c.Source.URL, "#L"+strings.Replace(c.Source.Lines, "-", "-L", 1)) {
			t.Errorf("%s: URL %q lacks the line fragment for %s", module, c.Source.URL, c.Source.Lines)
		}
	}
	if checked == 0 {
		t.Fatal("no go.mod declaration claims were produced")
	}
}
