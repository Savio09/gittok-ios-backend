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

func jsonFields(t *testing.T, value any) map[string]any {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(b, &fields); err != nil {
		t.Fatal(err)
	}
	return fields
}

// The new status must describe the statement itself, not the existence of a
// citation: a sourced role inference is still an inference.
func TestEvidenceSeparatesDeclarationsFromClassification(t *testing.T) {
	in := corpusInput(t, "cli__cli.json")
	p := Build(in)
	fields := jsonFields(t, p)
	if fields["kind_evidence_status"] != "inferred" || fields["evidence_mode"] != "not_executed" {
		t.Errorf("kind/runtime contract = %v / %v", fields["kind_evidence_status"], fields["evidence_mode"])
	}
	proof, ok := kindProof(p)
	if !ok {
		t.Fatal("fixture lost kind explanation")
	}
	if jsonFields(t, proof)["evidence_status"] != "inferred" || proof.Confidence != apimodel.Uncertain {
		t.Errorf("classification masquerades as a direct fact: %+v", proof)
	}
	direct := 0
	for _, section := range p.Sections {
		if section.EvidenceMode != apimodel.NotExecuted {
			t.Errorf("section runtime = %s", section.EvidenceMode)
		}
		for _, c := range section.Claims {
			if c.Label == proof.Label {
				continue
			}
			direct++
			if jsonFields(t, c)["evidence_status"] != "supported" || c.Confidence != apimodel.Confident {
				t.Errorf("scoped declaration/attribution not supported: %+v", c)
			}
			if err := ValidateClaim(in, c); err != nil {
				t.Fatal(err)
			}
		}
	}
	if direct == 0 {
		t.Fatal("fixture has no direct facts")
	}
}

func TestEvidenceSupportsAttributedStatementsAndMetrics(t *testing.T) {
	p := Build(metadataInput())
	var readme, metadata, command bool
	for _, c := range claimsIn(p) {
		readme = readme || c.Label == "README passage"
		metadata = metadata || c.Source.Kind == apimodel.SourceMetadata
		command = command || strings.HasPrefix(c.Label, "Declares command")
		if jsonFields(t, c)["evidence_status"] != "supported" || c.Limit == "" {
			t.Errorf("missing attribution status or scope limit: %+v", c)
		}
	}
	if !readme || !metadata || !command {
		t.Fatal("fixture lost README, metadata or command evidence")
	}
	if len(p.Metrics) != 3 {
		t.Fatal("fixture lost reported metrics")
	}
	for _, m := range p.Metrics {
		if jsonFields(t, m)["evidence_status"] != "supported" || m.Confidence != apimodel.Confident {
			t.Errorf("reported metric not supported at metadata scope: %+v", m)
		}
	}
}

func TestEvidenceUnknownKindRetainsPartialFactsAndCoverage(t *testing.T) {
	for name, in := range map[string]Input{
		"unresolved":              inputForTest(),
		"missing tree provenance": rustCLIInput(""),
		"metadata only":           {Repo: metadataInput().Repo, Coverage: Coverage{FilesNotAcquired: true}},
		"unavailable contents":    {Repo: inputForTest().Repo, Inspection: crawler.Inspection{Revision: crawler.Revision{Ref: testSHA, Resolved: true}, Unavailable: []crawler.Unavailable{{Path: "README.md", Reason: "unavailable"}}}},
		"invalid identity":        {},
	} {
		t.Run(name, func(t *testing.T) {
			p := Build(in)
			fields := jsonFields(t, p)
			if p.Kind != apimodel.KindOther || fields["kind_evidence_status"] != "unknown" || fields["evidence_mode"] != "not_executed" {
				t.Errorf("undecided output = %+v", p)
			}
			for _, c := range claimsIn(p) {
				if jsonFields(t, c)["evidence_status"] != "supported" {
					t.Errorf("invented unknown claim: %+v", c)
				}
			}
			if name != "invalid identity" {
				if body, ok := coverageBody(p); !ok || body == "" {
					t.Fatal("missing coverage note")
				}
			}
			if name == "unresolved" && len(claimsIn(p)) == 0 {
				t.Fatal("discarded valid partial facts")
			}
		})
	}
}

func TestEvidenceMetadataFeedNeverCallsHeuristicsSupported(t *testing.T) {
	for name, repo := range map[string]githubclient.Repo{
		"language heuristic": {Language: "Go"},
		"topic heuristic":    {Topics: []string{"cli"}},
		"unknown":            {},
	} {
		t.Run(name, func(t *testing.T) {
			repo.Owner, repo.Name, repo.FullName = "o", "r", "o/r"
			repo.PushedAt = time.Now().Add(-72 * time.Hour)
			repo.License = "MIT"
			repo.Metadata = map[string]githubclient.MetadataField{
				"stargazers_count": {Value: "0"},
				"license.spdx_id":  {Value: "MIT"},
				"pushed_at":        {Value: repo.PushedAt.Format(time.RFC3339)},
			}
			p := FromMetadata(repo)
			fields := jsonFields(t, p)
			want := "inferred"
			if name == "unknown" {
				want = "unknown"
			}
			if fields["kind_evidence_status"] != want || fields["evidence_mode"] != "not_executed" {
				t.Errorf("metadata feed overstates kind/runtime: %+v", fields)
			}
			if len(p.Metrics) != 3 {
				t.Fatalf("fixture lost reported metrics: %+v", p.Metrics)
			}
			for _, m := range p.Metrics {
				if jsonFields(t, m)["evidence_status"] != "supported" {
					t.Errorf("metric status = %+v", m)
				}
				if m.Source.Ref == "pushed_at" && (!strings.Contains(m.Label, "push") || strings.Contains(m.Label, "commit")) {
					t.Errorf("push time mislabelled: %+v", m)
				}
			}
		})
	}
}

