package config

import "testing"

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
