package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/Savio09/gittok-ios-backend/internal/walkthrough"
)

// Walkthrough is opt-in: configuring a key alone cannot silently start paid
// work. Accounting must already exist from the operator's explicit init step.
type Walkthrough struct {
	Enabled            bool
	Engine             walkthrough.EngineConfig
	LedgerPath         string
	JournalPath        string
	WindowID           string
	WindowBudgetMicros int64
}

func loadWalkthrough() (Walkthrough, error) {
	var c Walkthrough
	raw := os.Getenv("WALKTHROUGH_ENABLED")
	if raw == "" || raw == "false" {
		return c, nil
	}
	if raw != "true" {
		return c, fmt.Errorf("WALKTHROUGH_ENABLED must be true or false")
	}
	c.Enabled = true
	required := func(name string) (string, error) {
		v := os.Getenv(name)
		if strings.TrimSpace(v) == "" || strings.IndexFunc(v, unicode.IsControl) >= 0 {
			return "", fmt.Errorf("%s must be configured without control characters", name)
		}
		return v, nil
	}
	var err error
	if c.Engine.APIKey, err = required("OPENAI_API_KEY"); err != nil {
		return Walkthrough{}, err
	}
	if c.Engine.Model, err = required("OPENAI_MODEL"); err != nil {
		return Walkthrough{}, err
	}
	if len(c.Engine.Model) > 100 || strings.ContainsAny(c.Engine.Model, " /\\") {
		return Walkthrough{}, fmt.Errorf("OPENAI_MODEL must be a model identifier")
	}
	if c.LedgerPath, err = required("WALKTHROUGH_LEDGER_PATH"); err != nil {
		return Walkthrough{}, err
	}
	if c.JournalPath, err = required("WALKTHROUGH_JOURNAL_PATH"); err != nil {
		return Walkthrough{}, err
	}
	for _, p := range []string{c.LedgerPath, c.JournalPath} {
		if !filepath.IsAbs(p) || filepath.Clean(p) != p {
			return Walkthrough{}, fmt.Errorf("walkthrough control paths must be clean absolute file paths")
		}
	}
	controls := map[string]bool{}
	for _, p := range []string{c.LedgerPath, c.JournalPath} {
		for _, suffix := range []string{"", ".lock", ".pending"} {
			if controls[p+suffix] {
				return Walkthrough{}, fmt.Errorf("walkthrough control paths must be distinct")
			}
			controls[p+suffix] = true
		}
	}
	if c.WindowID, err = required("WALKTHROUGH_WINDOW_ID"); err != nil {
		return Walkthrough{}, err
	}
	if len(c.WindowID) > 128 {
		return Walkthrough{}, fmt.Errorf("WALKTHROUGH_WINDOW_ID is too long")
	}
	value := func(name string, fallback, low, high int64) (int64, error) {
		raw := os.Getenv(name)
		if raw == "" {
			if fallback > 0 {
				return fallback, nil
			}
			return 0, fmt.Errorf("%s is required", name)
		}
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v < low || v > high {
			return 0, fmt.Errorf("%s is outside its supported bounds", name)
		}
		return v, nil
	}
	if c.Engine.InputMicrosPerMillion, err = value("OPENAI_INPUT_MICROS_PER_MILLION", 0, 1, 1_000_000_000); err != nil {
		return Walkthrough{}, err
	}
	if c.Engine.OutputMicrosPerMillion, err = value("OPENAI_OUTPUT_MICROS_PER_MILLION", 0, 1, 1_000_000_000); err != nil {
		return Walkthrough{}, err
	}
	if c.WindowBudgetMicros, err = value("WALKTHROUGH_WINDOW_BUDGET_MICROS", 0, 1, 1_000_000_000); err != nil {
		return Walkthrough{}, err
	}
	if c.Engine.MaxJobMicros, err = value("WALKTHROUGH_MAX_JOB_MICROS", 500_000, 1, 1_000_000); err != nil {
		return Walkthrough{}, err
	}
	calls, err := value("WALKTHROUGH_MAX_CALLS", 8, 2, 16)
	if err != nil {
		return Walkthrough{}, err
	}
	c.Engine.MaxCalls = int(calls)
	revisions, err := value("WALKTHROUGH_MAX_REVISIONS", 1, 0, 1)
	if err != nil {
		return Walkthrough{}, err
	}
	c.Engine.MaxRevisions = int(revisions)
	if revisions == 1 && calls < 4 {
		return Walkthrough{}, fmt.Errorf("a reviewed revision requires at least four total model calls")
	}
	input, err := value("WALKTHROUGH_MAX_INPUT_BYTES", 512<<10, 4096, 2<<20)
	if err != nil {
		return Walkthrough{}, err
	}
	c.Engine.MaxInputBytes = int(input)
	output, err := value("WALKTHROUGH_MAX_OUTPUT_TOKENS", 12000, 1024, 32000)
	if err != nil {
		return Walkthrough{}, err
	}
	c.Engine.MaxOutputTokens = int(output)
	return c, nil
}
