package walkthrough

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func revisionRejected(review Review) Review {
	review.Accepted = false
	review.Reason = "PRIVATE_FIRST_REVIEW_FEEDBACK"
	review.Decisions = append([]Decision(nil), review.Decisions...)
	review.Decisions[2].Accepted = false
	review.Decisions[2].Reason = "Narrow the mechanism to its actual source-supported operation."
	return review
}

func TestEngineOneRevisionPublishesOnlyFreshlyReviewedCandidate(t *testing.T) {
	for _, scenario := range []string{"accepted", "second rejected", "duplicate scene reference"} {
		t.Run(scenario, func(t *testing.T) {
			inv, _, draft, review := engineInvestigation(t)
			for _, p := range []string{"README.md", "src/report.rs"} {
				if _, err := inv.Read(p); err != nil {
					t.Fatal(err)
				}
			}
			cfg := revisionConfig()
			revised := draft
			revised.Statements = append([]Statement(nil), draft.Statements...)
			revised.Scenes = append([]Scene(nil), draft.Scenes...)
			revised.Statements[2].Text = "The rows are passed to aggregate."
			switch scenario {
			case "duplicate scene reference":
				revised.Scenes[1].StatementIDs = []string{"a", "m", "m", "r"}
			}
			ledger := engineLedger(t)
			calls := 0
			engine, err := NewEngine(cfg, ledger, WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				var wire map[string]any
				if err := json.NewDecoder(r.Body).Decode(&wire); err != nil {
					t.Fatal(err)
				}
				switch calls {
				case 1:
					return engineHTTP(t, engineCall(t, "submit_story", draft, "initial-submit")), nil
				case 2:
					return engineHTTP(t, engineMessage(t, revisionRejected(review))), nil
				case 3:
					choice, _ := wire["tool_choice"].(map[string]any)
					if choice["name"] != "submit_story" || len(wire["tools"].([]any)) != 1 || !strings.Contains(engineJSON(t, wire["input"]), "PRIVATE_FIRST_REVIEW_FEEDBACK") {
						t.Fatal("revision omitted complete rejection feedback or retained source tools")
					}
					return engineHTTP(t, engineCall(t, "submit_story", revised, "revision-submit")), nil
				case 4:
					if len(wire["input"].([]any)) != 1 || strings.Contains(engineJSON(t, wire), "PRIVATE_FIRST_REVIEW_FEEDBACK") || wire["tools"] != nil {
						t.Fatal("second reviewer inherited first reviewer or writer conversation")
					}
					if scenario == "second rejected" {
						return engineHTTP(t, engineMessage(t, revisionRejected(review))), nil
					}
					return engineHTTP(t, engineMessage(t, review)), nil
				default:
					t.Fatal("more than one revision was attempted")
					return nil, errors.New("unexpected retry")
				}
			})}))
			if err != nil {
				t.Fatal(err)
			}
			accepted, err := engine.Run(context.Background(), "revision-"+strings.ReplaceAll(scenario, " ", "-"), inv, nil)
			if scenario == "accepted" {
				if err != nil || accepted.ID() == "" || calls != 4 || !strings.Contains(accepted.Sections()[1].Body, revised.Statements[2].Text) || strings.Contains(accepted.Sections()[1].Body, draft.Statements[2].Text) {
					t.Fatalf("latest candidate not exclusively published: %v calls=%d", err, calls)
				}
			} else {
				want := 3
				if scenario == "second rejected" {
					want = 4
				}
				if err == nil || accepted.ID() != "" || calls != want {
					t.Fatalf("revision escaped ordinary validation or repeated: %v calls=%d", err, calls)
				}
				if calls == 3 {
					if _, err := ledger.Reserve("unused-review-proof", "remaining", 1000000-6); err != nil {
						t.Fatalf("known-unexecuted second review was not refunded: %v", err)
					}
				}
			}
		})
	}
}

func TestEngineRevisionNeverRetriesMalformedTransportOrUnfundedPair(t *testing.T) {
	for _, failure := range []string{"transport", "malformed", "input budget", "cost budget", "global budget"} {
		t.Run(failure, func(t *testing.T) {
			inv, _, draft, review := engineInvestigation(t)
			for _, p := range []string{"README.md", "src/report.rs"} {
				if _, err := inv.Read(p); err != nil {
					t.Fatal(err)
				}
			}
			cfg := revisionConfig()
			if failure == "input budget" {
				cfg.MaxInputBytes = 26000
			}
			if failure == "cost budget" {
				cfg.MaxJobMicros = 60
			}
			ledger := engineLedger(t)
			if failure == "global budget" {
				if _, err := ledger.Reserve("other-job", "hold", 999950); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			engine, err := NewEngine(cfg, ledger, WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(*http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return engineHTTP(t, engineCall(t, "submit_story", draft, "initial-submit")), nil
				}
				if failure == "transport" {
					return nil, errors.New("unknown transport completion")
				}
				if failure == "malformed" {
					return engineHTTP(t, engineMessage(t, map[string]any{"accepted": false})), nil
				}
				return engineHTTP(t, engineMessage(t, revisionRejected(review))), nil
			})}))
			if err != nil {
				t.Fatal(err)
			}
			if accepted, err := engine.Run(context.Background(), "revision-no-"+strings.ReplaceAll(failure, " ", "-"), inv, nil); err == nil || accepted.ID() != "" || calls != 2 {
				t.Fatalf("unfunded or invalid revision was paid: %v calls=%d", err, calls)
			}
		})
	}
}

