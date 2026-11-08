package config

import (
	"strings"
	"testing"
)

func walkthroughEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GITHUB_TOKEN", "feed")
	t.Setenv("WALKTHROUGH_ENABLED", "true")
	t.Setenv("OPENAI_API_KEY", "test-secret-value")
	t.Setenv("OPENAI_MODEL", "gpt-5.4-mini-2026-03-17")
	t.Setenv("OPENAI_INPUT_MICROS_PER_MILLION", "750000")
	t.Setenv("OPENAI_OUTPUT_MICROS_PER_MILLION", "4500000")
	t.Setenv("WALKTHROUGH_LEDGER_PATH", "/private/tmp/operator-owned/spend.json")
	t.Setenv("WALKTHROUGH_JOURNAL_PATH", "/private/tmp/operator-owned/jobs.json")
	t.Setenv("WALKTHROUGH_WINDOW_ID", "capstone-demo")
	t.Setenv("WALKTHROUGH_WINDOW_BUDGET_MICROS", "2000000")
}
func TestWalkthroughDisabledDoesNotRequireOrExposeProviderConfiguration(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "feed")
	t.Setenv("WALKTHROUGH_ENABLED", "")
	t.Setenv("OPENAI_API_KEY", "unused-secret")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Walkthrough.Enabled || cfg.Walkthrough.Engine.APIKey != "" {
		t.Fatal("unused provider enabled")
	}
}
func TestWalkthroughRequiresExplicitProviderPricingAndAccounting(t *testing.T) {
	for _, key := range []string{"OPENAI_API_KEY", "OPENAI_MODEL", "OPENAI_INPUT_MICROS_PER_MILLION", "OPENAI_OUTPUT_MICROS_PER_MILLION", "WALKTHROUGH_LEDGER_PATH", "WALKTHROUGH_JOURNAL_PATH", "WALKTHROUGH_WINDOW_ID", "WALKTHROUGH_WINDOW_BUDGET_MICROS"} {
		t.Run(key, func(t *testing.T) {
			walkthroughEnv(t)
			t.Setenv(key, "")
			_, err := Load()
			if err == nil {
				t.Fatal("missing configuration accepted")
			}
			if strings.Contains(err.Error(), "test-secret-value") {
				t.Fatal("key leaked")
			}
		})
	}
}
func TestWalkthroughConfiguredAndCapped(t *testing.T) {
	walkthroughEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Walkthrough.Enabled || cfg.Walkthrough.Engine.MaxCalls != 8 || cfg.Walkthrough.Engine.MaxOutputTokens != 12000 || cfg.Walkthrough.WindowBudgetMicros != 2000000 {
		t.Fatal("configuration lost")
	}
	for _, key := range []string{"WALKTHROUGH_MAX_CALLS", "WALKTHROUGH_MAX_OUTPUT_TOKENS", "WALKTHROUGH_MAX_INPUT_BYTES", "WALKTHROUGH_MAX_JOB_MICROS", "WALKTHROUGH_WINDOW_BUDGET_MICROS"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, "9223372036854775807")
			if _, err := Load(); err == nil {
				t.Fatal("unbounded setting accepted")
			}
		})
	}
}

func TestWalkthroughRevisionIsBoundedAndCanBeDisabled(t *testing.T) {
	walkthroughEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Walkthrough.Engine.MaxRevisions != 1 {
		t.Fatal("one bounded source revision was not configured")
	}
	t.Setenv("WALKTHROUGH_MAX_REVISIONS", "2")
	if _, err := Load(); err == nil {
		t.Fatal("unbounded revisions allowed")
	}
	t.Setenv("WALKTHROUGH_MAX_REVISIONS", "0")
	t.Setenv("WALKTHROUGH_MAX_CALLS", "2")
	cfg, err = Load()
	if err != nil || cfg.Walkthrough.Engine.MaxRevisions != 0 {
		t.Fatal("strict single-pass operation unavailable", err)
	}
}

func TestWalkthroughReasoningEffortIsExplicitAndValidated(t *testing.T) {
	walkthroughEnv(t)
	t.Setenv("OPENAI_REASONING_EFFORT", "low")
	cfg, err := Load()
	if err != nil || cfg.Walkthrough.Engine.ReasoningEffort != "low" {
		t.Fatal("reasoning choice missing", err)
	}
	t.Setenv("OPENAI_REASONING_EFFORT", "unbounded")
	if _, err := Load(); err == nil {
		t.Fatal("unknown effort accepted")
	}
}
