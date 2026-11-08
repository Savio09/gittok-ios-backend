package preview

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/crawler"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

// This file renders the explanation of a resolved kind from the classifier's
// typed basis. Each premise is stated by a fixed template for its witness
// construct or absence kind (observation prose is never copied) and cited by
// the evidence it rests on: a consumed file (at its verified lines when
// known) or the recorded tree request for listing facts. The claim's sources
// together cover the whole basis; if any premise cannot be stated and cited,
// no explanation is produced and the kind is not displayed.

var kindNames = map[apimodel.RepoKind]string{
	apimodel.KindFrontend: "a frontend",
	apimodel.KindBackend:  "a backend",
	apimodel.KindLibrary:  "a library",
	apimodel.KindCLI:      "a CLI",
	apimodel.KindMobile:   "a mobile app",
}

// kindClaim explains a resolved classification, or reports false when the
// verdict is not resolved or some premise cannot be represented and cited.
func kindClaim(in Input, cls crawler.Classification) (candidate, bool) {
	b := cls.Basis
	name, known := kindNames[cls.Kind]
	if cls.Status != crawler.VerdictResolved || b == nil || !known || b.Kind != cls.Kind || len(b.Premises) == 0 {
		return candidate{}, false
	}
	var sources []apimodel.Source
	var excerpt string
	add := func(src []apimodel.Source) {
		for _, s := range src {
			dup := false
			for _, have := range sources {
				dup = dup || have == s
			}
			if !dup {
				sources = append(sources, s)
			}
		}
	}
	phrases, srcs, quote, ok := premisesOf(in, *b)
	if !ok {
		return candidate{}, false
	}
	add(srcs)
	excerpt = quote
	where := "the inspected files"
	if b.Component != "" {
		where = "component " + strconv.Quote(b.Component) + "'s inspected files"
	}
	text := "Classified as " + name + " from " + where + ": " + strings.Join(phrases, "; ") + "."
	limit := "An inference from a bounded inspection; the code was not run, and files outside the inspection could change it."
	if b.ByComparison {
		tree, ok := treeSource(in, "git/trees/"+in.Inspection.Revision.Ref)
		if !ok || !in.Inspection.Scope.Complete() {
			return candidate{}, false
		}
		add([]apimodel.Source{tree})
		var others []string
		allSame := true
		for _, other := range b.Compared {
			if _, known := kindNames[other.Kind]; !known || len(other.Premises) == 0 {
				return candidate{}, false
			}
			if _, srcs, _, ok := premisesOf(in, other); ok {
				add(srcs)
			} else {
				return candidate{}, false
			}
			label := "the repository root"
			if other.Component != "" {
				label = strconv.Quote(other.Component)
			}
			others = append(others, label+" ("+string(other.Kind)+")")
			allSame = allSame && other.Kind == b.Kind
		}
		// Each role-bearing component set aside as supporting is a premise of
		// the comparison: state and cite why, or withhold the kind.
		var excluded []string
		for _, e := range b.Excluded {
			if len(e.Premises) == 0 {
				return candidate{}, false
			}
			var reasons []string
			for _, o := range e.Premises {
				phrase, srcs, _, ok := premise(in, o)
				if !ok {
					return candidate{}, false
				}
				reasons = append(reasons, phrase)
				add(srcs)
			}
			role := "role not determined"
			if e.Kind != "" {
				if _, known := kindNames[e.Kind]; !known || len(e.RolePremises) == 0 {
					return candidate{}, false
				}
				_, roleSources, _, ok := premisesOf(in, crawler.Basis{Premises: e.RolePremises})
				if !ok {
					return candidate{}, false
				}
				add(roleSources)
				role = string(e.Kind)
			}
			excluded = append(excluded, "component "+strconv.Quote(e.Component)+" ("+role+") because "+strings.Join(reasons, " and "))
		}
		if len(excluded) > 0 {
			text += " Set aside as supporting: " + strings.Join(excluded, "; ") + "."
		}
		switch {
		case len(others) == 0:
			text += " It is the only product component in the complete file tree."
		case allSame:
			text += " Every other product component in the complete file tree has the same role: " + strings.Join(others, ", ") + "."
		default:
			text += " The other product components in the complete file tree are " + strings.Join(others, ", ") + "; an executable product outranks importable libraries."
		}

	}
	label := "Why this is " + name
	c := apimodel.Claim{
		Label:          label,
		Source:         sources[0],
		Excerpt:        excerpt,
		Explanation:    text,
		Supports:       "GitTok's classification of this repository from the cited premises.",
		Limit:          limit,
		Confidence:     apimodel.Uncertain,
		EvidenceStatus: apimodel.EvidenceInferred,
	}
	if len(sources) > 1 {
		c.AdditionalSources = sources[1:]
	}
	if len(c.Excerpt) > 1024 {
		c.Excerpt = ""
	}
	return candidate{section: apimodel.SectionIdea, claim: c, component: b.Component}, true
}

