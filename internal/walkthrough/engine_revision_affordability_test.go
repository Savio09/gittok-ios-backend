package walkthrough

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strings"
	"testing"
)

func pricedRevisionConfig() EngineConfig {
	cfg := revisionConfig()
	cfg.InputMicrosPerMillion = 2500000
	cfg.OutputMicrosPerMillion = 10000000
	cfg.MaxJobMicros = 500000
	return cfg
}

func TestAffordableReviewBodyHonorsMoneyInputMinimumAndAbsoluteBounds(t *testing.T) {
	engine := &Engine{cfg: pricedRevisionConfig()}
	for _, test := range []struct {
		name              string
		bytes             int
		micros            int64
		output, wantBytes int
	}{
		{"input bound", 10000, 500000, 2000, 10000},
		{"money bound", finalReviewBodyCap, 170000, 2000, 27952},
		{"absolute bound", 4 << 20, 1000000, 2000, finalReviewBodyCap},
		{"exact useful minimum", finalReviewBodyCap, 71200, 2000, minimumFinalReviewBodyBytes},
		{"insufficient minimum input", minimumFinalReviewBodyBytes - 1, 500000, 2000, 0},
		{"insufficient minimum money", finalReviewBodyCap, 71199, 2000, 0},
		{"insufficient output", finalReviewBodyCap, 500000, 1023, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			bytes, cost, err := engine.affordableReviewBody(test.bytes, test.micros, test.output)
			if test.wantBytes == 0 {
				if !errors.Is(err, ErrGenerationBudget) {
					t.Fatal("unusable review allocation accepted")
				}
				return
			}
			if err != nil || bytes != test.wantBytes || cost > test.micros {
				t.Fatalf("unexpected bounded allocation: bytes=%d cost=%d err=%v", bytes, cost, err)
			}
		})
	}
	engine.cfg.InputMicrosPerMillion = math.MaxInt64
	if _, _, err := engine.affordableReviewBody(finalReviewBodyCap, math.MaxInt64, 2000); !errors.Is(err, ErrGenerationBudget) {
		t.Fatal("overflowing cost became an affordable review")
	}
}

func TestRevisionPairFitsRealisticPricesAndUnchangedHalfDollarJobCap(t *testing.T) {
	inv, _, draft, review := engineInvestigation(t)
	for _, path := range []string{"README.md", "src/report.rs"} {
		if _, err := inv.Read(path); err != nil {
			t.Fatal(err)
		}
	}
	cfg := pricedRevisionConfig()
	calls := 0
	engine, err := NewEngine(cfg, engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		var request responseRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.MaxOutputTokens < 1024 {
			t.Fatal("paired request has unusable output allowance")
		}
		if calls == 1 {
			return engineHTTP(t, engineCall(t, "submit_story", draft, "affordable-rewrite")), nil
		}
		if calls != 2 || request.Text == nil || len(request.Input) != 1 || len(request.Tools) != 0 {
			t.Fatal("revision did not receive one independent fresh review")
		}
		var message struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(request.Input[0], &message); err != nil {
			t.Fatal(err)
		}
		var packet struct {
			Windows []sourceWindow `json:"cited_source_context"`
		}
		if err := json.Unmarshal([]byte(message.Content), &packet); err != nil {
			t.Fatal(err)
		}
		for _, file := range inv.Snapshot().Files {
			found := false
			for _, window := range packet.Windows {
				if window.FileID == file.ID {
					var lines []string
					for _, line := range window.Lines {
						lines = append(lines, line.Text)
					}
					found = strings.Join(lines, "\n")+"\n" == file.Content
				}
			}
			if !found {
				t.Fatal("affordable reservation omitted complete retained cited source")
			}
		}
		return engineHTTP(t, engineMessage(t, review)), nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	budget := engineBudget{calls: 2, input: 20000, output: 1500, spent: 100000}
	_, accepted, err := engine.revise(context.Background(), "priced-pair", inv.Snapshot(), draft, revisionRejected(review), &budget, func(string) {})
	if err != nil || !accepted.Accepted || calls != 2 || budget.spent > cfg.MaxJobMicros {
		t.Fatalf("affordable pair refused or exceeded the job cap: %v calls=%d spent=%d", err, calls, budget.spent)
	}
}

func TestUnfundedPricedRevisionPairMakesNoHTTPCallsOrReservations(t *testing.T) {
	inv, _, draft, review := engineInvestigation(t)
	for _, path := range []string{"README.md", "src/report.rs"} {
		if _, err := inv.Read(path); err != nil {
			t.Fatal(err)
		}
	}
	ledger := engineLedger(t)
	calls := 0
	engine, err := NewEngine(pricedRevisionConfig(), ledger, WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(*http.Request) (*http.Response, error) { calls++; return nil, ErrProviderUnavailable })}))
	if err != nil {
		t.Fatal(err)
	}
	budget := engineBudget{calls: 2, input: 20000, output: 1500, spent: 350000}
	_, _, err = engine.revise(context.Background(), "unfunded-priced-pair", inv.Snapshot(), draft, revisionRejected(review), &budget, func(string) {})
	if !errors.Is(err, ErrGenerationBudget) || calls != 0 {
		t.Fatalf("unfunded correction paid a call: %v calls=%d", err, calls)
	}
	if _, err := ledger.Reserve("uncharged-proof", "all", 1000000); err != nil {
		t.Fatal("unexecuted pair retained a reservation", err)
	}
}

