package crawler

import (
	"strings"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

// Classify infers a repository's RepoKind from already-gathered evidence. It is
// a pure, network-free function: the caller assembles an Inspection (the file
// tree, the files it managed to read, and the scope that bounded the fetch), and
// Classify reasons only over that. It never calls the GitHub API.
//
// It works in three steps, each kept separate so a reader can audit it:
//
//  1. Inventory (inventory.go): partition the inspection into components — the
//     repository root and each directory holding its own manifest.
//  2. Component roles (classify.go, classifyComponent): extract role evidence
//     inside each component only, and resolve that component's role.
//  3. Repository role (resolveRepository): decide which component speaks for the
//     repository, keeping the others as attributable secondary components.
//
// It returns a structured Classification (see evidence.go): the Kind, a Status
// that is honest about abstention, the Observations that produced the verdict,
// the Secondary evidence of other components, and the Limitations that bounded
// the decision.
func Classify(insp Inspection) Classification { return classify(insp, nil) }

// candidateHook lets a test replace a component's extracted candidates before
// validation, to show that evidence which fails its witness never reaches the
// verdict. Production code passes nil.
type candidateHook func(root string, cands []candidate) []candidate

// classify is Classify with an optional hook between extraction and validation.
func classify(insp Inspection, hook candidateHook) Classification {
	st := indexInspection(insp)
	comps := buildInventory(st)
	verdicts := make([]compVerdict, 0, len(comps))
	for _, c := range comps {
		ix := newRepoIndex(st, c, insp.Language)
		cands := extractCandidates(ix)
		if hook != nil {
			cands = hook(c.root, cands)
		}
		verdicts = append(verdicts, classifyComponent(ix, validateCandidates(ix, cands)))
	}
	return resolveRepository(st, verdicts)
}

// detectors extract role evidence from one component, in precedence order.
//
// # Precedence within a component (#algorithms)
//
// A component often carries several signals at once (a Next.js app also runs a
// Node server; a Go server also exposes subcommands). Every detector runs; the
// first hit in this order is the component's role, the rest are recorded as
// secondary evidence of the same component:
//
//   - mobile before frontend: react-native/expo apps also depend on react.
//   - frontend before backend: a client app that ships a server dependency for
//     its dev tooling is labelled by its user-facing surface.
//   - backend before cli: a server that exposes subcommands is still a server.
//   - library last: an importable package that also produces a runnable
//     artifact is classified by the artifact.
var detectors = []func(*repoIndex) (apimodel.RepoKind, []evidenceGroup){
	detectMobile,
	detectFrontend,
	detectBackend,
	detectCLI,
	detectLibrary,
}

// roleRank orders roles by the precedence above; lower outranks higher.
func roleRank(k apimodel.RepoKind) int {
	switch k {
	case apimodel.KindMobile:
		return 0
	case apimodel.KindFrontend:
		return 1
	case apimodel.KindBackend:
		return 2
	case apimodel.KindCLI:
		return 3
	case apimodel.KindLibrary:
		return 4
	}
	return 5
}

// roleStatus is a component's outcome.
type roleStatus int

const (
	// roleNone: nothing role-bearing was found and nothing material was left
	// uninspected (e.g. a tooling-only pyproject, or a directory of assets).
	roleNone roleStatus = iota
	// roleResolved: a role is supported and no uninspected evidence could
	// establish a role that outranks it.
	roleResolved
	// roleUnresolved: uninspected evidence could establish a role that the
	// inspected evidence does not rule out.
	roleUnresolved
)

// compVerdict is one component's role and the evidence behind it.
type compVerdict struct {
	comp      *component
	name      string  // the name its manifest declares, for dependency matching
	nameW     Witness // the field that declares name (located), when known
	workspace bool    // its manifest aggregates other components
	status    roleStatus
	kind      apimodel.RepoKind
	obs       []Observation // support for kind, when resolved
	basis     []Observation // one complete validated group for kind, locations attached
	others    []Observation // the component's other candidates' evidence
	limits    []string
}

// candidate is one role a component's evidence may support: a disjunction of
// evidence groups, each a conjunction of witnessed observations.
type candidate struct {
	kind   apimodel.RepoKind
	groups []evidenceGroup
}

// extractCandidates runs every detector over the component, in precedence
// order, attributing each observation to the component and role.
func extractCandidates(ix *repoIndex) []candidate {
	var cands []candidate
	for _, detect := range detectors {
		kind, groups := detect(ix)
		if len(groups) == 0 {
			continue
		}
		for i := range groups {
			groups[i] = tag(groups[i], ix.comp.root, kind)
		}
		cands = append(cands, candidate{kind: kind, groups: groups})
	}
	return cands
}

// validateCandidates checks every observation's witness against the inspection
// (see witness.go) before it can affect a verdict. A group survives only if all
// of its observations validate; a candidate survives only if at least one of
// its non-context groups does, and its rejected groups are recorded with the
// reason. Context observations are kept only when they validate and their
// candidate is supported. What survives is exactly what resolution — and the
// rendered pack — may use.
func validateCandidates(ix *repoIndex, cands []candidate) []candidate {
	var out []candidate
	for _, c := range cands {
		var kept, context []evidenceGroup
		for _, g := range c.groups {
			failure := ""
			for _, o := range g {
				if err := validateWitness(ix.st, o.Witness); err != nil {
					failure = o.describe() + ": " + err.Error()
					break
				}
			}
			switch {
			case failure != "" && g.isContext():
				ix.note("rejected supporting " + string(c.kind) + " evidence " + failure)
			case failure != "":
				ix.note("rejected " + string(c.kind) + " evidence " + failure)
			case g.isContext():
				context = append(context, g)
			default:
				kept = append(kept, g)
			}
		}
		if len(kept) == 0 {
			continue
		}
		out = append(out, candidate{kind: c.kind, groups: append(kept, context...)})
	}
	return out
}

// isContext reports whether every observation in the group is supporting
// context, so the group cannot support a role on its own.
func (g evidenceGroup) isContext() bool {
	for _, o := range g {
		if !o.Context {
			return false
		}
	}
	return len(g) > 0
}

// flatten lists a candidate's observations in group order.
func (c candidate) flatten() []Observation {
	var out []Observation
	for _, g := range c.groups {
		out = append(out, g...)
	}
	return out
}

// describe names an observation and its witness, for limitations.
func (o Observation) describe() string {
	return "(" + o.Detail + "; witness " + o.Witness.String() + ")"
}

// classifyComponent resolves one component's role from its validated
// candidates. The first in precedence order is the role unless some uninspected
// evidence could establish a role that outranks it — an unreadable manifest, an
// unread Go entry point, a withheld absence premise — in which case the
// component is unresolved. A block that can only establish the same or a
// lower-precedence role does not change the verdict; it stays a limitation.
func classifyComponent(ix *repoIndex, cands []candidate) compVerdict {
	v := compVerdict{comp: ix.comp, name: ix.componentName(), nameW: ix.componentNameWitness(), workspace: ix.isWorkspace()}
	switch {
	case len(cands) > 0:
		chosen := cands[0]
		var outranking []string
		for _, b := range ix.blocks {
			if roleRank(b.role) < roleRank(chosen.kind) {
				outranking = append(outranking, string(b.role))
			}
		}
		if len(outranking) > 0 {
			v.status = roleUnresolved
			ix.note("uninspected evidence could establish a " + strings.Join(dedupe(outranking), "/") + " role, which would outrank the inspected " + string(chosen.kind) + " evidence")
			for _, c := range cands {
				v.others = append(v.others, c.flatten()...)
			}
		} else {
			v.status, v.kind, v.obs = roleResolved, chosen.kind, chosen.flatten()
			// validateCandidates orders supported groups before context groups.
			for _, o := range chosen.groups[0] {
				if !o.Context {
					o.Witness = ix.st.locate(o.Witness)
					v.basis = append(v.basis, o)
				}
			}
			for _, c := range cands[1:] {
				v.others = append(v.others, c.flatten()...)
			}
		}
	case len(ix.blocks) > 0:
		v.status = roleUnresolved
	}
	v.limits = ix.limitations()
	return v
}

// tag attributes observations to their component and the role they support.
func tag(obs []Observation, root string, role apimodel.RepoKind) []Observation {
	out := make([]Observation, len(obs))
	for i, o := range obs {
		o.Component, o.Role = root, role
		out[i] = o
	}
	return out
}

// resolveRepository decides which component speaks for the repository.
//
// # Policy
//
//   - The root manifest is the repository's declared package, so a resolved
//     repository root is the primary product and every other component is
//     secondary. This follows each ecosystem's manifest semantics, not location
//     alone: a workspace root aggregates members, and can also declare its own
//     product. Its supported role participates in the workspace comparison;
//     an artifact-free aggregator contributes no product candidate.
//   - An unresolved root makes the repository inconclusive: whatever the root
//     declares could not be determined, so no nested component may stand in.
//   - With no product at the root, the nested components are compared — but
//     only over a complete listing, since an omitted component could itself be
//     a product. Those
//     that are supporting — under an examples/docs/tooling directory, or named
//     as a dependency by another component in the repository — are set aside.
//     If the remaining ones are all resolved and agree on one kind, that is the
//     repository's kind. If they disagree only in that some are importable
//     libraries and the rest share one executable role, the executable role is
//     the product — the within-component rule that an artifact outranks a
//     library — and the libraries are packages published alongside it. Any
//     unresolved member, or two different executable roles, is an explained
//     abstention listing every candidate. Known limit: a library monorepo with a
//     single playground or demo app outside the examples/docs/tooling hints is
//     reported by that app.
func resolveRepository(st *inspIndex, vs []compVerdict) Classification {
	var limits []string
	if st.scope.Truncated {
		limits = append(limits, "the file listing was truncated; a signal absent from it cannot be treated as absent from the repository")
	}
	if st.scope.DepthLimited {
		limits = append(limits, "the file listing was depth-limited; files below the depth cap were not listed, so a deeper entry point may exist")
	}
	for _, v := range vs {
		for _, l := range v.limits {
			if v.comp.root != "" {
				l = "component " + v.comp.root + ": " + l
			}
			limits = append(limits, l)
		}
	}

	root := &vs[0] // the inventory sorts the repository root first
	if !root.workspace {
		switch root.status {
		case roleResolved:
			return decided(root, vs, limits, nil)
		case roleUnresolved:
			return abstain(vs, limits, "the repository root's role could not be determined from the inspected evidence, so no nested component can stand in for it", []*compVerdict{root})
		}
	}

	// Comparing nested components assumes every component is known. A truncated
	// or depth-limited listing can omit one that would itself be a product, so
	// that comparison needs a complete listing; the root-resolved case above does
	// not, because the root manifest speaks for the repository regardless.
	supporting, reasons := supportingComponents(st, vs)
	var top []*compVerdict
	// npm workspaces manage linked packages; they do not exclude an application
	// in the root package. Compare that artifact with members instead of dropping
	// it, including uncertainty that could change the workspace's primary role.
	// https://docs.npmjs.com/cli/v11/using-npm/workspaces/
	if root.workspace && root.status != roleNone {
		top = append(top, root)
	}
	for i := 1; i < len(vs); i++ {
		if !supporting[i] && vs[i].status != roleNone {
			top = append(top, &vs[i])
		}
	}
	if !st.scope.Complete() {
		return abstain(vs, limits, "the repository root declares no product of its own, and the listing is incomplete, so components it omits could also be products; nested components were not compared", top)
	}
	if len(top) == 0 {
		return abstain(vs, limits, "no decisive frontend / backend / library / CLI / mobile signal was found in the inspected manifests and file tree", nil)
	}
	var executables []*compVerdict
	for _, t := range top {
		if t.status == roleUnresolved {
			return abstain(vs, limits, "component "+t.comp.root+" could not be resolved, and with no product at the repository root it could be the primary one", top)
		}
		if t.kind != apimodel.KindLibrary {
			executables = append(executables, t)
		}
	}
	switch {
	case allKind(top, top[0].kind):
		return withExclusions(decided(top[0], vs, limits, top), vs, reasons)
	case len(executables) > 0 && allKind(executables, executables[0].kind):
		// One executable role beside importable libraries: the same rule as
		// within a component — an artifact outranks a library — so the libraries
		// are packages published alongside the product.
		return withExclusions(decided(executables[0], vs, limits, top), vs, reasons)
	}
	return abstain(vs, limits, "independent components declare different roles: "+strings.Join(componentRoles(top), ", "), top)
}

func allKind(vs []*compVerdict, k apimodel.RepoKind) bool {
	for _, v := range vs {
		if v.kind != k {
			return false
		}
	}
	return true
}

func componentRoles(top []*compVerdict) []string {
	out := make([]string, len(top))
	for i, t := range top {
		out[i] = t.comp.root + " (" + string(t.kind) + ")"
	}
	return out
}

// exclusionCause is why supportingComponents set a component aside: its path,
// or a dependency another component declares on its name.
type exclusionCause struct {
	reason ExclusionReason
	dep    Witness // the depending declaration, for a dependency exclusion
}

// supportingComponents marks the nested components that support another rather
// than standing as a product: those under an examples/docs/tooling directory,
// and those whose declared name another component lists as a dependency. Each
// mark keeps its cause, so a comparison can cite why a component was set aside.
func supportingComponents(st *inspIndex, vs []compVerdict) (map[int]bool, map[int]exclusionCause) {
	out := map[int]bool{}
	causes := map[int]exclusionCause{}
	byName := map[string]int{}
	for i := 1; i < len(vs); i++ {
		if vs[i].name != "" {
			byName[vs[i].name] = i
		}
		if scopeRank(vs[i].comp.root+"/") >= 2 {
			out[i] = true
			causes[i] = exclusionCause{reason: ExcludedByPath}
		}
	}
	for i, v := range vs {
		for _, d := range v.comp.deps {
			if j, ok := byName[d.Name]; ok && j != i {
				out[j] = true
				if _, have := causes[j]; !have {
					causes[j] = exclusionCause{reason: ExcludedAsDependency, dep: dependencyWitness(st, d)}
				}
			}
		}
	}
	return out, causes
}

// withExclusions records, in a comparison's basis, every role-bearing
// component that was set aside as supporting, with its validated premises.
func withExclusions(c Classification, vs []compVerdict, causes map[int]exclusionCause) Classification {
	if c.Basis == nil {
		return c
	}
	for i := 1; i < len(vs); i++ {
		cause, ok := causes[i]
		if !ok || vs[i].status == roleNone {
			continue
		}
		e := Exclusion{Component: vs[i].comp.root, Reason: cause.reason}
		if vs[i].status == roleResolved {
			e.Kind = vs[i].kind
			e.RolePremises = vs[i].basis
		}
		switch cause.reason {
		case ExcludedByPath:
			if len(vs[i].comp.manifests) > 0 {
				m := vs[i].comp.manifests[0].path
				e.Premises = []Observation{{Path: m, Detail: "lies under an example, documentation or tooling directory", Witness: Witness{Kind: WitnessListing, Paths: []string{m}, Construct: constructSupportingPath}}}
			}
		case ExcludedAsDependency:
			if vs[i].nameW.Kind != "" {
				e.Premises = []Observation{
					{Path: cause.dep.Path, Detail: "declares a dependency on " + vs[i].name, Witness: cause.dep},
					{Path: vs[i].nameW.Path, Detail: "declares the name " + vs[i].name, Witness: vs[i].nameW},
				}
			}
		}
		c.Basis.Excluded = append(c.Basis.Excluded, e)
	}
	return c
}

// decided reports primary's role, keeping every other component's evidence as
// attributable secondary evidence. compared, when non-nil, holds the product
// candidates of a nested comparison (primary among them), recorded in the basis.
func decided(primary *compVerdict, vs []compVerdict, limits []string, compared []*compVerdict) Classification {
	c := Classification{Kind: primary.kind, Status: VerdictResolved, Observations: primary.obs}
	c.Basis = &Basis{Component: primary.comp.root, Kind: primary.kind, Premises: primary.basis, ByComparison: compared != nil}
	c.Products = []string{primary.comp.root}
	for _, t := range compared {
		if t != primary {
			c.Basis.Compared = append(c.Basis.Compared, Basis{Component: t.comp.root, Kind: t.kind, Premises: t.basis})
			c.Products = append(c.Products, t.comp.root)
		}
	}
	for i := range vs {
		if &vs[i] != primary {
			c.Secondary = append(c.Secondary, vs[i].obs...)
		}
		c.Secondary = append(c.Secondary, vs[i].others...)
	}
	c.Limitations = dedupe(limits)
	return c
}

// abstain reports an explained inconclusive verdict, listing every candidate
// and the components that could be the product.
func abstain(vs []compVerdict, limits []string, reason string, products []*compVerdict) Classification {
	c := Classification{Kind: apimodel.KindOther, Status: VerdictInconclusive}
	for _, p := range products {
		c.Products = append(c.Products, p.comp.root)
	}
	for _, v := range vs {
		c.Secondary = append(c.Secondary, v.obs...)
		c.Secondary = append(c.Secondary, v.others...)
	}
	c.Limitations = dedupe(append(limits, reason))
	return c
}
