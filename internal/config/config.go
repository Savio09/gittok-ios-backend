// Package config loads and validates the backend's runtime configuration from
// the environment. It is the single place that reads process environment
// variables, so the rest of the system depends on a typed Config value rather
// than scattering os.Getenv calls across packages.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds the validated runtime configuration for the GitTok backend.
type Config struct {
	// Port is the TCP port the HTTP server listens on.
	Port int
	// GitHubToken authenticates requests to the GitHub API. It is required:
	// without it the backend cannot ingest repositories, and GitHub's
	// unauthenticated rate limit is too low to be useful.
	GitHubToken string
}

const defaultPort = 8080

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

	return cfg, nil
}