func TestPricedCorrectionExceedingReservedReviewBodyCannotPayFinalReview(t *testing.T) {
	inv, g, original, review := engineInvestigation(t)
	g.files["src/report.rs"] += strings.Repeat("x", 2200) + "\n"
	for _, path := range []string{"README.md", "src/report.rs"} {
		if _, err := inv.Read(path); err != nil {
			t.Fatal(err)
		}
	}
	revised := Draft{}
	for _, kind := range []string{"purpose", "action", "mechanism", "result", "design", "handoff"} {
		statement := Statement{ID: kind, Kind: kind, Text: "A bounded candidate.", Visual: copyVisual(original.Statements[0].Visual)}
		statement.Visual.Cue = "bounded"
		for i := 0; i < 3; i++ {
			statement.Sources = append(statement.Sources, Citation{FileID: inv.sourceIDs["src/report.rs"], StartLine: 5, EndLine: 5})
		}
		revised.Statements = append(revised.Statements, statement)
	}
	revised.Scenes = []Scene{{ID: "flow", Role: "workflow", StatementIDs: []string{"purpose", "action", "mechanism", "result", "design"}}, {ID: "next", Role: "handoff", StatementIDs: []string{"handoff"}}}
	calls := 0
	engine, err := NewEngine(pricedRevisionConfig(), engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(*http.Request) (*http.Response, error) {
		calls++
		if calls != 1 {
			t.Fatal("oversized corrected review executed an HTTP call")
		}
		return engineHTTP(t, engineCall(t, "submit_story", revised, "larger-correction")), nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	grounded, err := groundDraft(inv.Snapshot(), revised)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateGeneratedDraft(inv.Snapshot(), grounded); err != nil {
		t.Fatal(err)
	}
	request, err := engine.reviewRequest(inv.Snapshot(), grounded, 2000)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := engine.prepareRequest(&engineBudget{}, request, true)
	if err != nil || len(prepared.body) >= finalReviewBodyCap {
		t.Fatal("control must fit absolute review cap but exceed affordable pair bound")
	}
	budget := engineBudget{calls: 2, input: 20000, output: 1500, spent: 200000}
	_, _, err = engine.revise(context.Background(), "priced-oversize", inv.Snapshot(), original, revisionRejected(review), &budget, func(string) {})
	if !errors.Is(err, ErrGenerationBudget) || calls != 1 {
		t.Fatalf("unreserved review was paid or useful rewrite never attempted: %v calls=%d", err, calls)
	}
}
