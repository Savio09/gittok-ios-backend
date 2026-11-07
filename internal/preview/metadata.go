package preview

import (
	"fmt"
	"strconv"
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
	kind := inferKind(r)
	status := apimodel.EvidenceInferred
	if kind == apimodel.KindOther {
		status = apimodel.EvidenceUnknown
	}
	return apimodel.RepoPreview{
		ID:                 r.FullName,
		GitHubID:           r.ID,
		Owner:              r.Owner,
		Name:               r.Name,
		Slug:               r.FullName,
		Kind:               kind,
		KindEvidenceStatus: status,
		EvidenceMode:       apimodel.NotExecuted,
		Language:           r.Language,
		Description:        r.Description,
		GitHubURL:          r.HTMLURL,
		License:            r.License,
		Metrics:            metadataMetrics(r),
	}
}

// reportedMetadataValue requires the retained provider field to establish the
// value. A zero-valued Go field alone cannot distinguish missing from reported.
func reportedMetadataValue(r githubclient.Repo, field, value string) bool {
	f, ok := r.Metadata[field]
	return ok && !f.Null && f.Value == value && value != ""
}

// metadataMetrics preserves feed display formats and legacy keys, but uses the
// same provider-value check as the analyzed preview before marking support.
func metadataMetrics(r githubclient.Repo) []apimodel.Metric {
	var metrics []apimodel.Metric
	add := func(key, label, field, raw, display string) {
		if !reportedMetadataValue(r, field, raw) {
			return
		}
		metrics = append(metrics, apimodel.Metric{
			Key: key, Label: label, Value: display,
			Source:     &apimodel.Source{Kind: apimodel.SourceMetadata, Ref: field},
			Confidence: apimodel.Confident, EvidenceStatus: apimodel.EvidenceSupported,
		})
	}
	add("stars", "stars", "stargazers_count", strconv.Itoa(r.Stars), formatCount(r.Stars))
	if !r.PushedAt.IsZero() {
		// Keep the legacy key; pushed_at reports a push, not a commit.
		add("last_commit", "since last push", "pushed_at", r.PushedAt.Format(time.RFC3339), humanizeDuration(time.Since(r.PushedAt)))
	}
	if r.License != "NOASSERTION" {
		add("license", "license", "license.spdx_id", r.License, r.License)
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
