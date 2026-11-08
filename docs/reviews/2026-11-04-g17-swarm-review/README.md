# Independent G17 repair swarm review

**Verdict: changes required before accepting the repair or moving to G18.**

Reviewed November 4, 2026. Base `82eb571`; final code head
`d0bd6918b04a5011dcb1041b848ff8079a55a246` on
`fortune/sav-23-g17-extract-claims-link-each-to-source-filemetadata`.
The 12 commits change 52 files. The working-tree API documentation and local
implementation report were also read. Existing document modifications and untracked
history were preserved. No application code, fixtures, branch, commits or tracker
state were changed by this review.

Contract: [repair handoff](../../../tasks/g17-review-repairs.md).
Author's report: [implementation record](../2026-11-04-g17-repair-implementation.md).
This review independently assesses that report rather than treating it as proof.

## Scope and method

Four reviewers independently examined product relevance, exact sources/kind proofs,
privacy/credentials and cache/concurrency. The parent agent read the relevant code,
reconciled findings and reran the nine failing adversarial controls together. Tests
use Go overlays; the live source tree remains unchanged. The cache scheduling test
adds exactly one conditional barrier after the actual outer cache miss; the parent
verified that this is the sole production-overlay difference.

The code-review, systematic debugging and fresh-context adversarial-review workflows
were applied. The user's earlier choice to continue with the current reviewer model
was preserved; no external model CLI was invoked. Normal test gates and failure
reproductions are reported separately.

## Required findings

All nine findings are **P2**, with deterministic reproductions. No P1 security
defect was found. These are specific failures of the repair contracts, not style
preferences or requirements for a new architecture.

### SW1 — JSON location identity is ambiguous

