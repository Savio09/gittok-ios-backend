package walkthrough

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/generation"
)

var (
	ErrGenerationBudget    = errors.New("walkthrough generation limit reached")
	ErrProviderUnavailable = errors.New("walkthrough provider unavailable")
	ErrProviderResponse    = errors.New("walkthrough provider response invalid")
	ErrStoryRejected       = errors.New("walkthrough story not accepted")
)

// ProviderStatusError exposes only the HTTP status for internal diagnostics.
// Provider response bodies, prompts, credentials and model text are excluded.
type ProviderStatusError struct{ Status int }

func (*ProviderStatusError) Error() string { return ErrProviderUnavailable.Error() }
func (*ProviderStatusError) Unwrap() error { return ErrProviderUnavailable }

// EngineConfig requires operator-selected model and matching published pricing;
// there are no model, credential or pricing defaults. Calls, serialized input
// bytes and output tokens are cumulative across writer, tools and reviewer.
// MaxRevisions is 0 or 1; one revision requires at least four total calls.
type EngineConfig struct {
	APIKey, Model                                 string
	ReasoningEffort                               string
	InputMicrosPerMillion, OutputMicrosPerMillion int64
	MaxCalls, MaxOutputTokens, MaxInputBytes      int
	MaxJobMicros                                  int64
	MaxRevisions                                  int
}

type Engine struct {
	cfg    EngineConfig
	ledger *generation.Ledger
	client *http.Client
	origin string
}
type EngineOption func(*Engine) error

// WithHTTPClient and WithBaseURL are explicit dependency injection for tests.
// Production callers use NewEngine without either option.
func WithHTTPClient(client *http.Client) EngineOption {
	return func(e *Engine) error {
		if client == nil {
			return ErrProviderUnavailable
		}
		copied := *client
		e.client = &copied
		return nil
	}
}
func WithBaseURL(origin string) EngineOption {
	return func(e *Engine) error {
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			return ErrProviderUnavailable
		}
		e.origin = strings.TrimRight(origin, "/")
		return nil
	}
}

func NewEngine(cfg EngineConfig, ledger *generation.Ledger, opts ...EngineOption) (*Engine, error) {
	if ledger == nil || cfg.APIKey == "" || len(cfg.APIKey) > 512 || strings.ContainsAny(cfg.APIKey, " \t\r\n") || !idPattern.MatchString(cfg.Model) || cfg.InputMicrosPerMillion <= 0 || cfg.OutputMicrosPerMillion <= 0 || cfg.MaxCalls < 2 || cfg.MaxCalls > 16 || cfg.MaxInputBytes < 1 || cfg.MaxInputBytes > 4<<20 || cfg.MaxOutputTokens < 2 || cfg.MaxOutputTokens > 64000 || cfg.MaxJobMicros <= 0 || cfg.MaxRevisions < 0 || cfg.MaxRevisions > 1 || cfg.MaxRevisions == 1 && cfg.MaxCalls < 4 || !validReasoningEffort(cfg.ReasoningEffort) {
		return nil, ErrProviderUnavailable
	}
	if _, err := modelCost(int64(cfg.MaxInputBytes)*2+4096, int64(cfg.MaxOutputTokens), cfg); err != nil {
		return nil, err
	}
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		DisableKeepAlives:   true,
		ForceAttemptHTTP2:   false,
		TLSNextProto:        map[string]func(string, *tls.Conn) http.RoundTripper{},
	}
	e := &Engine{cfg: cfg, ledger: ledger, client: &http.Client{Transport: transport}, origin: "https://api.openai.com"}
	for _, option := range opts {
		if option == nil {
			return nil, ErrProviderUnavailable
		}
		if err := option(e); err != nil {
			return nil, err
		}
	}
	// Redirects must never move credentials or silently create a second request.
	e.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return e, nil
}

type engineBudget struct {
	calls, input, output int
	spent                int64
}
type responseRequest struct {
	Model             string             `json:"model"`
	Instructions      string             `json:"instructions"`
	Input             []json.RawMessage  `json:"input"`
	Tools             []responseTool     `json:"tools,omitempty"`
	ToolChoice        any                `json:"tool_choice,omitempty"`
	Text              any                `json:"text,omitempty"`
	MaxOutputTokens   int                `json:"max_output_tokens"`
	ParallelToolCalls bool               `json:"parallel_tool_calls"`
	Store             bool               `json:"store"`
	Include           []string           `json:"include"`
	Reasoning         *responseReasoning `json:"reasoning,omitempty"`
	ServiceTier       string             `json:"service_tier"`
	pairPhase         bool
}
type responseReasoning struct {
	Effort string `json:"effort"`
}

