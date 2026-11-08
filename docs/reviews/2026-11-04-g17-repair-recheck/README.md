# G17 repair recheck — November 4, 2026

**Verdict: changes required.** The nine original failure cases now pass, but
six P2 repair/proof/relevance issues remain. No new P1 issue was identified in
this review. This is a bounded review, not a guarantee for every repository.

Reviewed `d0bd691..6af0e931b258baee35579f6a362a9e9833977011` on
`fortune/sav-23-g17-extract-claims-link-each-to-source-filemetadata`: six commits,
17 code/test files. Existing API documentation, plan edits and local historical
artifacts were preserved. No production code, maintained tests, branch, commit,
deployment or tracker state was changed by this review.

Contract: [repair handoff](../../../tasks/g17-review-repairs.md),
[previous findings SW1–SW9](../2026-11-04-g17-swarm-review/README.md).
The [author's implementation record](../2026-11-04-g17-repair-implementation.md)
describes the fixes but was not treated as independent verification.

## Method and confirmed progress

Three independent reviewers inspected source proofs, product relevance, and
cache/admission behavior. The parent reconciled their reports, read the code,
and reproduced the remaining failures. Code-review-and-quality,
doubt-driven-development and documentation-and-adrs workflows were applied.
The user's earlier selection of the current reviewer model was preserved; no
external model CLI was invoked.

- SW1 structural JSON location identity passes, including the escaped NUL key.
- SW2 consulted Go package-clause sources are now retained and cited.
- SW3 dependency/name and supporting-path exclusion provenance is retained.
- SW4 incomplete npm listings no longer establish no app shell.
- SW5 excess callers are rejected before visibility HTTP work.
- SW6 a deferred leader rechecks the completed cache and does not reset TTL.
- SW7–SW9 exact original peer/helper-import/README regressions pass.

The SW5 original test waited for all forbidden HTTP requests to start, so its
fixed-code version must assert rejection rather than wait for those requests.
The SW6 old production overlay would revert the fix; current-source hooks were
used instead. These adaptations preserve the intended failures, rather than
treating a hung test or stale source replacement as a regression.

## Required findings

### RC1 — P2: invalidate previews generated before these semantic fixes

Anchor: [build.go:32](../../../internal/preview/build.go#L32).

`BuilderVersion` remains 2 although ranking, proof wording and source semantics
changed. Persisted version-2 previews are accepted after reopening the cache and
are returned after one metadata read, without rebuilding their claims. The
independent control records `metadata=1, refs=0, trees=0, contents=0`.
An old incorrect proof can therefore bypass the repaired builder. Default TTL
is 30 minutes, but TTL is configurable; ephemeral hosting does not eliminate
the supported local/persistent-directory restart case.

Bump the version and add a persisted previous-version rejection regression.
Control: `TestReviewPriorBuilderSnapshotsMustNotSurviveRepair`.

### RC2 — P2: preserve decisive evidence for every inconclusive candidate

Anchor: [relevance.go:81](../../../internal/preview/relevance.go#L81).

Only a resolved basis contributes premise priority. An inconclusive frontend's
validated React observation is available but receives ordinary declaration
priority: changing irrelevant names from `z0…` to `a0…` removes React. This
also occurs on the unchanged recorded Excalidraw snapshot: earlier app
dependencies and documentation-site dependencies displace React/React DOM.
This is a residual repair-contract gap, not a new classifier policy request.

Preserve validated decisive observations for each plausible candidate without
inventing a primary component. Controls:
`TestRepairPeerFrameworkGetsSemanticPriority`,
`TestRepairRecordedExcalidrawPreservesPeerFramework`.

### RC3 — P2: redistribute unused peer shares before selecting tooling

Anchor: [relevance.go:265](../../../internal/preview/relevance.go#L265).

The new permanent equal-share cap keeps four dependencies from a CLI and one
from its frontend peer, then admits ESLint and Prettier from a root development
manifest. Five inspected CLI imports, including Cobra, are omitted while the
preview contains only seven dependency claims. Even the six-product cap used
with secondary declarations has remaining capacity.

Reserve fair initial representation, then allocate remaining capacity by
semantic priority before admitting lower-priority declarations. Preserve total
and secondary budgets. Control:
`TestRepairPeerQuotaDoesNotPreferDevelopmentToImportedProduct`.

### RC4 — P2: declarations alone do not establish a competing product

Anchor: [relevance.go:100](../../../internal/preview/relevance.go#L100).

Adding only root `package.json` with a private package and development-only
Prettier dependency to the original tools-product fixture suppresses that
product's HTTP construct. Its three product dependencies become Cobra plus
Prettier. Root `go.mod` declarations similarly suppress the only executable's
construct when its source lives under `tools/app/`.

The README exception repairs one instance; `anyPrimary` still treats any other
outside-supporting-path fact as competing product evidence. Use typed product
and component evidence before applying the supporting-path hint. This is a
residual SW9 scope gap. Controls:
`TestRepairToolProductSurvivesRootTooling`,
`TestRepairToolProductWithinRootGoModule`.

### RC5 — P2: Go absence wording exceeds the listing coverage

Anchor: [kindproof.go:291](../../../internal/preview/kindproof.go#L291).

The revised explanation says the file tree lists no other Go file under
`cmd/`, but its premise accepts a depth-limited listing at depth 3. A command
in `cmd/tool/sub/entry.go` is invisible at that depth. The partial inspection
displays a library proof; the complete counterexample displays CLI.

The classification policy predates this repair. The repair's explanation still
asserts more absence than its sources establish. Bound the explanation to the
actual conventional scope or withhold the proof when its asserted scope is
uncovered. This does not require a general classifier rewrite.
Control: `TestSourceRecheckGoCommandBeyondDepth`.

### RC6 — P2: cite an excluded component's displayed role or omit that role

Anchor: [kindproof.go:102](../../../internal/preview/kindproof.go#L102).

The new explanation labels an excluded sidecar `(backend)`, but its only
sidecar citation covers the name declaration on line 2. The Express dependency
establishing that role is on line 3 and is not cited. Root dependency plus
member name explains the exclusion, but does not establish the displayed role.

Cite its role basis, or omit the optional role label and retain the properly
cited exclusion reason. Control: `TestSourceRecheckExcludedKindSource`.

## Separately tracked, pre-existing classification limitation

A listed unread module-root `entry.go` can contain `package main` behind
`//go:build cli`, while the inspected `lib.go` contains `package lib` behind
`//go:build !cli`. The current no-command policy accepts one read root clause,
so it displays library until the other file is inspected; then it displays CLI.
The reviewer reproduced the same classification at `d0bd691` and independently
compiled both build-tag configurations. This belongs with G15/G16 coverage
work, not a claim that this repair introduced the classifier behavior.
Control: `TestSourceRecheckRootUninspectedAlternativePackage`.
The earlier Python-library/backend boundary remains separately documented in
the previous review. No runtime execution support is implied by these tests.

## Validation and reproducible evidence

Maintained gates independently passed on this head:

```sh
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go build ./...
git diff --check d0bd691..6af0e93
```

Sandbox loopback/cache restrictions initially blocked some commands; the
affected commands passed when rerun with the required execution permission.
Those environment errors were not attributed to the application.

Additional cache controls passed 20 repetitions under the race detector:
100 excess warm callers caused zero additional HTTP requests, canceled
visibility calls released admission, deferred leader hits preserved cache
bytes/capture/commit/expiry, and expiry triggered reacquisition. Maintained
visibility, identity, cancellation and generation controls passed.

Independent security overlays passed under the race detector: credential
scope expansion, missing scope evidence and revocation across six acquisition
stages; warm-cache scope expansion; five disallowed redirect origins. This
does not constitute a full penetration test.

Eight recorded repository replays were inspected. The separate Python audit
checks file excerpts, line spans and pinned URLs against consumed snapshots
for CLI, PocketBase, Glow and Rack (42 total source references). The additional
four replays passed production claim validation; they were not included in
that independent Python audit. Excalidraw fails the separate usefulness
assertion in RC2 despite having genuine underlying sources.

Reviewer tests are archived as `.go.txt`, outside maintained packages.
The [overlay](overlay.json) introduces virtual test files only; it does not
replace production source. Product capture output is redirected to temporary
test directories so old audit artifacts are preserved. To reproduce:

```sh
go test -count=1 -overlay docs/reviews/2026-11-04-g17-repair-recheck/overlay.json \
  ./internal/preview ./internal/analysis \
  -run '^(TestIndependentSource|TestSwarm|TestRepair|TestSourceRecheck|TestReviewPriorBuilder|TestRecheckSwarm)' -v
python3 docs/reviews/2026-11-04-g17-repair-recheck/audit.py
```

The first command intentionally fails. See [captured output](controls-output.txt):
eight failing controls establish the six required findings (RC2 and RC4 each
have two reproductions); one additional failure records the pre-existing
build-tag limit. Other controls pass. Paths in the overlay and audit script
refer to this checkout and need adjusting if the repository is moved.

## Process and next step

This pass demonstrates the distinction between exact regression repair and
general contract repair: all original counterexamples pass, while small
manifest/path variations and one unchanged real repository expose residual
failures. Normal gates also pass, so independent usefulness and evidence-scope
checks remain necessary.

Request a bounded follow-up for RC1–RC6, with the simple option of omitting an
unsupported label or abstaining instead of expanding the classifier. Recheck
these controls before accepting G17 or moving backend work to G18. UI work can
proceed in parallel against the existing contract, including all
`additional_sources`, limited/unknown results, loading and failure states.
No cloud sandbox or broad classifier expansion is needed to close this review.
