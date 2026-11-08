package config

import (
	"testing"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/analysis"
)

var analysisEnvs = []string{"ANALYSIS_TIMEOUT", "ANALYSIS_MAX_TREE_ENTRIES", "ANALYSIS_MAX_DEPTH", "ANALYSIS_MAX_FILES", "ANALYSIS_MAX_FILE_BYTES", "ANALYSIS_MAX_TOTAL_BYTES", "ANALYSIS_MAX_API_CALLS", "ANALYSIS_MAX_RESPONSE_BYTES"}

func resetAnalysisEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GITHUB_TOKEN", "test-only-feed")
	t.Setenv("GITHUB_PUBLIC_TOKEN", "")
	for _, name := range analysisEnvs {
		t.Setenv(name, "")
	}
}

func TestAnalysisLimitsDefaultsAndOverrides(t *testing.T) {
	resetAnalysisEnv(t)
	cfg, err := Load()
	if err != nil || cfg.AnalysisLimits != analysis.DefaultLimits() {
		t.Fatalf("limits=%+v err=%v", cfg.AnalysisLimits, err)
	}
	for name, value := range map[string]string{
		"ANALYSIS_TIMEOUT": "2s", "ANALYSIS_MAX_TREE_ENTRIES": "23", "ANALYSIS_MAX_DEPTH": "3", "ANALYSIS_MAX_FILES": "2",
		"ANALYSIS_MAX_FILE_BYTES": "32", "ANALYSIS_MAX_TOTAL_BYTES": "48", "ANALYSIS_MAX_API_CALLS": "9", "ANALYSIS_MAX_RESPONSE_BYTES": "4096",
	} {
		t.Setenv(name, value)
	}
	cfg, err = Load()
	want := analysis.Limits{Timeout: 2 * time.Second, MaxTreeEntries: 23, MaxDepth: 3, MaxFiles: 2, MaxFileBytes: 32, MaxTotalBytes: 48, MaxAPICalls: 9, MaxResponseBytes: 4096}
	if err != nil || cfg.AnalysisLimits != want {
		t.Fatalf("limits=%+v err=%v", cfg.AnalysisLimits, err)
	}
}

func TestAnalysisLimitsRejectInvalidAndExcessiveEnvironment(t *testing.T) {
	for _, name := range analysisEnvs {
		for _, raw := range []string{"0", "-1", "invalid", "99999999999999999999999"} {
			t.Run(name+raw, func(t *testing.T) {
				resetAnalysisEnv(t)
				t.Setenv(name, raw)
				if _, err := Load(); err == nil {
					t.Fatal("accepted invalid limit")
				}
			})
		}
	}
	for name, raw := range map[string]string{"ANALYSIS_TIMEOUT": "1h", "ANALYSIS_MAX_FILES": "100000", "ANALYSIS_MAX_TREE_ENTRIES": "100000", "ANALYSIS_MAX_FILE_BYTES": "1073741824", "ANALYSIS_MAX_TOTAL_BYTES": "1073741824", "ANALYSIS_MAX_API_CALLS": "10000", "ANALYSIS_MAX_RESPONSE_BYTES": "1073741824", "ANALYSIS_MAX_DEPTH": "100000"} {
		t.Run(name+"ceiling", func(t *testing.T) {
			resetAnalysisEnv(t)
			t.Setenv(name, raw)
			if _, err := Load(); err == nil {
				t.Fatal("accepted excessive limit")
			}
		})
	}
}
