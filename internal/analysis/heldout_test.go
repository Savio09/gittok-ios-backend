package analysis

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/crawler"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
	"github.com/Savio09/gittok-ios-backend/internal/preview"
)

// Evaluation inputs only. Neither service nor builder refers to these slugs.
var heldoutRepos = []string{"charmbracelet/glow", "rack/rack"}

type heldoutCapture struct {
	Repo        githubclient.Repo            `json:"metadata"`
	CapturedAt  time.Time                    `json:"captured_at"`
	Revision    crawler.Revision             `json:"revision"`
	Tree        githubclient.FileTree        `json:"tree"`
	Requested   []string                     `json:"selected"`
	Fetched     githubclient.FetchedContents `json:"consumed"`
	Limitations []string                     `json:"limitations,omitempty"`
}

func (c heldoutCapture) input() preview.Input {
	return preview.Input{Repo: c.Repo, CapturedAt: c.CapturedAt, Inspection: crawler.NewInspection(c.Repo.Language, c.Revision, c.Tree, c.Fetched), Limitations: c.Limitations}
}

func TestCaptureG17Heldout(t *testing.T) {
	if os.Getenv("GITTOK_CAPTURE_G17") != "1" {
		t.Skip("opt-in network capture: GITTOK_CAPTURE_G17=1 go test ./internal/analysis -run TestCaptureG17Heldout -count=1")
	}
	dir := filepath.Join("testdata", "g17")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, slug := range heldoutRepos {
		owner, name, _ := strings.Cut(slug, "/")
		result, err := NewPublicService(githubclient.WithMaxRetries(0), githubclient.WithUserAgent("gittok-g17-acceptance")).Acquire(context.Background(), owner, name)
		if err != nil {
			t.Fatalf("%s: %v", slug, err)
		}
		if !result.Inspection.Revision.Resolved || len(result.Inspection.Files) == 0 {
			t.Fatalf("%s: capture incomplete: %v", slug, result.Limitations)
		}
		c := heldoutCapture{Repo: result.Repo, CapturedAt: result.CapturedAt, Revision: result.Inspection.Revision, Tree: result.Tree, Requested: result.Requested, Fetched: result.Fetched, Limitations: result.Limitations}
		p := preview.Build(c.input())
		auditPreview(t, c, p)
		stem := strings.ReplaceAll(slug, "/", "__")
		for suffix, value := range map[string]any{".json": c, ".preview.json": p} {
			b, err := json.MarshalIndent(value, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, stem+suffix), append(b, '\n'), 0644); err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("%s at %s: %d selected, %d consumed, %d claims, kind %s", slug, c.Revision.Ref, len(c.Requested), len(c.Fetched.Files), claimCount(p), p.Kind)
	}
}

func TestHeldoutPreviewsAreGrounded(t *testing.T) {
	for _, slug := range heldoutRepos {
		t.Run(slug, func(t *testing.T) {
			stem := filepath.Join("testdata", "g17", strings.ReplaceAll(slug, "/", "__"))
			var c heldoutCapture
			readJSON(t, stem+".json", &c)
			var expected apimodel.RepoPreview
			readJSON(t, stem+".preview.json", &expected)
			if c.CapturedAt.IsZero() || !c.Repo.Public || !c.Revision.Resolved || !githubclient.ValidCommitID(c.Revision.Ref) {
				t.Fatal("capture lacks identity/visibility/time/revision provenance")
			}
			// This golden output belongs to its recorded capture selection.
			// The current selector has a separate grounded replay below.
			paths := c.Requested

			allowed := map[string]bool{}
			for _, p := range paths {
				allowed[p] = true
			}
			for _, f := range c.Fetched.Files {
				if !allowed[f.Path] {
					t.Fatalf("manual extra file %s", f.Path)
				}
			}
			got := preview.Build(c.input())
			if os.Getenv("GITTOK_UPDATE_G17_PREVIEWS") == "1" {
				b, err := json.MarshalIndent(got, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(stem+".preview.json", append(b, '\n'), 0644); err != nil {
					t.Fatal(err)
				}
				expected = got
			}
			if !reflect.DeepEqual(expected, got) {
				t.Fatal("preview changed; inspect source support before updating golden output")
			}
			auditPreview(t, c, got)
			// Identity is not a rule selector: renaming changes only source/identity
			// metadata, never file fact count or role resolution.
			renamed := c.input()
			renamed.Repo.Owner = "renamed"
			renamed.Repo.Name = "project"
			renamed.Repo.FullName = "renamed/project"
			renamed.Repo.Metadata = make(map[string]githubclient.MetadataField)
			for key, value := range c.Repo.Metadata {
				renamed.Repo.Metadata[key] = value
			}
			renamed.Repo.Metadata["full_name"] = githubclient.MetadataField{Value: "renamed/project"}
			other := preview.Build(renamed)
			if other.Kind != got.Kind || claimCount(other) != claimCount(got) {
				t.Fatal("repository identity changed analysis")
			}
		})
	}
}

func readJSON(t *testing.T, path string, target any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, target); err != nil {
		t.Fatal(err)
	}
}
func claimCount(p apimodel.RepoPreview) int {
	n := 0
	for _, s := range p.Sections {
		n += len(s.Claims)
	}
	return n
}