func validReasoningEffort(effort string) bool {
	switch effort {
	case "", "none", "low", "medium", "high", "xhigh", "max":
		return true
	}
	return false
}

type responseTool struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Strict      bool   `json:"strict"`
	Parameters  any    `json:"parameters"`
}
type providerUsage struct {
	Input  *int64 `json:"input_tokens"`
	Output *int64 `json:"output_tokens"`
	Total  *int64 `json:"total_tokens"`
}
type responseEnvelope struct {
	Status string            `json:"status"`
	Output []json.RawMessage `json:"output"`
	Usage  *providerUsage    `json:"usage"`
}
type responseItem struct {
	Type      string `json:"type"`
	Status    string `json:"status"`
	Name      string `json:"name"`
	CallID    string `json:"call_id"`
	Arguments string `json:"arguments"`
	Content   []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

type engineAllowance struct {
	Calls, InputBytes, OutputTokens int
	Micros                          int64
}

func (e *Engine) allowance(budget *engineBudget, reviewer bool) engineAllowance {
	remaining := engineAllowance{Calls: e.cfg.MaxCalls - budget.calls, InputBytes: e.cfg.MaxInputBytes - budget.input, OutputTokens: e.cfg.MaxOutputTokens - budget.output, Micros: e.cfg.MaxJobMicros - budget.spent}
	if !reviewer {
		remaining.Calls--
		remaining.InputBytes -= e.cfg.MaxInputBytes / 2
		remaining.Micros -= e.cfg.MaxJobMicros / 2
		if e.cfg.MaxRevisions == 1 {
			remaining.OutputTokens = e.cfg.MaxOutputTokens/3 - budget.output
		} else {
			remaining.OutputTokens -= min(4096, max(1, e.cfg.MaxOutputTokens/3))
		}
	}
	return remaining
}

type preparedRequest struct {
	request             responseRequest
	body                []byte
	inputUpper, maximum int64
}

func (e *Engine) prepareRequest(budget *engineBudget, request responseRequest, reviewer bool) (preparedRequest, error) {
	remaining := e.allowance(budget, reviewer)
	if request.pairPhase {
		remaining = e.allowance(budget, true)
	}
	if remaining.Calls <= 0 || remaining.OutputTokens <= 0 {
		return preparedRequest{}, ErrGenerationBudget
	}
	request.Model = e.cfg.Model
	// Standard tier keeps operator-supplied standard pricing applicable; an
	// optional effort is model-dependent operator configuration, never source.
	// https://developers.openai.com/api/docs/guides/reasoning
	// https://developers.openai.com/api/docs/guides/fast-mode
	request.ServiceTier = "default"
	request.Reasoning = nil
	if e.cfg.ReasoningEffort != "" {
		request.Reasoning = &responseReasoning{Effort: e.cfg.ReasoningEffort}
	}
	outputCap := min(4096, remaining.OutputTokens)
	if request.MaxOutputTokens > 0 {
		outputCap = min(outputCap, request.MaxOutputTokens)
	}
	request.MaxOutputTokens = outputCap
	request.Include = []string{"reasoning.encrypted_content"}
	body, err := json.Marshal(request)
	if err != nil || len(body) > remaining.InputBytes {
		return preparedRequest{}, ErrGenerationBudget
	}
	// For text-only BPE models, each serialized UTF-8 byte can contribute a
	// token. Double the complete wire size plus 4096 covers message/schema
	// framing conservatively; usage outside this bound never releases spend.
	inputUpper := int64(len(body))*2 + 4096
	maximum, err := modelCost(inputUpper, int64(request.MaxOutputTokens), e.cfg)
	if err != nil || maximum > remaining.Micros {
		return preparedRequest{}, ErrGenerationBudget
	}
	return preparedRequest{request: request, body: body, inputUpper: inputUpper, maximum: maximum}, nil
}

// Responses wire formats and strict tool/output schemas follow official docs:
// https://developers.openai.com/api/docs/guides/function-calling
// https://developers.openai.com/api/docs/guides/structured-outputs?api-mode=responses
// Reasoning continuation is retained statelessly with store:false and encrypted
// reasoning items; no previous_response_id hides repeated input accounting.
func (e *Engine) respond(ctx context.Context, jobID string, budget *engineBudget, request responseRequest, reviewer bool) (responseEnvelope, error) {
	return e.respondReserved(ctx, jobID, budget, request, reviewer, nil)
}

func (e *Engine) respondReserved(ctx context.Context, jobID string, budget *engineBudget, request responseRequest, reviewer bool, receipt *reservedCall) (responseEnvelope, error) {
	var zero responseEnvelope
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	prepared, err := e.prepareRequest(budget, request, reviewer)
	if err != nil {
		return zero, err
	}
	request, body, inputUpper, maximum := prepared.request, prepared.body, prepared.inputUpper, prepared.maximum
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, e.origin+"/v1/responses", bytes.NewReader(body))
	if err != nil {
		return zero, ErrProviderUnavailable
	}
	// A paid attempt consumes its reservation even if the response is lost.
	// Neither Go's redirect machinery nor transport may replay this body.
	httpRequest.GetBody = nil
	httpRequest.Close = true
	httpRequest.Header.Set("Authorization", "Bearer "+e.cfg.APIKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	requestIdentity := sha256.Sum256([]byte(jobID + "\x00responses-v1-" + strconv.Itoa(budget.calls+1)))
	httpRequest.Header.Set("Idempotency-Key", "walkthrough-"+hex.EncodeToString(requestIdentity[:]))
	var reservation generation.Reservation
	if receipt == nil {
		reservation, err = e.ledger.Reserve(jobID, "responses-v1-"+strconv.Itoa(budget.calls+1), maximum)
		if err != nil || reservation.AlreadyReserved {
			return zero, ErrProviderUnavailable
		}
	} else {
		remaining := e.allowance(budget, reviewer)
		if request.pairPhase {
			remaining = e.allowance(budget, true)
		}
		if receipt.consumed || receipt.jobID != jobID || receipt.call != budget.calls+1 || receipt.reservation.AlreadyReserved || maximum > receipt.reservation.MaxCostMicros || receipt.reservation.MaxCostMicros > remaining.Micros {
			return zero, ErrProviderUnavailable
		}
		// The original fresh in-memory receipt is the authorization. This
		// idempotent lookup only checks that its durable charge still exists;
		// a persisted duplicate by itself can never authorize an HTTP call.
		checked, checkErr := e.ledger.Reserve(jobID, "responses-v1-"+strconv.Itoa(receipt.call), receipt.reservation.MaxCostMicros)
		if checkErr != nil || !checked.AlreadyReserved || checked.AlreadySettled {
			return zero, ErrProviderUnavailable
		}
		reservation = receipt.reservation
		maximum = reservation.MaxCostMicros
		receipt.consumed = true
	}
	budget.calls++
	budget.input += len(body)
	budget.spent += maximum
	response, err := e.client.Do(httpRequest)
	if err != nil {
		return zero, ErrProviderUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return zero, &ProviderStatusError{Status: response.StatusCode}
	}
	contents, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(contents) > 1<<20 || json.Unmarshal(contents, &zero) != nil {
		return responseEnvelope{}, ErrProviderResponse
	}
	if zero.Status != "completed" || len(zero.Output) == 0 || len(zero.Output) > 16 {
		return responseEnvelope{}, ErrProviderResponse
	}
	u := zero.Usage
	if u == nil || u.Input == nil || u.Output == nil || u.Total == nil || *u.Input < 0 || *u.Output < 0 || *u.Input > math.MaxInt64-*u.Output || *u.Total != *u.Input+*u.Output {
		return responseEnvelope{}, ErrProviderResponse
	}
	actual, costErr := modelCost(*u.Input, *u.Output, e.cfg)
	if costErr != nil {
		_ = e.ledger.Settle(reservation, math.MaxInt64)
		return responseEnvelope{}, ErrProviderResponse
	}
	if actual > maximum {
		_ = e.ledger.Settle(reservation, actual)
		return responseEnvelope{}, ErrProviderResponse
	}
	if *u.Input > inputUpper || *u.Output > int64(request.MaxOutputTokens) {
		return responseEnvelope{}, ErrProviderResponse
	}
	if err := e.ledger.Settle(reservation, actual); err != nil {
		return responseEnvelope{}, ErrProviderUnavailable
	}
	budget.spent -= maximum - actual
	budget.output += int(*u.Output)
	return zero, nil
}

func modelCost(input, output int64, cfg EngineConfig) (int64, error) {
	ceil := func(tokens, rate int64) (int64, error) {
		if tokens < 0 || rate <= 0 || tokens > (math.MaxInt64-999999)/rate {
			return 0, ErrGenerationBudget
		}
		return (tokens*rate + 999999) / 1000000, nil
	}
	i, err := ceil(input, cfg.InputMicrosPerMillion)
	if err != nil {
		return 0, err
	}
	o, err := ceil(output, cfg.OutputMicrosPerMillion)
	if err != nil || i > math.MaxInt64-o {
		return 0, ErrGenerationBudget
	}
	return i + o, nil
}

func strictJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return ErrProviderResponse
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return ErrProviderResponse
	}
	return nil
}

