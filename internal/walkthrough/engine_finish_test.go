package walkthrough

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestReadyWriterGetsDedicatedFullSubmissionTurn(t *testing.T) {
	for _, revisions := range []int{0, 1} {
		t.Run(string(rune('0'+revisions)), func(t *testing.T) {
			inv, _, draft, review := engineInvestigation(t)
			for _, path := range []string{"README.md", "src/report.rs"} {
				if _, err := inv.Read(path); err != nil {
					t.Fatal(err)
				}
			}
			cfg := engineConfig()
			cfg.MaxCalls = 8
			cfg.MaxRevisions = revisions
			calls := 0
			engine, err := NewEngine(cfg, engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				var request responseRequest
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Fatal(err)
				}
				switch calls {
				case 1:
					ready := false
					for _, tool := range request.Tools {
						if tool.Name == "submit_story" {
							t.Error("low-cap exploration exposed full-story submission")
						}
						ready = ready || tool.Name == "finish_reading"
					}
					cap := 1024
					if revisions == 1 {
						cap = 512
					}
					if !ready || request.MaxOutputTokens > cap {
						t.Error("bounded ready tool missing from exploration")
					}
					return engineHTTP(t, engineCall(t, "finish_reading", map[string]any{}, "ready")), nil
				case 2:
					choice, _ := request.ToolChoice.(map[string]any)
					if choice["name"] != "submit_story" || len(request.Tools) != 1 || request.Tools[0].Name != "submit_story" || request.MaxOutputTokens < 3900 {
						t.Fatalf("dedicated final turn did not use its full remaining initial allocation: cap=%d", request.MaxOutputTokens)
					}
					return engineHTTP(t, engineCall(t, "submit_story", draft, "full-story")), nil
				case 3:
					return engineHTTP(t, engineMessage(t, review)), nil
				default:
					t.Fatal("unexpected retry")
					return nil, ErrProviderUnavailable
				}
			})}))
			if err != nil {
				t.Fatal(err)
			}
			accepted, err := engine.Run(context.Background(), "ready-phase-"+string(rune('0'+revisions)), inv, nil)
			if err != nil || accepted.ID() == "" || calls != 3 {
				t.Fatalf("ready writer lost dedicated submission: %v calls=%d", err, calls)
			}
		})
	}
}

func TestUnexpectedEarlyStorySubmissionIsRejectedWithoutRetry(t *testing.T) {
	inv, _, draft, _ := engineInvestigation(t)
	cfg := engineConfig()
	cfg.MaxCalls = 8
	calls := 0
	engine, err := NewEngine(cfg, engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(*http.Request) (*http.Response, error) {
		calls++
		return engineHTTP(t, engineCall(t, "submit_story", draft, "unexpected-early")), nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	if accepted, err := engine.Run(context.Background(), "early-malformed", inv, nil); !errors.Is(err, ErrProviderResponse) || accepted.ID() != "" || calls != 1 {
		t.Fatalf("unexpected early story was executed or retried: %v calls=%d", err, calls)
	}
}
