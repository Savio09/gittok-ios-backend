package walkthrough

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/generation"
)

func TestProviderUnknownOutcomesRetainReservedMaximum(t *testing.T) {
	for _, failure := range []string{"transport", "http", "incomplete", "missing usage", "malformed", "oversized"} {
		t.Run(failure, func(t *testing.T) {
			ledger := engineLedger(t)
			client := &http.Client{Transport: engineRoundTrip(func(*http.Request) (*http.Response, error) {
				switch failure {
				case "transport":
					return nil, errors.New("private-provider-secret")
				case "http":
					return &http.Response{StatusCode: 429, Body: io.NopCloser(strings.NewReader("private-provider-secret"))}, nil
				case "incomplete":
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"status":"incomplete","output":[]}`))}, nil
				case "missing usage":
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"status":"completed","output":[{"type":"message"}]}`))}, nil
				case "malformed":
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("private-provider-secret"))}, nil
				default:
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", (1<<20)+1)))}, nil
				}
			})}
			engine, err := NewEngine(engineConfig(), ledger, WithHTTPClient(client))
			if err != nil {
				t.Fatal(err)
			}
			budget := engineBudget{}
			_, err = engine.respond(context.Background(), "unknown-"+strings.ReplaceAll(failure, " ", "-"), &budget, responseRequest{Instructions: "test"}, false)
			if err == nil || strings.Contains(err.Error(), "private-provider-secret") || budget.spent <= 2 || budget.calls != 1 {
				t.Fatalf("unknown outcome: budget=%+v error=%v", budget, err)
			}
			if _, err := ledger.Reserve("other", "remaining", 1000000-budget.spent); err != nil {
				t.Fatal(err)
			}
			if _, err := ledger.Reserve("other", "overflow", 1); !errors.Is(err, generation.ErrBudgetExceeded) {
				t.Fatalf("unknown charge refunded: %v", err)
			}
		})
	}
}

