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
func Classify(insp Inspection) Classification {
	st := indexInspection(insp)
	comps := buildInventory(st)
	verdicts := make([]compVerdict, 0, len(comps))
	for _, c := range comps {
		verdicts = append(verdicts, classifyComponent(newRepoIndex(st, c, insp.Language)))
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
var detectors = []func(*repoIndex) (apimodel.RepoKind, []Observation, bool){
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
	name      string // the name its manifest declares, for dependency matching
	workspace bool   // its manifest aggregates other components
	status    roleStatus
	kind      apimodel.RepoKind
	obs       []Observation // support for kind, when resolved
	others    []Observation // the component's other candidates' evidence
	limits    []string
}

// classifyComponent resolves one component's role. The first candidate in
// precedence order is the role unless some uninspected evidence could establish
// a role that outranks it — an unreadable manifest, an unread Go entry point, a
// withheld absence premise — in which case the component is unresolved. A block
// that can only establish the same or a lower-precedence role does not change
// the verdict; it stays a limitation.
func classifyComponent(ix *repoIndex) compVerdict {
	v := compVerdict{comp: ix.comp, name: ix.componentName(), workspace: ix.isWorkspace()}
	type candidate struct {
		kind apimodel.RepoKind
		obs  []Observation
	}
	var cands []candidate
	for _, detect := range detectors {
		if kind, obs, ok := detect(ix); ok {
			cands = append(cands, candidate{kind: kind, obs: tag(obs, ix.comp.root, kind)})
		}
	}
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
				v.others = append(v.others, c.obs...)
			}
		} else {
			v.status, v.kind, v.obs = roleResolved, chosen.kind, chosen.obs
			for _, c := range cands[1:] {
				v.others = append(v.others, c.obs...)
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
//     alone: a root that declares a workspace (a private npm package with
//     "workspaces", a Cargo virtual manifest, a go.work without a module) is an
//     aggregator, not a product, and is set aside.
//   - An unresolved root makes the repository inconclusive: whatever the root
//     declares could not be determined, so no nested component may stand in.
//   - With no product at the root, the nested components are compared. Those
//     that are supporting — under an examples/docs/tooling directory, or named
//     as a dependency by another component in the repository — are set aside.
//     If the remaining ones are all resolved and agree on one kind, that is the
//     repository's kind; if any is unresolved, or they disagree, the result is
//     an explained abstention listing every candidate.
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
			return decided(root, vs, limits)
		case roleUnresolved:
			return abstain(vs, limits, "the repository root's role could not be determined from the inspected evidence, so no nested component can stand in for it")
		}
	}

	supporting := supportingComponents(vs)
	var top []*compVerdict
	for i := 1; i < len(vs); i++ {
		if !supporting[i] && vs[i].status != roleNone {
			top = append(top, &vs[i])
		}
	}
	if len(top) == 0 {
		return abstain(vs, limits, "no decisive frontend / backend / library / CLI / mobile signal was found in the inspected manifests and file tree")
	}
	for _, t := range top {
		if t.status == roleUnresolved {
			return abstain(vs, limits, "component "+t.comp.root+" could not be resolved, and with no product at the repository root it could be the primary one")
		}
		if t.kind != top[0].kind {
			return abstain(vs, limits, "independent components declare different roles: "+strings.Join(componentRoles(top), ", "))
		}
	}
	return decided(top[0], vs, limits)
}

func componentRoles(top []*compVerdict) []string {
	out := make([]string, len(top))
	for i, t := range top {
		out[i] = t.comp.root + " (" + string(t.kind) + ")"
	}
	return out
}

// supportingComponents marks the nested components that support another rather
// than standing as a product: those under an examples/docs/tooling directory,
// and those whose declared name another component lists as a dependency.
func supportingComponents(vs []compVerdict) map[int]bool {
	out := map[int]bool{}
	byName := map[string]int{}
	for i := 1; i < len(vs); i++ {
		if vs[i].name != "" {
			byName[vs[i].name] = i
		}
		if scopeRank(vs[i].comp.root+"/") >= 2 {
			out[i] = true
		}
	}
	for i, v := range vs {
		for _, d := range v.comp.deps {
			if j, ok := byName[d.Name]; ok && j != i {
				out[j] = true
			}
		}
	}
	return out
}

// decided reports primary's role, keeping every other component's evidence as
// attributable secondary evidence.
func decided(primary *compVerdict, vs []compVerdict, limits []string) Classification {
	c := Classification{Kind: primary.kind, Status: VerdictResolved, Observations: primary.obs}
	for i := range vs {
		if &vs[i] != primary {
			c.Secondary = append(c.Secondary, vs[i].obs...)
		}
		c.Secondary = append(c.Secondary, vs[i].others...)
	}
	c.Limitations = dedupe(limits)
	return c
}

// abstain reports an explained inconclusive verdict, listing every candidate.
func abstain(vs []compVerdict, limits []string, reason string) Classification {
	c := Classification{Kind: apimodel.KindOther, Status: VerdictInconclusive}
	for _, v := range vs {
		c.Secondary = append(c.Secondary, v.obs...)
		c.Secondary = append(c.Secondary, v.others...)
	}
	c.Limitations = dedupe(append(limits, reason))
	return c
}
