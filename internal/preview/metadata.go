package preview

import (
	"fmt"
	"strings"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

// FromMetadata builds a metadata-level RepoPreview from GitHub repository
// metadata. It fills identity, a classified kind, and the activity metrics that
// come straight from the API.
//
// The evidence layer — claims, fit, walkthrough sections, media — is left empty
// here. It is produced later by the analysis pipeline, and the schema
// deliberately lets a preview carry only what has actually been determined, so
// a metadata-only preview never implies analysis that has not run.
func FromMetadata(r githubclient.Repo) apimodel.RepoPreview {
	return apimodel.RepoPreview{
		ID:          r.FullName,
		Owner:       r.Owner,
		Name:        r.Name,
		Slug:        r.FullName,
		Kind:        inferKind(r),
		Language:    r.Language,
		Description: r.Description,
		GitHubURL:   r.HTMLURL,
		License:     r.License,
		Metrics:     metadataMetrics(r),
	}
}

// metadataMetrics turns the raw numbers into the display metrics the feed shows.
// Each is sourced to the API field it came from and marked confident, since it
// is reported directly by GitHub rather than inferred.
func metadataMetrics(r githubclient.Repo) []apimodel.Metric {
	metrics := []apimodel.Metric{
		{
			Key:        "stars",
			Label:      "stars",
			Value:      formatCount(r.Stars),
			Source:     &apimodel.Source{Kind: apimodel.SourceMetadata, Ref: "stargazers_count"},
			Confidence: apimodel.Confident,
		},
	}
	if !r.PushedAt.IsZero() {
		metrics = append(metrics, apimodel.Metric{
			Key:        "last_commit",
			Label:      "since last commit",
			Value:      humanizeDuration(time.Since(r.PushedAt)),
			Source:     &apimodel.Source{Kind: apimodel.SourceMetadata, Ref: "pushed_at"},
			Confidence: apimodel.Confident,
		})
	}
	if r.License != "" {
		metrics = append(metrics, apimodel.Metric{
			Key:        "license",
			Label:      "license",
			Value:      r.License,
			Source:     &apimodel.Source{Kind: apimodel.SourceMetadata, Ref: "license.spdx_id"},
			Confidence: apimodel.Confident,
		})
	}
	return metrics
}

// inferKind classifies a repo for the UI badge. Topics are the strongest signal
// (a repo that calls itself a CLI, prompt, or shell tool is one); language is
// the fallback. It is a coarse heuristic, honest about being metadata-only.
func inferKind(r githubclient.Repo) apimodel.RepoKind {
	for _, t := range r.Topics {
		if isCLITopic(strings.ToLower(t)) {
			return apimodel.KindCLI
		}
	}
	switch strings.ToLower(r.Language) {
	case "typescript", "javascript", "vue", "svelte", "html", "css":
		return apimodel.KindFrontend
	case "go", "rust", "python", "ruby", "java", "c#", "elixir", "kotlin", "scala", "php":
		return apimodel.KindBackend
	default:
		return apimodel.KindOther
	}
}

// cliMarkers are topic tokens that mark a command-line tool. A topic matches if
// it equals a marker or has a hyphen-separated part that does (so "fish-prompt"
// and "shell-prompt" match via "prompt"). "cli" is matched as a whole token
// only, to avoid false hits like "client".
var cliMarkers = map[string]bool{
	"cli": true, "tui": true, "terminal": true, "shell": true,
	"prompt": true, "repl": true, "command-line": true,
	"command-line-tool": true, "command-line-interface": true, "cli-tool": true,
}

func isCLITopic(topic string) bool {
	if cliMarkers[topic] {
		return true
	}
	for _, part := range strings.Split(topic, "-") {
		if cliMarkers[part] {
			return true
		}
	}
	return false
}

// formatCount renders a star count compactly: 18432 -> "18.4k", 134045 ->
// "134k", 1_250_000 -> "1.2m".
func formatCount(n int) string {
	switch {
	case n >= 1_000_000:
		return trimDecimal(float64(n)/1_000_000) + "m"
	case n >= 1000:
		return trimDecimal(float64(n)/1000) + "k"
	default:
		return fmt.Sprintf("%d", n)
	}
}

// trimDecimal formats to one decimal place, dropping a trailing ".0".
func trimDecimal(f float64) string {
	s := fmt.Sprintf("%.1f", f)
	return strings.TrimSuffix(s, ".0")
}

// humanizeDuration renders an age coarsely: "today", "3d", "2w", "5mo", "1y".
func humanizeDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	switch {
	case days <= 0:
		return "today"
	case days < 14:
		return fmt.Sprintf("%dd", days)
	case days < 60:
		return fmt.Sprintf("%dw", days/7)
	case days < 365:
		return fmt.Sprintf("%dmo", days/30)
	default:
		return fmt.Sprintf("%dy", days/365)
	}
}
