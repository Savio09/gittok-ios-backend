# Execution plan: G17 six-issue repair

Prepared November 4, 2026. User authorized planning and execution.
Baseline: `6af0e931b258baee35579f6a362a9e9833977011` on
`fortune/sav-23-g17-extract-claims-link-each-to-source-filemetadata`.
The previous active plan is preserved in
[g17-plan-before-six-issue-execution.md](g17-plan-before-six-issue-execution.md).
Linear G17/SAV-23 remains the parent work item; this is its execution checklist.
No new tracker items, deployment, UI or cloud execution are required.

## Contract

Implement RC1–RC6 from the [independent repair recheck](../docs/reviews/2026-11-04-g17-repair-recheck/README.md).
Preserve arbitrary-repository ingestion, provenance, public visibility gates,
static/not-executed wording and bounded output. Never special-case repository
names or promote a plausible candidate to a resolved kind. Preserve the
24-claim, 8-dependency and 2-low-priority budgets and secondary-component limits.
Historical review artifacts stay unchanged.

The Python-library boundary and unread root Go build-tag limitation remain
separate G15/G16 work. This repair must not claim to solve them.

## Ordered tasks

### 1. RC1: invalidate pre-repair persisted previews

- [x] Seed a version-2 disk entry, reopen the cache, and require fresh acquisition.
- [x] Advance the builder version, keeping visibility gates and TTL intact.
- Verify: focused analysis/cache tests and current-version restart reuse.
- Files: preview `build.go`, analysis `cache_service_test.go`.
- Dependencies: none. Scope: small.

### 2. RC5: keep Go absence proofs within listing coverage

- [x] Reproduce depth-3 coverage with an unseen depth-4 command.
- [x] Withhold no-command display proofs unless coverage supports their whole
  asserted cmd/ scope. Keep complete-library proofs and all clause citations.
- Verify: kind-proof tests and earlier SW2/SW4 source controls.
- Files: preview `kindproof.go`, `kindproof_test.go`.
- Dependencies: none. Scope: small.

### 3. RC6: cite displayed excluded-component roles

- [x] Retain the excluded component's validated role basis separately from its
  dependency/name/path exclusion reason.
- [x] Cite that role basis when displaying its role; withhold a proof when a
  displayed role lacks a complete representable basis.
- [x] Cover multiline name/dependency locations and missing role evidence.
- Verify: crawler basis and preview kind-proof tests; original SW3 stays valid.
- Files: crawler `evidence.go`, `resolve.go`, preview `kindproof.go`, tests.
- Dependencies: none. Scope: medium.

Checkpoint: cache/source regressions pass; review the diff before ranking work.

### 4. RC2: retain decisive evidence for every plausible peer

- [x] Give validated non-context role observations of all plausible peers
  premise priority, without choosing a primary or changing the classifier.
- [x] Cover distractors ordered before React and unchanged recorded Excalidraw.
- Verify: relevance tests, permutations/renames and recorded inputs.
- Files: preview `relevance.go`, `relevance_test.go`.
- Dependencies: none. Scope: small.

### 5. RC4: distinguish products from root tooling declarations

- [x] Replace any-declaration competition with typed positive product evidence.
  README and dependency tables alone do not prove a competing product; real
  outside-product code must still demote helpers.
- [x] Cover root development-only package.json, root go.mod with tools source,
  genuine helpers beside a product, and path/rename variants.
- Verify: relevance tests and original helper HTTP/import controls.
- Files: preview ranker and tests, crawler facts; use validated observations,
  imports and parsed package clauses independently of the selected role.
- Dependencies: task 4 shares the ranker; implement sequentially. Scope: small.

### 6. RC3: reserve fair representation, then redistribute capacity

- [x] Replace permanent quotas with fair initial dependency reservations,
  followed by redistribution in semantic priority order before tooling.
- [x] Preserve all claim/dependency/product/secondary/low-priority budgets.
- [x] Cover uneven peers, more peers than slots, renames, irrelevant additions
  and deterministic output. Validated role premises precede ordinary imports.
- Verify: selection/relevance tests and recorded repository previews.
- Files: preview `build.go`, selection/ranking code and tests.
- Dependencies: tasks 4–5. Scope: medium.

Checkpoint: the eight required failing reviewer controls turn green and the
original regressions stay fixed. Keep the separate G15/G16 limit documented.

### 7. Independent review, final checks and process record

- [x] Fresh-context adversarial review of proof/citations, ranking and cache;
  reconcile executable findings using the user's chosen current reviewer model.
- [x] Run `go test -count=1 ./...`, `go test -race -count=1 ./...`,
  `go vet ./...`, `go build ./...`, formatting and diff checks.
- [x] Run original and RC1–RC6 reviewer controls through test-only overlays.
- [x] Inspect outputs/citations across recorded repositories; no credentials,
  target-code execution or external network needed.
- [x] Record implementation, tests, review outcome and remaining G15/G16 limits;
  update this checklist.
- Dependencies: tasks 1–6. Scope: verification and documentation.

## Risks and decisions

- Reservations must permit borrowing without displacing another peer's
  decisive evidence with ordinary declarations.
- Supporting directory names remain hints. Parsed code and typed role evidence
  supply the competition decision.
- Extra role citations enlarge proofs; an unavailable basis requires abstention.
- Incomplete Go listings may now show unknown instead of library. This is
  intentional and needs no additional GitHub requests.
- Preserve unrelated local edits and history. Do not commit local research or
  process documents, or access secret configuration.

## Result

RC1–RC6 are complete. Maintained gates and combined independent controls pass;
see the [implementation and process record](../docs/reviews/2026-11-04-g17-six-issue-implementation/README.md).
Independent review caught and resolved a no-import root-package relevance
regression. Remaining G15/G16 classifier limits are recorded separately.

Saved as local code/test commits `3eed2f2`, `708cf44`, `29384c1` (final head).
No push, deployment or tracker update was performed. Plan/process documents
remain local, alongside the preserved pre-existing documentation edits.