func TestEvidenceStatusParticipatesInValidationAndDeduplication(t *testing.T) {
	in := corpusInput(t, "cli__cli.json")
	original, ok := kindProof(Build(in))
	if !ok {
		t.Fatal("fixture lost kind proof")
	}
	if err := ValidateClaim(in, original); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"supported", "unknown", "", "future"} {
		fields := jsonFields(t, original)
		fields["evidence_status"] = status
		b, _ := json.Marshal(fields)
		var forged apimodel.Claim
		if err := json.Unmarshal(b, &forged); err != nil {
			t.Fatal(err)
		}
		if ValidateClaim(in, forged) == nil {
			t.Errorf("accepted forged evidence status %q", status)
		}
		if keyOf(original) == keyOf(forged) {
			t.Errorf("dedup ignores evidence status %q", status)
		}
	}
}

// Provider zero values are ambiguous without their retained presence metadata.
// A reported zero is supported; an absent, null or inconsistent value is not.
func TestEvidenceMetadataMetricsRequireMatchingProviderFields(t *testing.T) {
	cases := []struct {
		name, field, key string
		mutate           func(*githubclient.Repo)
		present          bool
	}{
		{"stars missing defaults to zero", "stargazers_count", "stars", func(r *githubclient.Repo) { r.Stars = 0; delete(r.Metadata, "stargazers_count") }, false},
		{"stars null defaults to zero", "stargazers_count", "stars", func(r *githubclient.Repo) {
			r.Stars = 0
			r.Metadata["stargazers_count"] = githubclient.MetadataField{Value: "0", Null: true}
		}, false},
		{"stars mismatch", "stargazers_count", "stars", func(r *githubclient.Repo) { r.Metadata["stargazers_count"] = githubclient.MetadataField{Value: "17"} }, false},
		{"stars reported zero", "stargazers_count", "stars", func(r *githubclient.Repo) {
			r.Stars = 0
			r.Metadata["stargazers_count"] = githubclient.MetadataField{Value: "0"}
		}, true},
		{"stars reported count", "stargazers_count", "stars", func(r *githubclient.Repo) {}, true},
		{"license missing", "license.spdx_id", "license", func(r *githubclient.Repo) { delete(r.Metadata, "license.spdx_id") }, false},
		{"license null", "license.spdx_id", "license", func(r *githubclient.Repo) {
			r.Metadata["license.spdx_id"] = githubclient.MetadataField{Value: r.License, Null: true}
		}, false},
		{"license mismatch", "license.spdx_id", "license", func(r *githubclient.Repo) {
			r.Metadata["license.spdx_id"] = githubclient.MetadataField{Value: "Apache-2.0"}
		}, false},
		{"license no assertion", "license.spdx_id", "license", func(r *githubclient.Repo) {
			r.License = "NOASSERTION"
			r.Metadata["license.spdx_id"] = githubclient.MetadataField{Value: "NOASSERTION"}
		}, false},
		{"license reported", "license.spdx_id", "license", func(r *githubclient.Repo) {}, true},
		{"push missing", "pushed_at", "last_commit", func(r *githubclient.Repo) { delete(r.Metadata, "pushed_at") }, false},
		{"push null", "pushed_at", "last_commit", func(r *githubclient.Repo) {
			r.Metadata["pushed_at"] = githubclient.MetadataField{Value: r.PushedAt.Format(time.RFC3339), Null: true}
		}, false},
		{"push mismatch", "pushed_at", "last_commit", func(r *githubclient.Repo) {
			r.Metadata["pushed_at"] = githubclient.MetadataField{Value: "2020-01-01T00:00:00Z"}
		}, false},
		{"push reported", "pushed_at", "last_commit", func(r *githubclient.Repo) {}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := metadataInput().Repo
			tc.mutate(&r)
			found := false
			for _, m := range FromMetadata(r).Metrics {
				if m.Key != tc.key {
					continue
				}
				found = true
				if m.Source == nil || m.Source.Ref != tc.field || m.EvidenceStatus != apimodel.EvidenceSupported || m.Confidence != apimodel.Confident {
					t.Errorf("reported metric lacks scope/status: %+v", m)
				}
			}
			if found != tc.present {
				t.Errorf("metric %s present = %v, want %v", tc.key, found, tc.present)
			}
		})
	}
	if p := FromMetadata(githubclient.Repo{Owner: "o", Name: "r", FullName: "o/r"}); len(p.Metrics) != 0 {
		t.Errorf("missing provider fields manufactured metrics: %+v", p.Metrics)
	}
}