func TestProviderVerifiedUsageSettlesActualCostAndOverchargeBlocks(t *testing.T) {
	for _, overcharge := range []bool{false, true} {
		t.Run(map[bool]string{false: "actual", true: "overcharge"}[overcharge], func(t *testing.T) {
			ledger := engineLedger(t)
			engine, err := NewEngine(engineConfig(), ledger, WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(r *http.Request) (*http.Response, error) {
				if r.Header.Get("Idempotency-Key") == "" {
					t.Fatal("missing stable provider idempotency key")
				}
				if !overcharge {
					return engineHTTP(t, engineMessage(t, Review{})), nil
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"status":"completed","usage":{"input_tokens":100,"output_tokens":1000000,"total_tokens":1000100},"output":[{"type":"message"}]}`))}, nil
			})}))
			if err != nil {
				t.Fatal(err)
			}
			budget := engineBudget{}
			_, err = engine.respond(context.Background(), "usage", &budget, responseRequest{Instructions: "test"}, false)
			if !overcharge {
				if err != nil || budget.spent != 2 || budget.output != 10 {
					t.Fatalf("verified charge: %+v %v", budget, err)
				}
			} else {
				if err == nil {
					t.Fatal("overcharge accepted")
				}
				if _, err := ledger.Reserve("other", "one", 1); !errors.Is(err, generation.ErrLedgerBlocked) {
					t.Fatalf("overcharge did not block ledger: %v", err)
				}
			}
		})
	}
}

func TestEngineCumulativeLimitsStopExplorationAndReserveReview(t *testing.T) {
	for _, kind := range []string{"calls", "output", "input", "spend"} {
		t.Run(kind, func(t *testing.T) {
			inv, _, _, _ := engineInvestigation(t)
			cfg := engineConfig()
			switch kind {
			case "calls":
				cfg.MaxCalls = 3
			case "output":
				cfg.MaxOutputTokens = 30
			case "input":
				cfg.MaxInputBytes = 1
			case "spend":
				cfg.MaxJobMicros = 1
			}
			calls := 0
			engine, err := NewEngine(cfg, engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(*http.Request) (*http.Response, error) {
				calls++
				return engineHTTP(t, engineCall(t, "list_paths", map[string]any{"query": "src", "limit": 2}, "call-"+string(rune('a'+calls)))), nil
			})}))
			if err != nil {
				t.Fatal(err)
			}
			expectedError := ErrGenerationBudget
			if kind == "calls" || kind == "output" {
				expectedError = ErrProviderResponse
			} // This fake ignores forced submission.
			if _, err := engine.Run(context.Background(), "cap-"+kind, inv, nil); !errors.Is(err, expectedError) {
				t.Fatalf("budget did not stop: %v", err)
			}
			if (kind == "calls" && calls != 2) || (kind == "output" && calls != 1) || ((kind == "input" || kind == "spend") && calls != 0) {
				t.Fatalf("unexpected paid calls: %d", calls)
			}
		})
	}
}

func TestEngineRejectsRefusalMalformedSchemaAndUnsafeToolWithoutRetry(t *testing.T) {
	for _, kind := range []string{"refusal", "unknown field", "unsafe tool", "multiple calls"} {
		t.Run(kind, func(t *testing.T) {
			inv, _, _, _ := engineInvestigation(t)
			calls := 0
			engine, err := NewEngine(engineConfig(), engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(*http.Request) (*http.Response, error) {
				calls++
				switch kind {
				case "refusal":
					return engineHTTP(t, map[string]any{"type": "message", "status": "completed", "content": []any{map[string]any{"type": "refusal", "refusal": "private-model-text"}}}), nil
				case "unknown field":
					return engineHTTP(t, engineCall(t, "submit_story", map[string]any{"statements": []any{}, "scenes": []any{}, "unreviewed_body": "private-model-text"}, "c1")), nil
				case "unsafe tool":
					return engineHTTP(t, engineCall(t, "read_file", map[string]any{"path": "/etc/passwd", "start_line": 1, "line_count": 1}, "c1")), nil
				default:
					return engineHTTP(t, engineCall(t, "list_paths", map[string]any{"query": "", "limit": 1}, "c1"), engineCall(t, "list_paths", map[string]any{"query": "", "limit": 1}, "c2")), nil
				}
			})}))
			if err != nil {
				t.Fatal(err)
			}
			if accepted, err := engine.Run(context.Background(), "invalid-"+strings.ReplaceAll(kind, " ", "-"), inv, nil); err == nil || accepted.ID() != "" || strings.Contains(err.Error(), "private-model-text") || calls != 1 {
				t.Fatalf("invalid response retried or published: %s %v calls=%d", accepted.ID(), err, calls)
			}
		})
	}
}

func TestProviderCancellationBeforeReservationAndStrictSchemas(t *testing.T) {
	engine, err := NewEngine(engineConfig(), engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(*http.Request) (*http.Response, error) {
		t.Fatal("cancelled call reached transport")
		return nil, errors.New("unexpected")
	})}))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	budget := engineBudget{}
	if _, err := engine.respond(ctx, "cancelled", &budget, responseRequest{}, false); !errors.Is(err, context.Canceled) || budget.calls != 0 {
		t.Fatalf("cancelled call charged: %+v %v", budget, err)
	}
	var check func(any)
	check = func(schema any) {
		m, ok := schema.(map[string]any)
		if !ok {
			return
		}
		if m["type"] == "object" {
			properties := m["properties"].(map[string]any)
			if m["additionalProperties"] != false || len(m["required"].([]string)) != len(properties) {
				t.Fatal("non-strict object schema")
			}
			for _, p := range properties {
				check(p)
			}
		}
		if item, ok := m["items"]; ok {
			check(item)
		}
	}
	for _, tool := range writerTools() {
		if !tool.Strict {
			t.Fatal("non-strict tool")
		}
		check(tool.Parameters)
	}
	check(reviewSchema())
	if _, err := modelCost(math.MaxInt64, 1, engineConfig()); err == nil {
		t.Fatal("overflowing token charge accepted")
	}
	for _, origin := range []string{"file:///etc/passwd", "https://key@provider.test", "https://provider.test?key=value"} {
		if _, err := NewEngine(engineConfig(), engineLedger(t), WithBaseURL(origin)); err == nil {
			t.Fatal("unsafe explicit test origin accepted")
		}
	}
}

func TestProviderConnectionFailureNeverReplaysPaidPOST(t *testing.T) {
	var physical atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if physical.Add(1) == 2 {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			_ = conn.Close()
			return
		}
		response := engineHTTP(t, engineMessage(t, Review{}))
		defer response.Body.Close()
		_, _ = io.Copy(w, response.Body)
	}))
	defer server.Close()
	cfg := engineConfig()
	cfg.MaxCalls = 3 // Two physical writer attempts plus the reserved reviewer slot.
	engine, err := NewEngine(cfg, engineLedger(t), WithBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	budget := engineBudget{}
	if _, err := engine.respond(context.Background(), "physical-post", &budget, responseRequest{Instructions: "prime connection"}, false); err != nil {
		t.Fatal(err)
	}
	_, secondErr := engine.respond(context.Background(), "physical-post", &budget, responseRequest{Instructions: "must not replay"}, false)
	if physical.Load() != 2 || secondErr == nil {
		t.Fatalf("paid POST replayed: physical=%d error=%v", physical.Load(), secondErr)
	}
}

func TestProviderStatusErrorExposesOnlyHTTPStatus(t *testing.T) {
	engine, err := NewEngine(engineConfig(), engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("provider-body-secret explicit-test-key"))}, nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = engine.respond(context.Background(), "safe-status", &engineBudget{}, responseRequest{Instructions: "test"}, false)
	var status *ProviderStatusError
	if !errors.As(err, &status) || status.Status != 503 || !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("missing safe HTTP status: %v", err)
	}
	if err.Error() != ErrProviderUnavailable.Error() || strings.Contains(err.Error(), "provider-body-secret") || strings.Contains(err.Error(), "explicit-test-key") {
		t.Fatalf("provider diagnostic leaked content: %v", err)
	}
}

func TestProviderUsesConfiguredReasoningAndStandardServiceTier(t *testing.T) {
	for _, effort := range []string{"", "low"} {
		t.Run("effort-"+effort, func(t *testing.T) {
			cfg := engineConfig()
			cfg.ReasoningEffort = effort
			engine, err := NewEngine(cfg, engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(r *http.Request) (*http.Response, error) {
				var request map[string]any
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Fatal(err)
				}
				if request["service_tier"] != "default" {
					t.Fatal("request inherited an unpriced project service tier")
				}
				if effort == "" {
					if request["reasoning"] != nil {
						t.Fatal("empty configuration changed provider reasoning default")
					}
				} else if request["reasoning"].(map[string]any)["effort"] != effort {
					t.Fatal("configured reasoning effort not sent")
				}
				return engineHTTP(t, engineMessage(t, Review{})), nil
			})}))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := engine.respond(context.Background(), "reasoning-"+effort, &engineBudget{}, responseRequest{Instructions: "bounded wire test"}, false); err != nil {
				t.Fatal(err)
			}
		})
	}
	cfg := engineConfig()
	cfg.ReasoningEffort = "unbounded"
	if _, err := NewEngine(cfg, engineLedger(t)); err == nil {
		t.Fatal("unknown reasoning effort accepted")
	}
}
