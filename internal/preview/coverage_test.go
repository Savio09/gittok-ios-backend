package preview

import (
	"regexp"
	"strings"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/crawler"
)

func coverageBody(p apimodel.RepoPreview) (string, bool) {
	for _, s := range p.Sections {
		if s.Kind == apimodel.SectionDecision {
			return s.Body, true
		}
	}
	return "", false
}

// Reader-facing notes are short and plain: no paths, parser details or
// classifier vocabulary. A path-like token is a word containing '/' or a
// file-name extension.
var technical = regexp.MustCompile(`(?i)\S+/\S*|\b\S+\.(go|mod|json|toml|ya?ml|md|rs|py)\b|witness|premise|component|decisive|listing|manifest|package clause|entry point`)

func assertReadable(t *testing.T, body string) {
	t.Helper()
	notes := strings.Split(body, "\n")
	if len(notes) > 3 || len(body) > 500 {
		t.Errorf("coverage notes are not concise (%d notes, %d bytes): %q", len(notes), len(body), body)
	}
	if m := technical.FindString(body); m != "" {
		t.Errorf("technical detail %q in reader-facing notes: %q", m, body)
	}
}

// R5: the CLI preview's notes are short, while the detailed causes remain in
// the internal classification result.
func TestReviewCoverageNotesAreShortAndPlain(t *testing.T) {
	in := corpusInput(t, "cli__cli.json")
	body, _ := coverageBody(Build(in))
	assertReadable(t, body)
	if len(crawler.Classify(in.Inspection).Limitations) == 0 {
		t.Error("internal diagnostics were discarded rather than kept out of the preview")
	}
}

func TestCoverageNotesFollowTypedState(t *testing.T) {
	const (
		identity = "could not reconfirm this repository's identity"
		metaOnly = "GitHub metadata only"
		unread   = "could not be read in full"
		partTree = "part of a large repository"
		kind     = "project type could not be confirmed"
	)
	for name, tc := range map[string]struct {
		mutate func(*Input)
		want   []string
		absent []string
	}{
		"complete, explained": {func(in *Input) {}, nil, []string{unread, partTree, metaOnly}},
		"identity withheld": {func(in *Input) {
			in.Coverage.IdentityUnverified = true
			in.Inspection.Files, in.Inspection.Revision = nil, crawler.Revision{}
		}, []string{identity}, []string{metaOnly}},
		"files not acquired": {func(in *Input) {
			in.Coverage.FilesNotAcquired = true
			in.Inspection.Files = nil
		}, []string{metaOnly}, nil},
		"unavailable file": {func(in *Input) {
			in.Inspection.Unavailable = []crawler.Unavailable{{Path: "src/x.go", Reason: "fetch error"}}
		}, []string{unread}, nil},
		"clipped file":        {func(in *Input) { in.Inspection.Files[0].Truncated = true }, []string{unread}, nil},
		"incomplete contents": {func(in *Input) { in.Coverage.ContentsIncomplete = true }, []string{unread}, nil},
		"truncated tree":      {func(in *Input) { in.Inspection.Scope.Truncated = true }, []string{partTree}, nil},
		"every gap at once": {func(in *Input) {
			in.Coverage = Coverage{IdentityUnverified: true, ContentsIncomplete: true, SymlinksExcluded: true}
			in.Inspection.Scope.DepthLimited = true
			in.Inspection.Unavailable = []crawler.Unavailable{{Path: "a", Reason: "b"}}
		}, []string{identity, unread, partTree}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			in := rustCLIInput(githubTreeFor("o", "r"))
			tc.mutate(&in)
			body, present := coverageBody(Build(in))
			if len(tc.want) == 0 && present {
				t.Errorf("unexpected notes: %q", body)
			}
			for _, w := range tc.want {
				if !strings.Contains(body, w) {
					t.Errorf("missing %q in %q", w, body)
				}
			}
			for _, a := range tc.absent {
				if strings.Contains(body, a) {
					t.Errorf("unexpected %q in %q", a, body)
				}
			}
			if present {
				assertReadable(t, body)
			}
		})
	}
}

func TestAbstentionIsExplainedWithoutClaimingAbsence(t *testing.T) {
	in := rustCLIInput("") // without tree provenance the CLI kind is withheld
	p := Build(in)
	body, _ := coverageBody(p)
	if p.Kind != apimodel.KindOther || !strings.Contains(body, "project type could not be confirmed") {
		t.Fatalf("withheld kind not explained: %s %q", p.Kind, body)
	}
	if strings.Contains(strings.ToLower(body), "no cli") || strings.Contains(strings.ToLower(body), "is not a") {
		t.Errorf("an abstention was worded as an absence: %q", body)
	}
}

// Whole-file citations (formats without a locator) are not a coverage gap.
func TestWholeFileCitationsAreNotReportedAsGaps(t *testing.T) {
	p := Build(rustCLIInput(githubTreeFor("o", "r")))
	if body, present := coverageBody(p); present {
		t.Errorf("a missing line range was reported as a gap: %q", body)
	}
}
