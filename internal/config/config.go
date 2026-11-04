// Package config loads and validates the backend's runtime configuration from
// the environment. It is the single place that reads process environment
// variables, so the rest of the system depends on a typed Config value rather
// than scattering os.Getenv calls across packages.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds the validated runtime configuration for the GitTok backend.
type Config struct {
	// Port is the TCP port the HTTP server listens on.
	Port int
	// GitHubToken authenticates requests to the GitHub API. It is required:
	// without it the backend cannot ingest repositories, and GitHub's
	// unauthenticated rate limit is too low to be useful.
	GitHubToken string
	// PublicToken (GITHUB_PUBLIC_TOKEN) is the optional, separate credential
	// for public repository previews: a classic personal access token with no
	// OAuth scopes. Empty selects explicit anonymous mode. It is never the
	// feed's GitHubToken.
	PublicToken string
	// PreviewCacheTTL (PREVIEW_CACHE_TTL, a Go duration) bounds how long a
	// stored preview is served. PreviewCacheDir (PREVIEW_CACHE_DIR) names the
	// dedicated cache directory; empty selects the default.
	PreviewCacheTTL time.Duration
	PreviewCacheDir string
}

const (
	defaultPort            = 8080
	defaultPreviewCacheTTL = 30 * time.Minute
)

// Load reads configuration from the environment and validates it. It fails
// loudly — returning an error rather than falling back to a default — when a
// required value is missing or malformed, so a misconfigured process never
// boots into a half-working state.
func Load() (Config, error) {
	cfg := Config{Port: defaultPort}

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return Config{}, fmt.Errorf("GITHUB_TOKEN is required but not set")
	}
	cfg.GitHubToken = token

	if raw := os.Getenv("PORT"); raw != "" {
		port, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("PORT %q is not a valid integer: %w", raw, err)
		}
		if port < 1 || port > 65535 {
			return Config{}, fmt.Errorf("PORT %d is out of range (1-65535)", port)
		}
		cfg.Port = port
	}

	cfg.PublicToken = os.Getenv("GITHUB_PUBLIC_TOKEN")
	if cfg.PublicToken != "" && cfg.PublicToken == cfg.GitHubToken {
		return Config{}, fmt.Errorf("GITHUB_PUBLIC_TOKEN must be a separate credential from GITHUB_TOKEN")
	}
	cfg.PreviewCacheTTL = defaultPreviewCacheTTL
	if raw := os.Getenv("PREVIEW_CACHE_TTL"); raw != "" {
		ttl, err := time.ParseDuration(raw)
		if err != nil || ttl <= 0 {
			return Config{}, fmt.Errorf("PREVIEW_CACHE_TTL %q must be a positive duration such as 30m", raw)
		}
		cfg.PreviewCacheTTL = ttl
	}
	cfg.PreviewCacheDir = os.Getenv("PREVIEW_CACHE_DIR")

	return cfg, nil
}