Anchor: [locations.go:46](../../../internal/crawler/locations.go#L46).
`locationKey` joins decoded JSON keys with NUL. A legal property containing
`\u0000` therefore aliases a nested field path. With a real `dependencies.react`
declaration followed by `dependencies\u0000react: "UNRELATED"`, both the React
claim and frontend proof cite the unrelated member on line 3 rather than the real
declaration on line 2. Semantic validation uses the same collided locator key.

Use a structural/collision-free field-path key, including descendant invalidation
for duplicate keys. Preserve exact effective-field selection and add this legal
escaped-key counterexample. Test: `TestIndependentSourceNULCollision`.

### SW2 — Go no-command proof omits necessary read source

Anchor: [classify.go:649](../../../internal/crawler/classify.go#L649).
The typed `AbsenceGoCommand` scope records only `go.mod`. In an importable module,
a consumed `cmd/tool/helper.go` with `package helper` is also necessary to exclude
a conventional command, but the proof omits it and says the tree shows no package
main. Making just that file unread changes the result to `other`; tree names cannot
establish source package clauses.

Carry every required inspected clause/file through the typed premise and citations;
describe the content inference accurately. If any premise is unavailable, withhold
the resolved display. Test: `TestIndependentSourceAbsencePremiseFiles`.

### SW3 — Product comparison drops exclusion provenance

Anchor: [resolve.go:402](../../../internal/crawler/resolve.go#L402).
The exported basis retains compared products, but not the facts that excluded
other components as supporting. A workspace root depending on a named backend
sidecar can resolve its nested frontend as the sole product. The explanation cites
only the frontend and tree, omitting the root dependency and sidecar name. Removing
that omitted dependency changes the result to `other`.

Retain typed workspace/root and dependency/name selection premises, or withhold
the displayed conclusion when the full comparison basis cannot be cited. A generic
supporting-component disclaimer does not establish the specific exclusion.
Test: `TestIndependentSourceWorkspacePremises`.

### SW4 — Truncated listing becomes confident absence

Anchor: [kindproof.go:260](../../../internal/preview/kindproof.go#L260).
A complete `package.json` with `main: "./index.js"`, a recorded tree request and
`Scope.Truncated=true` still produces a library proof claiming the tree shows no
app shell. The proof checks tree provenance but lacks the coverage premise needed
to establish absence. An unseen shell may lie beyond truncation.

Require the relevant listing coverage in both the typed premise and display gate;
otherwise abstain. The detector policy existed earlier, but the new complete-proof
promise and presentation make this a repair acceptance issue.
Test: `TestIndependentSourceIncompleteAbsence`.

### SW5 — Visibility work runs outside admission bounds

Anchor: [preview.go:31](../../../internal/analysis/preview.go#L31).
Every request starts a GitHub metadata call before joining the bounded group.
With active/per-key/waiter limits all set to one and already saturated, 80 excess
callers still made 80 simultaneous upstream metadata requests. They were rejected
only after paying that network/quota cost. Slow/retrying metadata is also outside
the group's resource bound.

Apply finite admission to request/visibility work before contacting GitHub. Keep
current public checks on cache hits; visibility and full-acquisition budgets may
be separate, but neither should be unbounded. Test:
`TestSwarmAdmissionBoundsPreAdmissionHTTP`.

### SW6 — Cache/flight handoff repeats completed acquisition

Anchors: [preview.go:44](../../../internal/analysis/preview.go#L44),
[coalesce.go:111](../../../internal/analysis/coalesce.go#L111).
A caller can observe a cache miss, pause, then win a new flight after the first
caller stores and retires its flight. The new leader unconditionally reacquires.
Two concurrent identical callers produced 4 metadata reads, 2 ref resolutions and
2 tree reads despite the first storing a valid entry. Selected contents would be
fetched twice too. The race detector does not detect this logical race.

Recheck the verified-identity cache inside newly selected leader work. Treat it
as a hit without rewriting zero/new provenance or extending the original TTL.
Test: `TestSwarmCacheMissRechecksAfterWinningFlight`.

### SW7 — Abstention invents a primary component

Anchor: [relevance.go:76](../../../internal/preview/relevance.go#L76).
The ranker uses `Products[0]` as primary even when classification is inconclusive.
For independent CLI/frontend candidates, renaming only the CLI directory from `a/`
to `z/` flips the allocation from six CLI/two frontend dependency slots to the
opposite allocation, dropping Cobra. Candidate ordering is not a product decision.

Treat inconclusive products as peers and preserve meaningful component-local
evidence across the budget. Only a resolved deciding component can receive the
primary preference. Test: `TestSwarmInconclusiveCandidatesArePeers`.

### SW8 — Helper-only imports gain product priority

Anchor: [relevance.go:87](../../../internal/preview/relevance.go#L87).
`CollectImports` retains the importing file, but ranking drops that file's scope
and indexes imports only by component. Helper imports below `script/` in the root
module promote root `go.mod` declarations as if product source imported them.
A product importing `zproduct` and Cobra loses `zproduct` to seven helper-only
dependencies imported exclusively by a script.

Retain importing-file product/support scope when deriving relevance. Preserve
legitimate tool-product fallback instead of globally banning paths.
Test: `TestSwarmHelperImportsDisplaceProduct`.

### SW9 — A root README demotes an actual tool product

Anchor: [relevance.go:90](../../../internal/preview/relevance.go#L90).
`anyPrimary` treats any non-supporting-path fact, including a root README, as
evidence that `tools/` code is supporting. When the sole executable is under
`tools/app/`, adding its root product description removes its actual HTTP source
construct and reduces declarations from three to two. There is no other product;
only documentation was added.

Determine competing product evidence from typed facts, not the presence of any
root documentation. A path hint cannot alone negate the actual product fallback.
Test: `TestSwarmToolProductWithREADME`.

## What works

The ordinary CLI/PocketBase cases improve materially: CLI includes Cobra and no
helper gateway; PocketBase retains useful main/UI declarations and exact build/dev
lines. Glow keeps Cobra; unsupported Rack remains a useful attributed partial
preview with a short abstention note. Dependency budgets, metadata metrics and
ordinary source positions work on these recorded inputs.

The product reviewer independently checked 42 source references across four rebuilt
previews; the source reviewer separately checked 43 positioned references across
another four-repository set. These sets overlap; they are not 85 unique sources.
Ordinary sources/line ranges passed, while SW1 shows an adversarial exception.
Seven independent normal Go/JSON position cases also passed.

No concrete P1/P2 privacy/credential issue was found. Independent race-enabled
security controls checked 18 mid-pipeline policy failures with no preview/cache
write, scope expansion on a warm hit, and five off-origin redirect variants that
sent no second request. Maintained identity, cache expiry, cancellation and retired-
generation controls pass. Live authenticated access was reported by the author,
but was not independently repeated using the owner's secret in this review.

## Verification and reproducibility

Parent-agent gates on the unchanged application tree:

```text
go test -count=1 ./...           PASS
go test -race -count=1 ./...     PASS
go vet ./...                    PASS
go build ./...                  PASS
git diff --check 82eb571..HEAD   PASS
git diff --check                PASS
changed Go formatting           PASS
```

The initial sandboxed build warned about a Go module metadata-cache write; a
subsequent build with the required permission completed cleanly. No environment
restriction was counted as a passing check.

The parent merged the reviewers' overlay mappings and independently reran all nine
behavioral controls. **All nine fail as described.** The seven normal parser cases
in the same run pass. Cache controls also fail in all 20 reviewer repetitions under
`-race`, with no data-race report. Existing green suites do not cover these inputs
and interleavings.

Reproduce from this checkout, with local test-server permissions:

```sh
go test -overlay docs/reviews/2026-11-04-g17-swarm-review/integrator-overlay.json -run '^(TestIndependentSource(NULCollision|AbsencePremiseFiles|WorkspacePremises|IncompleteAbsence|NormalPositions)|TestSwarm(HelperImportsDisplaceProduct|InconclusiveCandidatesArePeers|ToolProductWithREADME|AdmissionBoundsPreAdmissionHTTP|CacheMissRechecksAfterWinningFlight))$' -v ./internal/preview ./internal/analysis
```

Overlay mappings contain this checkout's absolute paths; regenerate them when
moving the artifacts. They add virtual tests and one synchronization hook; they
do not modify live Go files or standard test configuration.

Detailed reports and archived controls:

- [Product relevance](product/README.md).
- [Sources and kind proof](sources/review.md).
- [Privacy and credentials](security/README.md).
- [Cache and concurrency](cache/README.md).

## Recommended next pass

1. Fix SW1–SW4: collision-free locations and genuinely complete/coverage-valid kind
   proofs. Promote the failing controls into maintained tests beside each fix.
2. Fix SW5–SW6: admission before external work and the cache recheck inside leader
   work. Keep current visibility checks, cache provenance and privacy invariants.
3. Fix SW7–SW9: preserve unresolved peer relevance and importing-file scope, and
   avoid treating a README as another product. Retest both ordinary and tool-only
   layouts without repository-specific rules.
4. Repeat focused adversarial controls, independent source/usefulness audits and
   full verification after the last change. Preserve these failures in the process
   record; do not just regenerate goldens or weaken the tests.

There is also a **separate pre-existing G15 classification limitation**: a generic
packaged Python library becomes backend solely when `starlette` is added, and
library when removed. The detector policy is unchanged from `82eb571`; a faithful
one-field classifier proof does not establish correct product classification.
[Boundary evidence](sources/library-boundary.md). Track this explicitly before
claiming broad classification accuracy; do not fold it into the nine new repair
findings or require an unbounded classifier rewrite to close this pass.

The next planned ticket is G18/SAV-24 (supported vs inferred vs unknown evidence),
confirmed by a read-only Linear lookup. Accept the G17 repair only after these
contract failures are resolved; moving on now would carry unsupported proofs and
unreliable quota protection into the uncertainty layer. No Linear status was changed.
