package walkthrough

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func revisionHTTPUsage(t *testing.T, tokens int, output ...any) *http.Response {
	response := engineHTTP(t, output...)
	raw, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	value["usage"] = map[string]int{"input_tokens": 100, "output_tokens": tokens, "total_tokens": 100 + tokens}
	response.Body = io.NopCloser(strings.NewReader(engineJSON(t, value)))
	return response
}

func TestRevisionPairWith4001RemainingNeverPaysForOneTokenRewrite(t *testing.T) {
	inv, _, draft, review := engineInvestigation(t)
	for _, path := range []string{"README.md", "src/report.rs"} {
		if _, err := inv.Read(path); err != nil {
			t.Fatal(err)
		}
	}
	cfg := revisionConfig()
	calls := 0
	engine, err := NewEngine(cfg, engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		var request responseRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.MaxOutputTokens < 1024 {
			t.Errorf("paid call shrunk to unusable output cap %d", request.MaxOutputTokens)
		}
		if calls == 1 {
			return engineHTTP(t, engineCall(t, "submit_story", draft, "revision-submit")), nil
		}
		return engineHTTP(t, engineMessage(t, review)), nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	budget := engineBudget{calls: 2, output: cfg.MaxOutputTokens - 4001}
	_, accepted, err := engine.revise(context.Background(), "tight-pair", inv.Snapshot(), draft, revisionRejected(review), &budget, func(string) {})
	if err != nil || !accepted.Accepted || calls != 2 {
		t.Fatalf("funded tight pair failed: %v calls=%d", err, calls)
	}
}

func TestRevisionOutputPhasesPreservePairAfterMaximumInitialUsage(t *testing.T) {
	inv, _, draft, review := engineInvestigation(t)
	for _, path := range []string{"README.md", "src/report.rs"} {
		if _, err := inv.Read(path); err != nil {
			t.Fatal(err)
		}
	}
	cfg := revisionConfig()
	calls, writerOutput, totalOutput, reviews := 0, 0, 0, 0
	engine, err := NewEngine(cfg, engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		var request responseRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		cap := request.MaxOutputTokens
		totalOutput += cap
		if request.Text != nil {
			reviews++
			if cap > cfg.MaxOutputTokens/6 {
				t.Errorf("review phase exceeded its share: %d", cap)
			}
			if reviews == 1 {
				return revisionHTTPUsage(t, cap, engineMessage(t, revisionRejected(review))), nil
			}
			return revisionHTTPUsage(t, cap, engineMessage(t, review)), nil
		}
		if reviews == 1 {
			if cap > cfg.MaxOutputTokens/3 || cap < 1024 {
				t.Errorf("unusable revision phase: %d", cap)
			}
			return revisionHTTPUsage(t, cap, engineCall(t, "submit_story", draft, "revision-finish")), nil
		}
		writerOutput += cap
		if request.ToolChoice == "required" {
			if cap > 512 {
				t.Errorf("revision-mode exploration cap=%d", cap)
			}
			return revisionHTTPUsage(t, cap, engineCall(t, "read_file", map[string]any{"path": "src/report.rs", "start_line": 1, "line_count": 10}, "read-phase-"+string(rune('a'+calls)))), nil
		}
		if cap > cfg.MaxOutputTokens/3 {
			t.Errorf("initial final cap=%d", cap)
		}
		return revisionHTTPUsage(t, cap, engineCall(t, "submit_story", draft, "first-finish")), nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := engine.Run(context.Background(), "output-phases", inv, nil)
	if err != nil || accepted.ID() == "" || reviews != 2 || writerOutput > cfg.MaxOutputTokens/3 || totalOutput > cfg.MaxOutputTokens || calls > cfg.MaxCalls {
		t.Fatalf("initial phases consumed the revision pair: %v writer=%d total=%d calls=%d", err, writerOutput, totalOutput, calls)
	}
}

func TestMalformedReviewKeysNeverTriggerPaidRevision(t *testing.T) {
	for _, kind := range []string{"uppercase root", "duplicate accepted", "uppercase decision", "duplicate decision key", "missing accepted", "empty reason", "empty decisions", "duplicate statement decision", "unknown statement decision"} {
		t.Run(kind, func(t *testing.T) {
			inv, _, draft, review := engineInvestigation(t)
			for _, path := range []string{"README.md", "src/report.rs"} {
				if _, err := inv.Read(path); err != nil {
					t.Fatal(err)
				}
			}
			raw := engineJSON(t, revisionRejected(review))
			switch kind {
			case "uppercase root":
				raw = strings.Replace(raw, `"accepted":false`, `"Accepted":false`, 1)
			case "duplicate accepted":
				raw = strings.Replace(raw, `"accepted":false`, `"accepted":true,"accepted":false`, 1)
			case "uppercase decision":
				raw = strings.Replace(raw, `"statement_id":"p"`, `"Statement_ID":"p"`, 1)
			case "duplicate decision key":
				raw = strings.Replace(raw, `"statement_id":"p"`, `"statement_id":"p","statement_id":"p"`, 1)
			case "missing accepted":
				raw = strings.Replace(raw, `"accepted":false,`, "", 1)
			case "empty reason":
				negative := revisionRejected(review)
				negative.Reason = ""
				raw = engineJSON(t, negative)
			case "empty decisions":
				negative := revisionRejected(review)
				negative.Decisions = []Decision{}
				raw = engineJSON(t, negative)
			case "duplicate statement decision":
				negative := revisionRejected(review)
				negative.Decisions[1].StatementID = negative.Decisions[0].StatementID
				raw = engineJSON(t, negative)
			case "unknown statement decision":
				negative := revisionRejected(review)
				negative.Decisions[0].StatementID = "absent"
				raw = engineJSON(t, negative)
			}
			cfg := revisionConfig()
			calls := 0
			engine, err := NewEngine(cfg, engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(*http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return engineHTTP(t, engineCall(t, "submit_story", draft, "first-submit")), nil
				}
				if calls == 2 {
					return engineHTTP(t, engineMessage(t, json.RawMessage(raw))), nil
				}
				return nil, errors.New("malformed feedback must not trigger rewrite")
			})}))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := engine.Run(context.Background(), "malformed-review-"+strings.ReplaceAll(kind, " ", "-"), inv, nil); err == nil || calls != 2 {
				t.Fatalf("malformed review triggered revision: %v calls=%d", err, calls)
			}
		})
	}
}