func schemaObject(properties map[string]any) map[string]any {
	required := make([]string, 0, len(properties))
	for key := range properties {
		required = append(required, key)
	}
	sort.Strings(required)
	return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}
func schemaArray(item any, maximum int) any {
	return map[string]any{"type": "array", "items": item, "maxItems": maximum}
}
func schemaString() any             { return map[string]any{"type": "string"} }
func schemaInteger() map[string]any { return map[string]any{"type": "integer", "minimum": 1} }
func schemaCount(maximum int) any {
	schema := schemaInteger()
	schema["maximum"] = maximum
	return schema
}
func schemaEnum(values ...string) any { return map[string]any{"type": "string", "enum": values} }
func draftSchema() any {
	// Official Structured Outputs supports numeric bounds and array maxItems:
	// https://developers.openai.com/api/docs/guides/structured-outputs?api-mode=responses
	citation := schemaObject(map[string]any{"file_id": schemaString(), "start_line": schemaInteger(), "line_count": schemaCount(10)})
	statement := schemaObject(map[string]any{"id": schemaString(), "kind": schemaEnum("purpose", "action", "mechanism", "result", "design", "handoff"), "text": schemaString(), "sources": schemaArray(citation, maxSources), "limit": schemaString()})
	scene := schemaObject(map[string]any{"id": schemaString(), "role": schemaEnum("purpose", "workflow", "design", "handoff"), "statement_ids": schemaArray(schemaString(), maxStatements)})
	return schemaObject(map[string]any{"statements": schemaArray(statement, maxStatements), "scenes": schemaArray(scene, maxScenes)})
}
func reviewSchema() any {
	decision := schemaObject(map[string]any{"statement_id": schemaString(), "accepted": map[string]any{"type": "boolean"}, "reason": schemaString()})
	return schemaObject(map[string]any{"accepted": map[string]any{"type": "boolean"}, "reason": schemaString(), "decisions": schemaArray(decision, maxStatements)})
}
func writerTools() []responseTool {
	return []responseTool{
		{Type: "function", Name: "list_paths", Description: "Find eligible pinned repository paths using ONE literal path substring, or empty string to list paths. Not natural-language search or a space-joined OR list. Listed paths alone are not evidence. Limit 1..40.", Strict: true, Parameters: schemaObject(map[string]any{"query": schemaString(), "limit": schemaCount(40)})},
		{Type: "function", Name: "read_file", Description: "Read an eligible repository source window with actual evidence ID and original line numbers. start_line>=1, line_count=1..80. Maximum 8192 text bytes.", Strict: true, Parameters: schemaObject(map[string]any{"path": schemaString(), "start_line": schemaInteger(), "line_count": schemaCount(80)})},
		{Type: "function", Name: "search_file", Description: "Find the first literal query match in one eligible file at or after start_line. Return matched_line and a window starting at that line, up to line_count lines/8192 text bytes, with actual evidence IDs and original numbers. If the byte cap hides the query, return a matching_line_exceeds_window error and matched_line. query=1..160 bytes; line_count=1..80. Same acquisition budget as read_file; no regex or global search.", Strict: true, Parameters: schemaObject(map[string]any{"path": schemaString(), "query": schemaString(), "start_line": schemaInteger(), "line_count": schemaCount(80)})},
		{Type: "function", Name: "finish_reading", Description: "Signal that existing inspected evidence is sufficient. No reads occur. The next dedicated turn requires submit_story with the full remaining initial output allocation.", Strict: true, Parameters: schemaObject(map[string]any{})},
	}
}

func storySubmissionTool() responseTool {
	return responseTool{Type: "function", Name: "submit_story", Description: "Submit final bounded source-grounded story for independent review. No further investigation calls follow.", Strict: true, Parameters: draftSchema()}
}
