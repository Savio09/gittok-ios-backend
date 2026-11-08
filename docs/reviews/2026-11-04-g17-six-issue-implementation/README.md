# G17 six-issue implementation — November 4, 2026

**Outcome: RC1–RC6 implemented and independently reviewed.** Full maintained
tests, race tests, vet, build and required historical/adversarial controls pass.
This completes the six-issue repair scope; it does not claim universal
classification coverage, runtime execution, a completed UI or a deployed release.

Baseline: `6af0e931b258baee35579f6a362a9e9833977011` on
`fortune/sav-23-g17-extract-claims-link-each-to-source-filemetadata`.
User explicitly authorized an execution plan and implementation. The
[plan](../../../tasks/plan.md) records the ordered work. Historical
[repair recheck](../2026-11-04-g17-repair-recheck/README.md) and its failing
evidence were preserved. Existing API documentation and unrelated local
artifacts were retained. Local research/process documents are not committed.

Final head: `29384c1`. Three local code/test commits (not pushed):

- `3eed2f2` — complete, cited classification explanations (RC5/RC6).
- `708cf44` — evidence ranking and reusable peer reservations (RC2/RC3/RC4).
- `29384c1` — persisted-preview invalidation (RC1).

The version migration was committed last so it invalidates output predating all
proof/ranking changes. Source hashes before and after committing matched the
fully tested working tree; unrelated documentation was never staged.

## Changes and acceptance evidence

| Finding | Implementation | Maintained regression evidence |
|---|---|---|
| RC1 stale rendered previews | Builder version advances from 2 to 3, changing persisted-cache identity | Version-2 disk entry is not served after restart; fresh acquisition occurs and its result is subsequently reused after visibility checking |
| RC2 peer evidence displaced | Validated non-context observations from each plausible peer receive premise priority | Alphabetically earlier distractors and directory renames retain React/Cobra; unchanged Excalidraw snapshot retains React without resolving its unknown kind |
| RC3 unused peer shares | Dependency selection reserves fair shares of the effective product budget, then redistributes remaining capacity before tooling | Uneven peers now use all eight relevant dependency slots; real secondary declarations keep the six-product limit; ten peers receive distinct representation within eight slots |
| RC4 root tooling suppresses product | Competition uses actual positive source paths, validated role observations, parsed imports and complete parsed package clauses; dependency tables alone do not establish a competitor | Root dev-only package.json and root go.mod do not hide tools-only source; real no-import products still suppress helper calls/imports |
| RC5 overbroad Go absence | Incomplete listings withhold the display proof; wording describes only cited package clauses within the module's inspected scope | Depth-3/depth-4 counterexample abstains; complete-library proof remains available; nested-module files are not falsely denied |
| RC6 unsupported exclusion role | Excluded components retain role premises separately from dependency/name/path exclusion premises, and the display cites both | Multiline name and Express declarations are separately cited; missing role basis withholds proof; conjunctive CLI role cites both Go source and manifest |

Ranking applies to arbitrary repository evidence, with no repository-name
rules. The preview schema, visibility/credential gates, acquisition request
counts and not-executed presentation are preserved. Output stays within 24
claims, eight dependency declarations and two low-priority claims, with the
existing script/README and secondary-component limits.

`FactGoPackage` is internal relevance evidence. It reuses the existing Go
parser and witness validator, requires complete parsed non-test source, and
does not add a new rendered package/runtime claim. The selector was moved to
`internal/preview/selection.go` to keep reservation/allocation separate from
fact relevance. Dependency preselection uses candidate indices; final output
retains semantic order and counts omissions only once.

## Test-driven implementation and review

Focused tests demonstrated each failure before its fix: persisted old-version
reuse, depth-limited Go proof, missing role citation, lost React evidence,
root tooling suppression and unused dependency shares. Negative controls
retain real-product/helper distinction, current-version cache reuse and honest
inconclusive kinds. The implementation reused current standard-library/parser
contracts; no new dependencies or external framework APIs were introduced.

