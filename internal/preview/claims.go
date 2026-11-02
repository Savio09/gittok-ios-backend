package preview

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/crawler"
)

type candidate struct {
	section   apimodel.SectionKind
	claim     apimodel.Claim
	component string
	round     int
}

func metadataClaims(in Input) []candidate {
	r := in.Repo
	values := []struct {
		key, label, value string
		section           apimodel.SectionKind
	}{
		{"full_name", "Repository identity", r.Owner + "/" + r.Name, apimodel.SectionIdea},
		{"description", "GitHub description", r.Description, apimodel.SectionIdea},
		{"language", "GitHub primary language", r.Language, apimodel.SectionStack},
		{"license.spdx_id", "GitHub license identification", r.License, apimodel.SectionActivity},
		{"stargazers_count", "GitHub star count", strconv.Itoa(r.Stars), apimodel.SectionActivity},
	}
	if !r.PushedAt.IsZero() {
		values = append(values, struct {
			key, label, value string
			section           apimodel.SectionKind
		}{"pushed_at", "Last push reported by GitHub", r.PushedAt.Format(time.RFC3339), apimodel.SectionActivity})
	}
	var out []candidate
	for _, v := range values {
		field, ok := r.Metadata[v.key]
		if !ok || field.Null || field.Value != v.value || v.value == "" || !utf8.ValidString(v.value) || (v.key == "license.spdx_id" && v.value == "NOASSERTION") {
			continue
		}
		explanation := fmt.Sprintf("GitHub reports %s as %q.", v.key, v.value)
		if v.key == "description" {
			explanation = fmt.Sprintf("The repository's GitHub description states: %q.", v.value)
		}
		limit := "Provider metadata is mutable and does not verify implementation or runtime behavior."
		if !in.CapturedAt.IsZero() {
			limit = "Captured " + in.CapturedAt.UTC().Format(time.RFC3339) + ". " + limit
		}
		out = append(out, candidate{section: v.section, claim: apimodel.Claim{Label: v.label, Source: apimodel.Source{Kind: apimodel.SourceMetadata, Ref: v.key, URL: metadataURL(r)}, Explanation: explanation, Supports: "The value reported by GitHub for this repository.", Limit: limit, Confidence: apimodel.Confident}})
	}
	return out
}

func factClaim(in Input, f crawler.Fact) (candidate, bool) {
	source, excerpt, err := fileSource(in, f.Witness)
	if err != nil {
		return candidate{}, false
	}
	c := apimodel.Claim{Source: source, Excerpt: excerpt, Confidence: apimodel.Confident}
	section := apimodel.SectionHow
	where := "the repository root"
	if f.Component != "" {
		where = "component " + strconv.Quote(f.Component)
	}
	field := strings.Join(f.Witness.Field, " → ")
	switch f.Kind {
	case crawler.FactDependency:
		section = apimodel.SectionStack
		c.Label = "Declares " + f.Name
		c.Explanation = fmt.Sprintf("%q declares %q in %q", source.Ref, f.Name, f.Category)
		if f.Spec != "" {
			c.Explanation += " with declared specification " + strconv.Quote(f.Spec)
		}
		c.Explanation += "."
		if f.Optional {
			c.Explanation += " This dependency is declared optional."
		}
		c.Supports = "A dependency declaration in " + where + "."
		c.Limit = "A declaration does not prove installation, runtime use or repository-wide architecture."
	case crawler.FactScript, crawler.FactCommand:
		section = apimodel.SectionSetup
		c.Label = "Declares command " + f.Name
		c.Explanation = fmt.Sprintf("%q declares field %q as %q.", source.Ref, field, f.Spec)
		c.Supports = "A static command declaration in " + where + "."
		c.Limit = "The command was not executed; success and required environment were not verified."
	case crawler.FactTarget:
		c.Label = "Declares " + f.Name
		c.Explanation = fmt.Sprintf("%q contains the %q target declaration.", source.Ref, field)
		c.Supports = "An explicit target table in " + where + "."
		c.Limit = "The target was not built or executed."
	case crawler.FactServerCall:
		c.Label = "Contains net/http call"
		c.Explanation = fmt.Sprintf("%q contains a call to %q resolved to a net/http server entry point.", source.Ref, f.Name)
		c.Supports = "An inspected non-test source construct in " + where + "."
		c.Limit = "Static syntax does not establish reachability, successful execution or a running service."
	case crawler.FactReadme:
		if excerpt == "" {
			return candidate{}, false
		}
		section = apimodel.SectionIdea
		c.Label = "README passage"
		c.Explanation = fmt.Sprintf("%q states: %q.", source.Ref, excerpt)
		c.Supports = "An attributed documentation passage in " + where + "."
		c.Limit = "Repository documentation is an author statement; its described behavior was not verified."
	default:
		return candidate{}, false
	}
	return candidate{section: section, claim: c, component: f.Component}, true
}

func candidates(in Input) []candidate {
	out := metadataClaims(in)
	var files []candidate
	rounds := map[string]int{}
	for _, f := range crawler.CollectFacts(in.Inspection) {
		c, ok := factClaim(in, f)
		if !ok {
			continue
		}
		// Interleave fact families/components before taking second and later facts
		// from the same bucket, so one large dependency table cannot fill the preview.
		bucket := string(f.Kind) + "\x00" + f.Component
		c.round = rounds[bucket]
		rounds[bucket]++
		files = append(files, c)
	}
	sort.SliceStable(files, func(i, j int) bool {
		a, b := files[i], files[j]
		if a.round != b.round {
			return a.round < b.round
		}
		if a.section != b.section {
			return sectionRank(a.section) < sectionRank(b.section)
		}
		if a.component != b.component {
			return a.component < b.component
		}
		if a.claim.Source.Ref != b.claim.Source.Ref {
			return a.claim.Source.Ref < b.claim.Source.Ref
		}
		return a.claim.Label < b.claim.Label
	})
	return append(out, files...)
}

func sectionRank(k apimodel.SectionKind) int {
	switch k {
	case apimodel.SectionIdea:
		return 0
	case apimodel.SectionHow:
		return 1
	case apimodel.SectionStack:
		return 2
	case apimodel.SectionActivity:
		return 3
	case apimodel.SectionSetup:
		return 4
	default:
		return 5
	}
}