// premisesOf states and cites every premise of one basis. The quote is the
// first premise's exact excerpt, when its source has one.
func premisesOf(in Input, b crawler.Basis) (phrases []string, sources []apimodel.Source, quote string, ok bool) {
	for i, p := range b.Premises {
		phrase, srcs, q, ok := premise(in, p)
		if !ok || len(srcs) == 0 {
			return nil, nil, "", false
		}
		if i == 0 {
			quote = q
		}
		phrases = append(phrases, phrase)
		sources = append(sources, srcs...)
	}
	return phrases, sources, quote, true
}

// premise renders one observation's witness. Unknown constructs or untyped
// absences cannot be represented truthfully, so they fail.
func premise(in Input, o crawler.Observation) (string, []apimodel.Source, string, bool) {
	w := o.Witness
	switch w.Kind {
	case crawler.WitnessField:
		src, q, err := fileSource(in, w)
		if err != nil {
			return "", nil, "", false
		}
		field := strconv.Quote(strings.Join(w.Field, " → "))
		phrase := strconv.Quote(w.Path) + " declares " + field
		if w.Value != "" {
			phrase += " as " + strconv.Quote(w.Value)
		}
		return phrase, []apimodel.Source{src}, q, true
	case crawler.WitnessSpan:
		src, q, err := fileSource(in, w)
		if err != nil || src.Lines == "" {
			return "", nil, "", false
		}
		var phrase string
		switch w.Construct {
		case "go.package-main":
			phrase = strconv.Quote(w.Path) + " declares package main"
		case "go.package":
			phrase = strconv.Quote(w.Path) + " declares package " + strconv.Quote(w.Quote)
		case "go.server-call":
			phrase = strconv.Quote(w.Path) + " calls a net/http server entry point"
		case "dockerfile.expose":
			phrase = strconv.Quote(w.Path) + " contains an EXPOSE instruction"
		default:
			return "", nil, "", false
		}
		return phrase, []apimodel.Source{src}, q, true
	case crawler.WitnessListing:
		return listingPremise(in, w)
	case crawler.WitnessScope:
		return scopePremise(in, o)
	}
	return "", nil, "", false
}

var listingGloss = map[string]string{
	"supporting-path":  "which lies under an example, documentation or tooling directory",
	"entry.index-html": "a web app entry page",
	"entry.src-app":    "an app root component",
	"entry.src-main":   "an app bootstrap file",
	"entry.app-config": "a web app build configuration",
}

func listingPremise(in Input, w crawler.Witness) (string, []apimodel.Source, string, bool) {
	var sources []apimodel.Source
	quoted := make([]string, len(w.Paths))
	for i, p := range w.Paths {
		s, ok := treeSource(in, p)
		if !ok {
			return "", nil, "", false
		}
		sources = append(sources, s)
		quoted[i] = strconv.Quote(p)
	}
	listed := "the file tree lists " + strings.Join(quoted, " and ")
	switch w.Construct {
	case "native-shells":
		return listed + ", Android and iOS app directories", sources, "", true
	case "cargo.implicit-bin", "cargo.implicit-lib":
		// The target exists because the owning manifest enables discovery.
		manifest, _, err := fileSource(in, crawler.Witness{Kind: crawler.WitnessField, Path: w.Path})
		if err != nil {
			return "", nil, "", false
		}
		target := "binary"
		if w.Construct == "cargo.implicit-lib" {
			target = "library"
		}
		return listed + ", Cargo's implicit " + target + " target for " + strconv.Quote(w.Path), append(sources, manifest), "", true
	}
	if gloss, ok := listingGloss[w.Construct]; ok {
		return listed + ", " + gloss, sources, "", true
	}
	return "", nil, "", false
}