Fresh-context plan review found no required plan change, but highlighted actual
evidence paths and effective product capacity. Independent implementation
review covered source/cache and ranking separately using the user's chosen
reviewer model; no external model CLI was invoked.

The first ranking review caught a real introduced regression: when an existing
classifier policy selects a tests-directory command, observations can omit a
genuine no-import root library. Helpers then gained product relevance. That
finding was accepted and fixed using independently collected package clauses,
with maintained HTTP and helper-import controls. Both formerly failing
independent controls pass after the change. A second source check also covered
nested module boundaries; the final wording no longer asserts that other
modules' Go files do not exist. Final reviews returned no required findings.

## Final verification

All commands below completed successfully after the last production change:

```sh
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go build ./...
git diff --check
```

Logs: [tests](tests.txt), [race](race.txt), [vet](vet.txt), [build](build.txt).
Formatting checks returned no unformatted modified Go files. Some Go commands
needed execution permission for build/module caches and localhost test servers;
these were environment restrictions, not application test failures.

The [combined reviewer controls](reviewer-controls.txt) pass under the race
detector. They include all eight RC1–RC6 failing controls, earlier original
source/relevance failures, cache capture/TTL/admission checks, current-builder
restart reuse, multi-premise excluded-role proofs, 4–13-peer/permutation cases,
no-import product cases and credential/redirect security controls. The earlier
SW5/SW6 stale-overlay/hanging-test caveats remain as documented in the recheck;
their maintained current-source regressions are part of the full suite.

Reproduce the combined controls and the eight captured previews:

```sh
go test -race -count=1 \
  -overlay docs/reviews/2026-11-04-g17-six-issue-implementation/overlay.json \
  ./internal/preview ./internal/analysis ./internal/githubclient \
  -run '^(TestIndependentSource|TestSwarm|TestRepair|TestSourceRecheck(GoCommandBeyondDepth|ExcludedKindSource)|TestReviewPriorBuilder|TestRecheckSwarm|TestSecurityOverlay|TestAdversarial|TestImplementationCapture)' -v
python3 docs/reviews/2026-11-04-g17-six-issue-implementation/audit.py
```

The overlay adds test files only; production source is not replaced. Supporting
review tests are archived as `.go.txt`, outside normal package compilation.
Historical controls are referenced unchanged. Absolute paths target this
checkout and require adjustment when moved.

The [offline byte/line audit](audit.py) checks fresh [previews](previews) from
eight unchanged snapshots: CLI, PocketBase, Glow, Rack, Starship, Excalidraw,
Plantly and face-recognition. All 85 claims pass their production validation;
the separate Python audit checks pinned URLs, cited-file membership and line
ranges, exact primary excerpts, dependency names, budgets, and not-executed
sections against consumed snapshot bytes. It covers 89 total references,
including 78 file references. These checks do not prove the truth of every
author-written README statement or execute target software.

## Separately scoped limits and next work

- G15/G16: an unread alternative root Go package selected by build tags can
  still change the classification when read. The Python-library/backend
  boundary and tests-directory classifier scope mismatch also remain separate
  classification concerns. This repair suppresses misleading helper relevance;
  it does not claim to change those classifier policies.
- Unsupported layouts may still return unknown, including the already recorded
  PocketBase/Excalidraw selection limits. No assertion of universal repository
  classification accuracy is made.
- G18 remains the next backend uncertainty-presentation task. UI implementation
  can proceed against the current API, rendering all `additional_sources` and
  unknown/loading/failure states. Cloud execution is not needed for these fixes.
- No deployment, remote push or Linear status changes are part of this work.

Process lesson: exact counterexamples and normal suites were necessary but not
sufficient. Varying path ownership, irrelevant declarations, unequal peer sizes
and no-import source exposed assumptions that required narrower proof wording
and stronger typed evidence. Record those review-driven corrections alongside
the passing gates in the capstone process documentation.
