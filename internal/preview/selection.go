package preview

// selectClaims preserves semantic order while enforcing the claim-family
// budgets. Dependency reservations are computed separately so an unused peer
// share can be filled before lower-priority candidates are admitted.
func selectClaims(cands []candidate, secondaries, balancePeers bool) (out []candidate, omitted int) {
	dependencies := selectDependencies(cands, secondaries, balancePeers)
	var low, scripts, readmes int
	for i, c := range cands {
		switch {
		case len(out) == maxClaims,
			c.dep && !dependencies[i],
			c.low && low == maxLowPriority,
			c.script && scripts == maxScripts,
			c.readme && readmes == maxReadmePassage:
			omitted++
			continue
		}
		if c.low {
			low++
		}
		if c.script {
			scripts++
		}
		if c.readme {
			readmes++
		}
		out = append(out, c)
	}
	return out, omitted
}

// selectDependencies reserves each plausible peer's fair share of the
// effective product budget, using that peer's highest-priority declarations.
// A second pass fills remaining capacity in semantic order. Reservations are
// not permanent caps: a small peer cannot strand another peer's useful facts.
func selectDependencies(cands []candidate, secondaries, balancePeers bool) map[int]bool {
	selected := map[int]bool{}
	productCap := maxDependencies
	if secondaries {
		productCap = maxProductDeps
	}
	var deps, products, low int
	take := func(i int, c candidate) bool {
		if selected[i] || deps == maxDependencies || (c.productDep && products == productCap) || (c.low && low == maxLowPriority) {
			return false
		}
		selected[i] = true
		deps++
		if c.productDep {
			products++
		}
		if c.low {
			low++
		}
		return true
	}
	if balancePeers {
		perPeer := map[string]int{}
		for _, c := range cands {
			if c.dep && c.productDep && !c.low {
				perPeer[c.component] = 0
			}
		}
		if len(perPeer) > 1 {
			share := max(1, productCap/len(perPeer))
			for i, c := range cands {
				if c.dep && c.productDep && !c.low && perPeer[c.component] < share && take(i, c) {
					perPeer[c.component]++
				}
			}
		}
	}
	for i, c := range cands {
		if c.dep {
			take(i, c)
		}
	}
	return selected
}