func scopePremise(in Input, o crawler.Observation) (string, []apimodel.Source, string, bool) {
	w := o.Witness
	var files []apimodel.Source
	var goFiles int
	dir := ""
	for _, p := range w.Paths {
		s, _, err := fileSource(in, crawler.Witness{Kind: crawler.WitnessField, Path: p})
		if err != nil {
			return "", nil, "", false
		}
		files = append(files, s)
		if strings.HasSuffix(p, ".go") {
			goFiles++
			if i := strings.LastIndexByte(p, '/'); i >= 0 {
				dir = p[:i]
			}
		}
	}
	manifest := ""
	if len(w.Paths) > 0 {
		manifest = strconv.Quote(w.Paths[0])
	}
	// An absence the listing establishes needs the recorded tree and a listing
	// that covers the depth the premise relies on; a truncated or too-shallow
	// listing cannot show that something is missing.
	depth := w.Depth
	if dir != "" && o.Absence == crawler.AbsenceGoServer {
		depth = strings.Count(dir, "/") + 2
	} else if depth == 0 {
		depth = 1
	}
	withTree := func(phrase string) (string, []apimodel.Source, string, bool) {
		tree, ok := treeSource(in, "git/trees/"+in.Inspection.Revision.Ref)
		covered := in.Inspection.Scope.Covers(depth)
		if w.Construct == "complete-listing" {
			covered = in.Inspection.Scope.Complete()
		}
		if !ok || !covered {
			return "", nil, "", false
		}
		return phrase, append(files, tree), "", true
	}
	switch o.Absence {
	case crawler.AbsenceGoServer:
		where := "the module root"
		if dir != "" {
			where = strconv.Quote(dir)
		}
		return withTree(fmt.Sprintf("all %d non-test Go files of the command package in %s, and go.mod, were read in full: they contain no net/http server call and require no recognised server framework", goFiles, where))
	case crawler.AbsenceGoCommand:
		// A depth cap can omit commands beyond the conventional entry points.
		// Keep the display conservative, and describe only the cited package
		// clauses: a separate module may own other files listed under cmd/.
		if !in.Inspection.Scope.Complete() {
			return "", nil, "", false
		}
		return withTree(fmt.Sprintf("%s was read in full; the package clauses of the %d inspected Go files at the module root and in this module's cmd/ scope declare no package main", manifest, len(w.Paths)-1))
	case crawler.AbsenceCargoWebFramework:
		return manifest + " was read in full and declares no recognised web-framework dependency", files, "", true
	case crawler.AbsenceCargoBinary:
		return manifest + " was read in full and declares no binary target", files, "", true
	case crawler.AbsenceCargoBinaryListing:
		return withTree(manifest + " was read in full and declares no [[bin]] table, and the inspected file tree shows no implicit binary entry")
	case crawler.AbsenceNPMCommandAndShell:
		return withTree(manifest + " was read in full and declares no executable command, and the inspected file tree shows no app shell")
	case crawler.AbsenceNPMCommandAndAppEntry:
		return withTree(manifest + " was read in full and declares no executable command, and the inspected file tree shows no app entry for its runtime framework")
	case crawler.AbsencePythonEntryPoints:
		return manifest + " was read in full and declares no command or GUI entry points", files, "", true
	}
	return "", nil, "", false
}

// treeSource cites the recorded tree request for ref (a listed path, or the
// tree itself). The request must be the one GetTree makes for this canonical
// repository at the inspected commit; a listed path must be in the listing.
func treeSource(in Input, ref string) (apimodel.Source, bool) {
	rev := in.Inspection.Revision
	if !rev.Resolved || !githubclient.ValidCommitID(rev.Ref) || !ValidIdentity(in.Repo.Owner, in.Repo.Name) {
		return apimodel.Source{}, false
	}
	if in.Inspection.TreeRequest == "" || in.Inspection.TreeRequest != githubclient.TreeRequestPath(in.Repo.Owner, in.Repo.Name, rev.Ref) {
		return apimodel.Source{}, false
	}
	if ref != "git/trees/"+rev.Ref {
		listed := false
		for _, p := range in.Inspection.Paths {
			listed = listed || p == ref
		}
		if !listed {
			return apimodel.Source{}, false
		}
	}
	return apimodel.Source{Kind: apimodel.SourceAPI, Ref: ref, URL: "https://api.github.com" + in.Inspection.TreeRequest}, true
}
