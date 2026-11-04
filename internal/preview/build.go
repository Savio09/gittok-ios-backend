package preview

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"time"
	"unicode/utf8"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/crawler"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

// Input is acquired evidence, not proposed prose. The builder derives its own
// facts and classification, preventing caller-supplied labels from becoming claims.
type Input struct {
	Repo        githubclient.Repo
	Inspection  crawler.Inspection
	CapturedAt  time.Time
	Limitations []string
}

// Build is pure: no GitHub requests, environment, model calls or current clock.
func Build(in Input) apimodel.RepoPreview {
	r := in.Repo
	if !ValidIdentity(r.Owner, r.Name) {
		return apimodel.RepoPreview{Kind: apimodel.KindOther}
	}
	p := apimodel.RepoPreview{ID: r.Owner + "/" + r.Name, Owner: r.Owner, Name: r.Name, Slug: r.Owner + "/" + r.Name, Kind: apimodel.KindOther, GitHubURL: repoURL(r)}
	classification := crawler.Classify(in.Inspection)
	// The kind is displayed only with an explanation citing its whole basis.
	_, explained := kindClaim(in, classification)
	if classification.Status == crawler.VerdictResolved && explained {
		p.Kind = classification.Kind
	}
	for _, field := range []struct {
		key, value string
		dest       *string
	}{{"description", r.Description, &p.Description}, {"language", r.Language, &p.Language}, {"license.spdx_id", r.License, &p.License}} {
		f, ok := r.Metadata[field.key]
		if ok && !f.Null && f.Value == field.value && utf8.ValidString(f.Value) {
			*field.dest = field.value
		}
	}
	limits := append([]string(nil), in.Limitations...)
	limits = append(limits, classification.Limitations...)
	if classification.Status == crawler.VerdictResolved && !explained {
		limits = append(limits, "The project type could not be confirmed from citable evidence.")
	}
	if !in.Inspection.Revision.Resolved {
		limits = append(limits, "File citations are unavailable because the inspected revision was not pinned.")
	}
	if len(in.Inspection.Files) == 0 {
		limits = append(limits, "No selected file contents were available for inspection.")
	}
	all := candidates(in, classification)
	secondaries := false
	for _, c := range all {
		secondaries = secondaries || (c.dep && !c.productDep && c.tier == tierSecondary)
	}
	claims, omitted := selectClaims(all, secondaries)
	if omitted > 0 {
		limits = append(limits, "Some lower-priority declarations were omitted to keep the preview focused.")
	}
	p.Metrics = metadataMetricsFrom(in)
	for _, kind := range []apimodel.SectionKind{apimodel.SectionIdea, apimodel.SectionHow, apimodel.SectionStack, apimodel.SectionActivity, apimodel.SectionSetup} {
		section := apimodel.Section{Kind: kind, Title: sectionTitle(kind), EvidenceMode: apimodel.NotExecuted}
		for _, c := range claims {
			if c.section == kind {
				section.Claims = append(section.Claims, c.claim)
			}
		}
		if len(section.Claims) > 0 {
			p.Sections = append(p.Sections, section)
		}
	}
	if len(limits) > 0 {
		sort.Strings(limits)
		unique := make([]string, 0, len(limits))
		for _, s := range limits {
			if s != "" && (len(unique) == 0 || unique[len(unique)-1] != s) {
				unique = append(unique, s)
			}
		}
		p.Sections = append(p.Sections, apimodel.Section{Kind: apimodel.SectionDecision, Title: "Inspection limits", Body: strings.Join(unique, "\n"), EvidenceMode: apimodel.NotExecuted})
	}
	return p
}

func sectionTitle(kind apimodel.SectionKind) string {
	switch kind {
	case apimodel.SectionIdea:
		return "Repository descriptions"
	case apimodel.SectionHow:
		return "Inspected source constructs"
	case apimodel.SectionStack:
		return "Declared stack"
	case apimodel.SectionActivity:
		return "GitHub metadata"
	case apimodel.SectionSetup:
		return "Declared commands"
	}
	return "Inspection limits"
}

// ValidateClaim accepts only a claim this evidence and its fixed templates can
// produce. It checks full wording and provenance, not just path/quote existence.
// This audit operation is separate from Build's validated construction path.
func ValidateClaim(in Input, claim apimodel.Claim) error {
	if !ValidIdentity(in.Repo.Owner, in.Repo.Name) {
		return fmt.Errorf("invalid repository identity")
	}
	for _, c := range candidates(in, crawler.Classify(in.Inspection)) {
		if reflect.DeepEqual(c.claim, claim) {
			return nil
		}
	}
	return fmt.Errorf("claim is not supported by this evidence and its templates")
}
