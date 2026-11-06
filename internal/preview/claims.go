package preview

import (
	"context"
	"encoding/json"
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
	section    apimodel.SectionKind
	claim      apimodel.Claim
	component  string
	round      int
	tier       int  // product relevance, most relevant first (see relevance.go)
	dep        bool // a dependency declaration (budgeted)
	productDep bool
	low        bool // lower priority (budgeted)
	script     bool
	readme     bool
}

func metadataClaims(in Input) []candidate {
	r := in.Repo
	values := []struct {
		key, label, value string
		section           apimodel.SectionKind
	}{
		// Card metadata (stars, license, last push) is rendered as sourced
		// metrics instead; identity is the card itself.
		{"description", "GitHub description", r.Description, apimodel.SectionIdea},
		{"language", "GitHub primary language", r.Language, apimodel.SectionStack},
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
		tier := tierPurpose
		if v.key == "language" {
			tier = tierImported
		}
		out = append(out, candidate{section: v.section, tier: tier, claim: apimodel.Claim{Label: v.label, Source: apimodel.Source{Kind: apimodel.SourceMetadata, Ref: v.key, URL: metadataURL(r)}, Explanation: explanation, Supports: "The value reported by GitHub for this repository.", Limit: limit, EvidenceStatus: apimodel.EvidenceSupported, Confidence: apimodel.Confident}})
	}
	return out
}

func factClaim(in Input, f crawler.Fact) (candidate, bool) {
	source, excerpt, err := fileSource(in, f.Witness)
	if err != nil {
		return candidate{}, false
	}
	c := apimodel.Claim{Source: source, Excerpt: excerpt, EvidenceStatus: apimodel.EvidenceSupported, Confidence: apimodel.Confident}
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

// claimKey is the equality definition for claim deduplication: every field of
// the claim, with AdditionalSources in its unambiguous JSON encoding.
type claimKey struct {
	label, excerpt, explanation, supports, limit string
	source                                       apimodel.Source
	confidence                                   apimodel.Confidence
	evidenceStatus                               apimodel.EvidenceStatus
	stance                                       apimodel.Stance
	additional                                   string
}

func keyOf(c apimodel.Claim) claimKey {
	k := claimKey{label: c.Label, excerpt: c.Excerpt, explanation: c.Explanation, supports: c.Supports, limit: c.Limit, source: c.Source, confidence: c.Confidence, evidenceStatus: c.EvidenceStatus, stance: c.Stance}
	if len(c.AdditionalSources) > 0 {
		b, _ := json.Marshal(c.AdditionalSources) // a []Source of strings always encodes
		k.additional = string(b)
	}
	return k
}

func candidates(in Input, cls crawler.Classification) []candidate {
	return candidatesContext(context.Background(), in, cls)
}

func candidatesContext(ctx context.Context, in Input, cls crawler.Classification) []candidate {
	var out []candidate
	if kc, ok := kindClaim(in, cls); ok {
		out = append(out, kc) // the reserved explanation leads the preview
	}
	out = append(out, metadataClaims(in)...)
	var files []candidate
	rounds := map[string]int{}
	seen := map[claimKey]bool{}
	facts := crawler.CollectFactsContext(ctx, in.Inspection)
	rank := newRanker(cls, facts, crawler.CollectImportsContext(ctx, in.Inspection))
	for _, f := range facts {
		if ctx.Err() != nil {
			return metadataClaims(in)
		}
		c, ok := factClaim(in, f)
		if !ok || !rank.place(f, &c) {
			continue
		}
		if k := keyOf(c.claim); seen[k] {
			continue
		} else {
			seen[k] = true
		}
		// Within a tier, interleave fact families/components before taking
		// later facts from the same bucket, so one table cannot fill the tier.
		bucket := strconv.Itoa(c.tier) + "\x00" + string(f.Kind) + "\x00" + f.Component
		c.round = rounds[bucket]
		rounds[bucket]++
		files = append(files, c)
	}
	files = append(out, files...)
	sort.SliceStable(files, func(i, j int) bool {
		a, b := files[i], files[j]
		if a.tier != b.tier {
			return a.tier < b.tier
		}
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
	return files
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