func TestRevisionPairReservesBothBeforeExecutionAndRefundsFirstOnFailure(t *testing.T) {
	ledger := engineLedger(t)
	if _, err := ledger.Reserve("other", "hold", 999900); err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(engineConfig(), ledger)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := engine.reserveRevisionPair("pair", 0, 50, 75); err == nil {
		t.Fatal("unfunded pair reserved")
	}
	if _, err := ledger.Reserve("other", "remaining", 100); err != nil {
		t.Fatalf("unexecuted first receipt remained charged: %v", err)
	}
}

func TestRevisionReceiptsAuthorizeOnlyOnePhysicalCall(t *testing.T) {
	ledger := engineLedger(t)
	calls := 0
	engine, err := NewEngine(engineConfig(), ledger, WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(*http.Request) (*http.Response, error) {
		calls++
		return engineHTTP(t, engineMessage(t, Review{})), nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	budget := engineBudget{}
	request := responseRequest{Instructions: "pre-reserved call"}
	prepared, err := engine.prepareRequest(&budget, request, false)
	if err != nil {
		t.Fatal(err)
	}
	first, second, err := engine.reserveRevisionPair("single-use", 0, prepared.maximum, prepared.maximum)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.releaseUnused(first, second)
	if _, err := engine.respondReserved(context.Background(), "single-use", &budget, request, false, first); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.respondReserved(context.Background(), "single-use", &budget, request, false, first); err == nil || calls != 1 {
		t.Fatalf("receipt was reused: %v calls=%d", err, calls)
	}
	duplicate, err := ledger.Reserve("single-use", "responses-v1-1", prepared.maximum)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.respondReserved(context.Background(), "single-use", &engineBudget{}, request, false, &reservedCall{jobID: "single-use", call: 1, reservation: duplicate}); err == nil || calls != 1 {
		t.Fatal("persisted duplicate authorized a new physical call")
	}
}

func TestRevisionRequiresFullBoundedFeedbackWithoutTruncation(t *testing.T) {
	inv, _, draft, _ := engineInvestigation(t)
	for _, p := range []string{"README.md", "src/report.rs"} {
		if _, err := inv.Read(p); err != nil {
			t.Fatal(err)
		}
	}
	for len(draft.Statements) < 40 {
		statement := draft.Statements[0]
		statement.ID = "extra-" + strconv.Itoa(len(draft.Statements))
		draft.Statements = append(draft.Statements, statement)
	}
	feedback := Review{Accepted: false, Reason: "Every statement needs review."}
	for _, statement := range draft.Statements {
		feedback.Decisions = append(feedback.Decisions, Decision{StatementID: statement.ID, Reason: strings.Repeat("<", 140)})
	}
	if len(engineJSON(t, feedback)) <= 32<<10 {
		t.Fatal("feedback fixture is not over the serialization cap")
	}
	cfg := revisionConfig()
	calls := 0
	engine, err := NewEngine(cfg, engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(*http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return engineHTTP(t, engineCall(t, "submit_story", draft, "many-statements")), nil
		}
		return engineHTTP(t, engineMessage(t, feedback)), nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Run(context.Background(), "full-feedback", inv, nil); err == nil || calls != 2 {
		t.Fatalf("oversized feedback was silently truncated or revised: %v calls=%d", err, calls)
	}
}

func TestEngineReservesFiveInitialWriterTurnsAndRevisionPairWithinEightCalls(t *testing.T) {
	inv, _, draft, review := engineInvestigation(t)
	for _, path := range []string{"README.md", "src/report.rs"} {
		if _, err := inv.Read(path); err != nil {
			t.Fatal(err)
		}
	}
	cfg := revisionConfig()
	cfg.MaxCalls = 8
	calls := 0
	engine, err := NewEngine(cfg, engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		switch calls {
		case 1, 2, 3, 4:
			if request["tool_choice"] != "required" {
				t.Fatal("initial investigation was shortened despite adequate budget")
			}
			return engineHTTP(t, engineCall(t, "read_file", map[string]any{"path": "src/report.rs", "start_line": 1, "line_count": 10}, "read-"+strconv.Itoa(calls))), nil
		case 5:
			choice, _ := request["tool_choice"].(map[string]any)
			if choice["name"] != "submit_story" {
				t.Fatal("initial writer consumed revision call allocation")
			}
			return engineHTTP(t, engineCall(t, "submit_story", draft, "first-final")), nil
		case 6:
			return engineHTTP(t, engineMessage(t, revisionRejected(review))), nil
		case 7:
			draft.Statements[2].Text = "The rows are passed to aggregate."
			return engineHTTP(t, engineCall(t, "submit_story", draft, "revision-final")), nil
		case 8:
			return engineHTTP(t, engineMessage(t, review)), nil
		default:
			t.Fatal("call budget exceeded")
			return nil, errors.New("unexpected call")
		}
	})}))
	if err != nil {
		t.Fatal(err)
	}
	if accepted, err := engine.Run(context.Background(), "exact-eight", inv, nil); err != nil || accepted.ID() == "" || calls != 8 {
		t.Fatalf("bounded revision path failed: %v calls=%d", err, calls)
	}
	for _, invalid := range []EngineConfig{func() EngineConfig { value := cfg; value.MaxRevisions = 2; return value }(), func() EngineConfig { value := cfg; value.MaxCalls = 3; return value }()} {
		if _, err := NewEngine(invalid, engineLedger(t)); err == nil {
			t.Fatal("invalid revision/call policy accepted")
		}
	}
}
