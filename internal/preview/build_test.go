package preview

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/crawler"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

const testSHA = "0123456789abcdef0123456789abcdef01234567"

func inputForTest() Input {
	insp := crawler.InspectionFromSources("Ruby", []string{"README.md", "web/package.json"}, map[string]string{"README.md": "# Project\n\nAn example with evidence.\n", "web/package.json": `{"devDependencies":{"react":"^19"},"scripts":{"build":"vite build"}}`})
	insp.Revision = crawler.Revision{Ref: testSHA, Resolved: true}
	return Input{Repo: githubclient.Repo{Owner: "any", Name: "repo", FullName: "any/repo", Description: "Author purpose", Metadata: map[string]githubclient.MetadataField{"full_name": {Value: "any/repo"}, "description": {Value: "Author purpose"}}}, Inspection: insp, CapturedAt: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)}
}
func claimsIn(p apimodel.RepoPreview) []apimodel.Claim {
	var out []apimodel.Claim
	for _, s := range p.Sections {
		out = append(out, s.Claims...)
	}
	return out
}

func TestBuildGroundedPreview(t *testing.T) {
	in := inputForTest()
	p := Build(in)
	if p.Kind != apimodel.KindOther {
		t.Fatalf("promoted metadata language/undeployed dependencies to a role: %s", p.Kind)
	}
	claims := claimsIn(p)
	if len(claims) < 4 {
		t.Fatalf("missing independent facts: %+v", p)
	}
	var readme, dep bool
	for _, c := range claims {
		if c.Source.Kind == apimodel.SourceFile {
			if !strings.Contains(c.Source.URL, "/blob/"+testSHA+"/") {
				t.Fatalf("mutable source: %+v", c.Source)
			}
			switch c.Source.Ref {
			case "README.md":
				readme = true
				if c.Excerpt != "An example with evidence." || c.Source.Lines != "3-3" {
					t.Fatalf("inexact quote: %+v", c)
				}
			case "web/package.json":
				if strings.Contains(c.Label, "react") {
					dep = true
					if c.Excerpt != "" || c.Source.Lines != "" || !strings.Contains(c.Explanation, "devDependencies") {
						t.Fatalf("invented location or category: %+v", c)
					}
				}
			default:
				t.Fatalf("unread source: %+v", c.Source)
			}
		}
		if err := ValidateClaim(in, c); err != nil {
			t.Fatal(err)
		}
	}
	if !readme || !dep {
		t.Fatal("missing README or component dependency")
	}
	if p.Media != nil || p.Fit != nil || len(p.Metrics) != 0 {
		t.Fatal("invented runtime/fit/missing-zero metrics")
	}
	for _, s := range p.Sections {
		if s.EvidenceMode != apimodel.NotExecuted {
			t.Fatal("runtime implication")
		}
	}
}

func TestValidateClaimRejectsRealSourceWithUnsupportedWords(t *testing.T) {
	in := inputForTest()
	c := claimsIn(Build(in))[0]
	for _, mutate := range []func(*apimodel.Claim){func(c *apimodel.Claim) { c.Explanation = "Setup has been run successfully" }, func(c *apimodel.Claim) { c.Source.URL = "https://evil.example/" }, func(c *apimodel.Claim) { c.Source.Ref = "absent" }, func(c *apimodel.Claim) { c.Excerpt = "fabricated" }} {
		forged := c
		mutate(&forged)
		if ValidateClaim(in, forged) == nil {
			t.Fatalf("accepted unsupported claim %+v", forged)
		}
	}
}

func TestBuildNoPinnedRevisionCannotEmitFileClaims(t *testing.T) {
	in := inputForTest()
	in.Inspection.Revision = crawler.Revision{Ref: "main"}
	for _, c := range claimsIn(Build(in)) {
		if c.Source.Kind == apimodel.SourceFile {
			t.Fatal("cited unpinned file")
		}
	}
}

func TestBuildExcerptsSurviveJSONRoundTrip(t *testing.T) {
	for _, content := range []string{"# x\n\nValid Unicode: 雪\n", "# x\n\nInvalid: \xff\n", "# x\n\nClipped: \xe9\x9b"} {
		in := inputForTest()
		in.Inspection.Files = []crawler.SourceFile{{Path: "README.md", Content: content, Truncated: true}}
		p := Build(in)
		b, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		var decoded apimodel.RepoPreview
		if err := json.Unmarshal(b, &decoded); err != nil {
			t.Fatal(err)
		}
		for _, c := range claimsIn(decoded) {
			if c.Source.Kind == apimodel.SourceFile && c.Excerpt != "" && !strings.Contains(content, c.Excerpt) {
				t.Fatalf("quote changed during serialization: %q", c.Excerpt)
			}
		}
	}
}

func TestBuildMissingMetadataDoesNotInventClaims(t *testing.T) {
	p := Build(Input{Repo: githubclient.Repo{Owner: "o", Name: "r", FullName: "o/r"}})
	if len(claimsIn(p)) != 0 || len(p.Metrics) != 0 {
		t.Fatalf("made up missing provider values: %+v", p)
	}
}
