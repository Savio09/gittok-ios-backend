package preview

import (
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/crawler"
)

// This file ranks claim candidates by product relevance and selects a bounded
// preview. Priority comes from the classifier's typed result (its product
// components and basis premises) and the inspected facts, never from a
// repository's name; lexical order only breaks genuinely equal priorities.
//
// Tiers, most relevant first:
//
//	0  the kind explanation
//	1  attributed purpose: the GitHub description, the product's own README
//	2  the classification's premises, product entry declarations, setup scripts
//	3  product declarations its inspected code imports; the primary language
//	4  other direct product declarations and constructs
//	5  secondary product components (their own ownership and role kept)
//	6  lower priority: development/peer/optional/indirect declarations,
//	   non-setup scripts, supporting-path facts (tooling, CI, tests, docs,
//	   examples) and READMEs that are not a component's own
const (
	tierKind = iota
	tierPurpose
	tierPremise
	tierImported
	tierProduct
	tierSecondary
	tierLow
)

// Bounded defaults; a shorter preview is valid.
const (
	maxClaims        = 24
	maxDependencies  = 8 // dependency claims in total
	maxProductDeps   = 6 // when secondary components also declare dependencies
	maxLowPriority   = 2 // tier 6 claims of any family
	maxScripts       = 6
	maxReadmePassage = 3
)

var setupScripts = map[string]bool{"start": true, "dev": true, "build": true, "serve": true, "test": true}

var lowCategories = map[string]bool{
	"devDependencies": true, "peerDependencies": true, "optionalDependencies": true,
	"require (indirect)": true, "dev-dependencies": true, "build-dependencies": true, "dev_dependencies": true,
}

type ranker struct {
	primary     string          // the deciding component of a resolved verdict
	hasPrimary  bool            // only a resolved verdict names a primary
	peers       map[string]bool // an abstention's candidates, ranked as equals
	premises    map[string]bool // witness keys of the primary basis
	imports     map[string][]crawler.Import
	anyPrimary  bool // some product-bearing fact lies outside supporting paths
	secondaries bool // a secondary component declares dependencies
	mainReadme  bool // some README lies outside supporting paths
}

func witnessKey(w crawler.Witness) string {
	if w.Kind == crawler.WitnessSpan {
		return "span " + strconv.Quote(w.Path) + " " + strconv.Itoa(w.Start) + " " + strconv.Itoa(w.End)
	}
	key := "field " + strconv.Quote(w.Path)
	for _, f := range w.Field {
		key += " " + strconv.Quote(f) // quoted segments cannot alias another path
	}
	return key
}

func newRanker(cls crawler.Classification, facts []crawler.Fact, imports []crawler.Import) *ranker {
	r := &ranker{premises: map[string]bool{}, imports: map[string][]crawler.Import{}}
	switch {
	case cls.Status == crawler.VerdictResolved && cls.Basis != nil:
		r.primary, r.hasPrimary = cls.Basis.Component, true
		for _, p := range cls.Basis.Premises {
			r.premises[witnessKey(p.Witness)] = true
		}
	case len(cls.Products) > 0:
		// Candidate order is not a product decision: an abstention's
		// candidates share product priority.
		r.peers = map[string]bool{}
		for _, p := range cls.Products {
			r.peers[p] = true
		}
		for _, o := range cls.Secondary {
			if r.peers[o.Component] && !o.Context {
				r.premises[witnessKey(o.Witness)] = true
			}
		}
	}
	for _, im := range imports {
		r.imports[im.Component] = append(r.imports[im.Component], im)
		if !crawler.SupportingPath(im.File) {
			r.anyPrimary = true
		}
	}
	// Competition requires positive product evidence at an actual outside
	// path. A root dependency table or README alone is not another product;
	// validated role evidence also covers products whose code has no imports.
	for _, observations := range [][]crawler.Observation{cls.Observations, cls.Secondary} {
		for _, o := range observations {
			if !o.Context && o.Absence == "" && o.Path != "" && !crawler.SupportingPath(o.Path) {
				r.anyPrimary = true
			}
		}
	}
	for _, f := range facts {
		switch f.Kind {
		case crawler.FactCommand, crawler.FactTarget, crawler.FactServerCall, crawler.FactGoPackage:
			if !crawler.SupportingPath(f.Witness.Path) {
				r.anyPrimary = true
			}
		}
		if f.Kind == crawler.FactReadme && !crawler.SupportingPath(f.Witness.Path) {
			r.mainReadme = true
		}
	}
	for _, f := range facts {
		if f.Kind == crawler.FactDependency && r.role(f) == tierSecondary {
			r.secondaries = true
		}
	}
	return r
}

