package apimodel

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// sampleFeed is a representative feed page used both as a round-trip fixture and
// to generate the golden JSON that documents the contract for the app. It is a
// frontend repo the reader can read (TypeScript/React) with a Go stretch, an
// observed-running demo, and one honestly uncertain claim.
func sampleFeed() Feed {
	return Feed{
		Items: []RepoPreview{
			{
				ID:          "orbit",
				Owner:       "acme",
				Name:        "orbit",
				Slug:        "acme/orbit",
				Kind:        KindFrontend,
				Language:    "TypeScript",
				Headline:    "Your work. One clear view.",
				Description: "A productivity dashboard that pulls tasks from several tools into a single web view.",
				GitHubURL:   "https://github.com/acme/orbit",
				License:     "MIT",
				Media: &Media{
					Kind:         "narrated_video",
					EvidenceMode: ObservedRunning,
					Caption:      "The dashboard loading a live board.",
					URL:          "https://cdn.example.com/previews/orbit.mp4",
				},
				Signals: []Signal{
					{Value: "TypeScript", Reason: ReasonYouKnow},
					{Value: "Go", Reason: ReasonYouWantToLearn},
					{Value: "Developer tools", Reason: ReasonDomainMatch},
				},
				Metrics: []Metric{
					{Key: "last_commit", Label: "since last commit", Value: "3d",
						Source: &Source{Kind: SourceMetadata, Ref: "pushed_at"}, Confidence: Confident},
					{Key: "good_first_issues", Label: "good first issues", Value: "12",
						Source: &Source{Kind: SourceAPI, Ref: "GET /repos/acme/orbit/issues?labels=good+first+issue"}, Confidence: Confident},
					{Key: "contributing_guide", Label: "contributing guide", Value: "Yes",
						Source: &Source{Kind: SourceFile, Ref: "CONTRIBUTING.md"}, Confidence: Confident},
				},
				Fit: &Fit{
					Summary: "TypeScript you know, Go you want",
					Dimensions: []FitDimension{
						{Name: "Matches your skills", Level: 3, Word: "Strong"},
						{Name: "Teaches you", Level: 2, Word: "Go"},
						{Name: "Setup effort", Level: 2, Word: "Medium"},
						{Name: "Way in for newcomers", Level: 3, Word: "Good"},
					},
				},
				Sections: []Section{
					{
						Kind:  SectionIdea,
						Title: "What it does",
						Body:  "Orbit is a dashboard that unifies tasks from issue trackers into one web view.",
					},
					{
						Kind:         SectionHow,
						Title:        "How it works",
						EvidenceMode: NotExecuted,
						Body:         "A React app talks to a Go API that stores items in PostgreSQL.",
						Claims: []Claim{
							{
								Label:       "Frontend is TypeScript and React",
								Source:      Source{Kind: SourceFile, Ref: "apps/web/package.json", Lines: "12-18"},
								Excerpt:     "\"react\": \"^18.2\",\n\"next\": \"^14.0\"",
								Explanation: "The web app lists React and Next.js as dependencies.",
								Supports:    "the stack as written",
								Limit:       "does not prove runtime behavior",
								Confidence:  Confident,
								Stance:      Match,
							},
							{
								Label:       "API is Go",
								Source:      Source{Kind: SourceFile, Ref: "go.mod", Lines: "1-5"},
								Excerpt:     "module github.com/acme/orbit",
								Explanation: "The server module is written in Go.",
								Supports:    "the API language",
								Limit:       "does not prove the API runs",
								Confidence:  Confident,
								Stance:      Stretch,
							},
						},
					},
					{
						Kind:  SectionDecision,
						Title: "Your call",
						Claims: []Claim{
							{
								Label:       "No tests found for the API",
								Source:      Source{Kind: SourceSearch, Ref: "glob: server/**/*_test.go"},
								Explanation: "A search for Go test files under the server turned up nothing.",
								Limit:       "absence of test files is not proof there are no tests",
								Confidence:  Uncertain,
								Stance:      Unknown,
							},
						},
					},
				},
			},
		},
		NextCursor: "eyJvZmZzZXQiOjF9",
	}
}

const goldenPath = "testdata/feed_example.json"

// TestGoldenFeed marshals the sample feed and checks it against the committed
// golden file. Run `UPDATE_GOLDEN=1 go test ./internal/apimodel` to refresh the
// golden after an intentional contract change.
func TestGoldenFeed(t *testing.T) {
	got, err := json.MarshalIndent(sampleFeed(), "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got = append(got, '\n')

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("updated %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run UPDATE_GOLDEN=1 go test to create it): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("serialized feed does not match %s.\n--- got ---\n%s", goldenPath, got)
	}
}

// TestRoundTrip proves the golden JSON unmarshals and re-marshals byte-for-byte,
// so the contract is stable in both directions.
func TestRoundTrip(t *testing.T) {
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Skipf("no golden yet: %v", err)
	}
	var f Feed
	if err := json.Unmarshal(want, &f); err != nil {
		t.Fatalf("unmarshal golden: %v", err)
	}
	got, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got = append(got, '\n')
	if !bytes.Equal(got, want) {
		t.Errorf("round-trip changed the JSON.\n--- got ---\n%s", got)
	}
}

func TestEnumValidity(t *testing.T) {
	valid := []interface{ Valid() bool }{
		Confident, Uncertain,
		Match, Stretch, Caution, Unknown, Stance(""),
		SourceFile, SourceSearch, SourceMetadata, SourceAPI, SourceKind(""),
		KindFrontend, KindBackend, KindLibrary, KindCLI, KindMobile, KindOther,
		ObservedRunning, NotExecuted, EvidenceMode(""),
		ReasonYouKnow, ReasonYouWantToLearn, ReasonDomainMatch, ReasonPopular,
		SectionIdea, SectionHow, SectionStack, SectionActivity, SectionSetup, SectionDecision,
		MixAMix, LearnMix(""), ExpSmallPRs, Experience(""), Time2to5h, TimePerWeek(""),
		ActionSave, ActionSkip, ActionInspect,
	}
	for _, v := range valid {
		if !v.Valid() {
			t.Errorf("%#v.Valid() = false, want true", v)
		}
	}

	invalid := []interface{ Valid() bool }{
		Confidence("maybe"), Stance("meh"), SourceKind("blob"), RepoKind(""),
		RepoKind("webapp"), EvidenceMode("ran_it"), SignalReason("vibes"),
		SectionKind("intro"), LearnMix("some"), Experience("pro"),
		TimePerWeek("lots"), EventAction("open"),
	}
	for _, v := range invalid {
		if v.Valid() {
			t.Errorf("%#v.Valid() = true, want false", v)
		}
	}
}
