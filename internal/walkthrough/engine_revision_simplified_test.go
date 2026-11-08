package walkthrough

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestRevisionMayReplaceIDsScenesAndCorrectRetainedSourceSelection(t *testing.T) {
	inv, _, original, _ := engineInvestigation(t)
	for _, path := range []string{"README.md", "src/report.rs"} {
		if _, err := inv.Read(path); err != nil {
			t.Fatal(err)
		}
	}
	revised := original
	revised.Statements = append([]Statement(nil), original.Statements...)
	for i := range revised.Statements {
		revised.Statements[i].ID = "fresh-" + revised.Statements[i].ID
	}
	revised.Statements[2].Text = "Rows reach aggregate."
	original.Statements = append([]Statement(nil), original.Statements...)
	original.Statements[2].Sources = []Citation{original.Statements[0].Sources[0]}
	revised.Scenes = []Scene{{ID: "fresh-layout", Role: "workflow", StatementIDs: []string{"fresh-p", "fresh-a", "fresh-m", "fresh-r"}}}
	negative := Review{Accepted: false, Reason: "PRIVATE_REJECTION_FEEDBACK"}
	positive := Review{Accepted: true, Reason: "The fresh candidate is source supported."}
	for _, s := range original.Statements {
		negative.Decisions = append(negative.Decisions, Decision{StatementID: s.ID, Accepted: s.ID != "m", Reason: "The cited premise needs correction."})
	}
	for _, s := range revised.Statements {
		positive.Decisions = append(positive.Decisions, Decision{StatementID: s.ID, Accepted: true, Reason: "The retained source supports this revised wording."})
	}
	cfg := revisionConfig()
	calls := 0
	reads, used := len(inv.read), inv.used
	engine, err := NewEngine(cfg, engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		var request responseRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		switch calls {
		case 1:
			return engineHTTP(t, engineCall(t, "submit_story", original, "initial")), nil
		case 2:
			return engineHTTP(t, engineMessage(t, negative)), nil
		case 3:
			body := engineJSON(t, request.Input)
			if len(request.Tools) != 1 || !strings.Contains(body, "inspected_source_windows") || !strings.Contains(body, "acquired_source_registry") || !strings.Contains(body, "PRIVATE_REJECTION_FEEDBACK") {
				t.Fatal("correction omitted acquired context/full feedback or gained tools")
			}
			return engineHTTP(t, engineCall(t, "submit_story", revised, "correction")), nil
		case 4:
			if len(request.Input) != 1 || strings.Contains(engineJSON(t, request.Input), "PRIVATE_REJECTION_FEEDBACK") || strings.Contains(engineJSON(t, request.Input), "prior_candidate") {
				t.Fatal("fresh review inherited prior candidate or feedback")
			}
			return engineHTTP(t, engineMessage(t, positive)), nil
		default:
			t.Fatal("unexpected extra call")
			return nil, ErrProviderUnavailable
		}
	})}))
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := engine.Run(context.Background(), "fresh-retained-correction", inv, nil)
	if err != nil || accepted.ID() == "" || calls != 4 || !strings.Contains(accepted.Sections()[0].Body, revised.Statements[2].Text) || strings.Contains(accepted.Sections()[0].Body, original.Statements[2].Text) || len(inv.read) != reads || inv.used != used {
		t.Fatalf("fresh retained-source correction failed: %v calls=%d", err, calls)
	}
}

func TestRevisionRejectsUnknownOutOfRetainedAndOversizedFinalReview(t *testing.T) {
	for _, failure := range []string{"unknown", "out of retained", "final body cap"} {
		t.Run(failure, func(t *testing.T) {
			inv, g, original, review := engineInvestigation(t)
			if failure == "final body cap" {
				g.files["src/report.rs"] += strings.Repeat("x", 2200) + "\n"
			}
			for _, path := range []string{"README.md", "src/report.rs"} {
				if _, err := inv.Read(path); err != nil {
					t.Fatal(err)
				}
			}
			revised := original
			revised.Statements = append([]Statement(nil), original.Statements...)
			switch failure {
			case "unknown":
				revised.Statements[2].Sources = []Citation{{FileID: "not-acquired", StartLine: 1, EndLine: 1}}
			case "out of retained":
				revised.Statements[2].Sources = []Citation{{FileID: inv.sourceIDs["src/report.rs"], StartLine: 999, EndLine: 999}}
			case "final body cap":
				revised.Statements = nil
				ids := []string{}
				kinds := []string{"purpose", "action", "mechanism", "result", "design", "handoff"}
				for _, kind := range kinds {
					id := "large-" + kind
					ids = append(ids, id)
					statement := Statement{ID: id, Kind: kind, Text: "A bounded candidate."}
					for j := 0; j < 10; j++ {
						statement.Sources = append(statement.Sources, Citation{FileID: inv.sourceIDs["src/report.rs"], StartLine: 5, EndLine: 5})
					}
					revised.Statements = append(revised.Statements, statement)
				}
				revised.Scenes = []Scene{{ID: "large-scene", Role: "workflow", StatementIDs: ids}}
			}
			cfg := revisionConfig()
			ledger := engineLedger(t)
			calls := 0
			engine, err := NewEngine(cfg, ledger, WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(*http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return engineHTTP(t, engineCall(t, "submit_story", original, "initial")), nil
				}
				if calls == 2 {
					return engineHTTP(t, engineMessage(t, revisionRejected(review))), nil
				}
				if calls == 3 {
					return engineHTTP(t, engineCall(t, "submit_story", revised, "correction")), nil
				}
				t.Fatal("invalid final candidate paid for review")
				return nil, ErrProviderUnavailable
			})}))
			if err != nil {
				t.Fatal(err)
			}
			if failure == "final body cap" {
				grounded, err := groundDraft(inv.Snapshot(), revised)
				if err != nil {
					t.Fatal(err)
				}
				if err := ValidateDraft(inv.Snapshot(), grounded); err != nil {
					t.Fatal(err)
				}
				request, err := engine.reviewRequest(inv.Snapshot(), grounded, 2000)
				if err != nil {
					t.Fatal(err)
				}
				prepared, err := engine.prepareRequest(&engineBudget{}, request, true)
				if err != nil || len(prepared.body) <= 128<<10 {
					t.Fatal("final body cap fixture is not mechanically valid and oversized")
				}
			}
			accepted, err := engine.Run(context.Background(), "invalid-correction-"+strings.ReplaceAll(failure, " ", "-"), inv, nil)
			if err == nil || accepted.ID() != "" || calls != 3 {
				t.Fatalf("invalid correction was published or paid twice: %v calls=%d", err, calls)
			}
			if failure == "final body cap" && !errors.Is(err, ErrGenerationBudget) {
				t.Fatalf("oversized final HTTP body escaped explicit budget cap: %v", err)
			}
			if _, err := ledger.Reserve("unused-proof", "remaining", 1000000-6); err != nil {
				t.Fatalf("known-unexecuted review reservation retained after validation failure: %v", err)
			}
		})
	}
}