// role places a fact's component: tierProduct for the resolved product, for
// an abstention's candidates, or — when the classifier named none — for any
// component; tierSecondary otherwise.
func (r *ranker) role(f crawler.Fact) int {
	switch {
	case r.hasPrimary:
		if f.Component == r.primary {
			return tierProduct
		}
		return tierSecondary
	case r.peers != nil:
		if r.peers[f.Component] {
			return tierProduct
		}
		return tierSecondary
	}
	return tierProduct
}

// supporting reports a supporting-path hint that should lower a fact.
func (r *ranker) supporting(f crawler.Fact) bool {
	return r.supportingFile(f.Component, f.Witness.Path)
}

// supportingFile applies the path hint to a file owned by component. Within a
// resolved product that itself lies under a tooling-like path, only paths
// below it count; and when no product-bearing fact lies outside supporting
// paths, the hint distinguishes nothing (the tooling is the product).
func (r *ranker) supportingFile(component, p string) bool {
	if r.hasPrimary && component == r.primary && crawler.SupportingPath(component+"/") {
		return crawler.SupportingPath(strings.TrimPrefix(p, component+"/"))
	}
	return r.anyPrimary && crawler.SupportingPath(p)
}

// imported reports whether inspected product code in the fact's component
// imports the declared module: an import path equal to it or below it at a
// path boundary. Imports from supporting files (helpers, scripts, tests) do
// not count.
func (r *ranker) imported(f crawler.Fact) bool {
	for _, im := range r.imports[f.Component] {
		if r.supportingFile(im.Component, im.File) {
			continue
		}
		if im.Path == f.Name || strings.HasPrefix(im.Path, f.Name+"/") {
			return true
		}
	}
	return false
}

func lowCategory(f crawler.Fact) bool {
	return f.Optional || lowCategories[f.Category] || strings.HasPrefix(f.Category, "project.optional-dependencies.")
}

// place assigns a fact candidate its tier and budget families. It reports
// false for a fact that must not be shown at all: a supporting tool's server
// call, which would stand in for the product's walkthrough.
func (r *ranker) place(f crawler.Fact, c *candidate) bool {
	role := r.role(f)
	support := r.supporting(f)
	premise := role == tierProduct && r.premises[witnessKey(f.Witness)]
	switch f.Kind {
	case crawler.FactDependency:
		c.dep = true
		c.productDep = role == tierProduct
		switch {
		case premise:
			c.tier = tierPremise
		case support || lowCategory(f):
			c.tier = tierLow
		case role == tierSecondary:
			c.tier = tierSecondary
		case r.imported(f):
			c.tier = tierImported
		default:
			c.tier = tierProduct
		}
	case crawler.FactScript:
		c.script = true
		switch {
		case support:
			c.tier = tierLow
		case role == tierSecondary:
			c.tier = tierSecondary
		case setupScripts[f.Name] || premise:
			c.tier = tierPremise
		default:
			c.tier = tierLow // lint/release/variant scripts are not setup commands
		}
	case crawler.FactCommand, crawler.FactTarget:
		switch {
		case support:
			c.tier = tierLow
		case role == tierSecondary:
			c.tier = tierSecondary
		default:
			c.tier = tierPremise
		}
	case crawler.FactServerCall:
		switch {
		case support:
			return false
		case premise:
			c.tier = tierPremise
		case role == tierSecondary:
			c.tier = tierSecondary
		default:
			c.tier = tierProduct
		}
	case crawler.FactReadme:
		c.readme = true
		// Supporting documentation describes the product only when nothing
		// else does.
		if r.mainReadme && crawler.SupportingPath(f.Witness.Path) {
			return false
		}
		own := path.Dir(f.Witness.Path) == f.Component || (f.Component == "" && !strings.Contains(f.Witness.Path, "/"))
		switch {
		case own && role == tierProduct && !support:
			c.tier = tierPurpose
		case own && !support:
			c.tier = tierSecondary
		default:
			c.tier = tierLow
		}
	}
	if c.tier == tierLow {
		c.low = true
	}
	return true
}

// metadataMetricsFrom renders the card's sourced metadata metrics. A value is
// shown only when the provider reported it; pushed_at is a push time, not a
// verified commit time.
func metadataMetricsFrom(in Input) []apimodel.Metric {
	r := in.Repo
	var out []apimodel.Metric
	add := func(key, label, field, value string) {
		f, ok := r.Metadata[field]
		if !ok || f.Null || f.Value != value || value == "" {
			return
		}
		out = append(out, apimodel.Metric{Key: key, Label: label, Value: value, Source: &apimodel.Source{Kind: apimodel.SourceMetadata, Ref: field, URL: metadataURL(r)}, Confidence: apimodel.Confident})
	}
	add("stars", "stars", "stargazers_count", strconv.Itoa(r.Stars))
	if r.License != "NOASSERTION" {
		add("license", "license", "license.spdx_id", r.License)
	}
	if !r.PushedAt.IsZero() {
		add("last_push", "last push reported by GitHub", "pushed_at", r.PushedAt.Format(time.RFC3339))
	}
	return out
}
