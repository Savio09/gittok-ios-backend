package config

import (
	"strings"
	"testing"
)

func TestLoadMissingTokenFails(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("PORT", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() succeeded with no GITHUB_TOKEN, want error")
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	t.Setenv("PORT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != defaultPort {
		t.Errorf("Port = %d, want %d", cfg.Port, defaultPort)
	}
	if cfg.GitHubToken != "test-token" {
		t.Errorf("GitHubToken = %q, want test-token", cfg.GitHubToken)
	}
}

func TestLoadPortOverride(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	t.Setenv("PORT", "9090")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090", cfg.Port)
	}
}

func TestLoadInvalidPort(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")

	for _, raw := range []string{"abc", "0", "70000", "-1"} {
		t.Setenv("PORT", raw)
		if _, err := Load(); err == nil {
			t.Errorf("Load() with PORT=%q succeeded, want error", raw)
		}
	}
}

func TestLoadPreviewSettingsDefaults(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "feed-token")
	t.Setenv("GITHUB_PUBLIC_TOKEN", "")
	t.Setenv("PREVIEW_CACHE_TTL", "")
	t.Setenv("PREVIEW_CACHE_DIR", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PublicToken != "" || cfg.PreviewCacheTTL != defaultPreviewCacheTTL || cfg.PreviewCacheDir != "" {
		t.Errorf("preview defaults: token set %v, ttl %v, dir %q", cfg.PublicToken != "", cfg.PreviewCacheTTL, cfg.PreviewCacheDir)
	}
}

func TestLoadPreviewSettingsOverrides(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "feed-token")
	t.Setenv("GITHUB_PUBLIC_TOKEN", "public-token")
	t.Setenv("PREVIEW_CACHE_TTL", "45m")
	t.Setenv("PREVIEW_CACHE_DIR", "/var/cache/gittok")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PublicToken != "public-token" || cfg.PreviewCacheTTL.Minutes() != 45 || cfg.PreviewCacheDir != "/var/cache/gittok" {
		t.Errorf("overrides not applied: ttl %v dir %q", cfg.PreviewCacheTTL, cfg.PreviewCacheDir)
	}
}

func TestLoadRejectsUnsafePreviewSettings(t *testing.T) {
	for name, env := range map[string]map[string]string{
		"feed token reused": {"GITHUB_PUBLIC_TOKEN": "feed-token"},
		"unparsable ttl":    {"PREVIEW_CACHE_TTL": "soon"},
		"zero ttl":          {"PREVIEW_CACHE_TTL": "0s"},
		"negative ttl":      {"PREVIEW_CACHE_TTL": "-5m"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("GITHUB_TOKEN", "feed-token")
			t.Setenv("GITHUB_PUBLIC_TOKEN", "")
			t.Setenv("PREVIEW_CACHE_TTL", "")
			for k, v := range env {
				t.Setenv(k, v)
			}
			if _, err := Load(); err == nil {
				t.Fatal("accepted")
			} else if strings.Contains(err.Error(), "feed-token") {
				t.Fatalf("error leaks a credential: %v", err)
			}
		})
	}
}