func auditPreview(t *testing.T, c heldoutCapture, p apimodel.RepoPreview) {
	t.Helper()
	if claimCount(p) == 0 {
		t.Fatal("vacuous preview with no claims")
	}
	consumed := map[string]string{}
	for _, f := range c.Fetched.Files {
		consumed[f.Path] = f.Content
	}
	for _, s := range p.Sections {
		if s.EvidenceMode != apimodel.NotExecuted {
			t.Fatal("implied target execution")
		}
		for _, claim := range s.Claims {
			if err := preview.ValidateClaim(c.input(), claim); err != nil {
				t.Fatal(err)
			}
			switch claim.Source.Kind {
			case apimodel.SourceFile:
				content, ok := consumed[claim.Source.Ref]
				if !ok {
					t.Fatalf("unread file %s", claim.Source.Ref)
				}
				if claim.Excerpt != "" && !strings.Contains(content, claim.Excerpt) {
					t.Fatalf("fabricated quote %q", claim.Excerpt)
				}
				if !strings.Contains(claim.Source.URL, "/blob/"+c.Revision.Ref+"/") {
					t.Fatal("unversioned file source")
				}
			case apimodel.SourceMetadata:
				field, ok := c.Repo.Metadata[claim.Source.Ref]
				if !ok || field.Null {
					t.Fatalf("absent metadata %s", claim.Source.Ref)
				}
			default:
				t.Fatalf("unexpected source kind %s", claim.Source.Kind)
			}
		}
	}
}

// Replaying a changed selector never gives it historical extras for free.
// Only requested, actually captured bytes can support the current preview.
func TestHeldoutCurrentSelectionGrounded(t *testing.T) {
	for _, slug := range heldoutRepos {
		t.Run(slug, func(t *testing.T) {
			var c heldoutCapture
			readJSON(t, filepath.Join("testdata", "g17", strings.ReplaceAll(slug, "/", "__")+".json"), &c)
			available := map[string]githubclient.FileContent{}
			for _, f := range c.Fetched.Files {
				available[f.Path] = f
			}
			c.Requested = nil
			c.Fetched = githubclient.FetchedContents{}
			for _, selected := range crawler.SelectFiles(c.Tree.Entries, crawler.DefaultSelectionBudget()) {
				c.Requested = append(c.Requested, selected.Path)
				if f, captured := available[selected.Path]; captured {
					c.Fetched.Files = append(c.Fetched.Files, f)
				} else {
					c.Fetched.Skipped = append(c.Fetched.Skipped, githubclient.SkippedFile{Path: selected.Path, Reason: "not captured by historical fixture"})
				}
			}
			p := preview.Build(c.input())
			auditPreview(t, c, p)
			fileClaims := 0
			for _, section := range p.Sections {
				for _, claim := range section.Claims {
					if claim.Source.Kind == apimodel.SourceFile {
						fileClaims++
					}
				}
			}
			if fileClaims == 0 {
				t.Fatal("current replay lost all captured file evidence")
			}
			t.Logf("%d selected, %d captured, %d unavailable, %d file claims", len(c.Requested), len(c.Fetched.Files), len(c.Fetched.Skipped), fileClaims)
		})
	}
}
