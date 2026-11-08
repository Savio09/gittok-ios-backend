# G17 repair implementation — November 4, 2026

Local process record (not committed, per the owner's no-documents rule).
Implements [the repair handoff](../../tasks/g17-review-repairs.md) against the
[November 4 product-readiness review](2026-11-04-g17-product-readiness/README.md).
The November 2 source audit remains accurate and unchanged: its claims were
sourced. This repair addresses what that audit did not measure — usefulness,
kind explanation, displayed limits and request economics.

- Branch: `fortune/sav-23-g17-extract-claims-link-each-to-source-filemetadata` (not pushed)
- Base: `82eb571` · Final head: `d0bd691` · 12 code/test commits, no documents
- Uncommitted by design: `docs/api-contract.md` (updated), `tasks/plan.md` (owner's edit), this record

## Baseline (T1)

The five archived November 4 controls were replayed unmodified on an isolated
copy of `82eb571`; all failed exactly as recorded (CLI: 9/9 helper/product
declarations, cobra missing, helper HTTP gateway shown; PocketBase: 20
declarations without lines; CLI kind unexplained; 1,126-byte diagnostics; two
identical previews: 4 metadata, 2 tree, 2 contents reads).

## R1–R6 → change → maintained tests

| Finding | Commits | Maintained tests |
|---|---|---|
| R4 Go/JSON positions | `0c3a572`, `de41f35` | `gomod_location_test.go`, `json_location_test.go`, `TestReviewGo/JSONDeclarationsCiteExactLocations` |
| R3 kind explanation | `6c92bcb`, `80dce9f` | `basis_test.go`, `kindproof_test.go` (`TestReviewKindExplanationCitesEveryPremise`, provenance, bounded absence, comparison) |
| R1/R2 relevance | `05ed27d`, `d815f64`, `df38196` (README/script refinement) | `products_test.go`, `relevance_test.go` (CLI, PocketBase, aggregator, dev tools, tool-only path, unknown ecosystem, import boundaries, permutations, rename) |
| R5 limits | `df38196` | `coverage_test.go`, updated `partial_test.go` (paths stay in internal diagnostics) |
| R6 economics | `28eb058`, `4058163`, `685b4d2`, `2e3e91c`, `d0bd691` | `outcome_test.go`, `policy_test.go`, `public_test.go`, `cache_test.go`, `cache_service_test.go`, `coalesce_test.go`, `cmd/server/preview_test.go`, `config_test.go` |

## Evidence

**Source accuracy** (independent Python audits, separate from usefulness):
Checkpoint A — 96 line-cited claims across the six corpus snapshots, 0 byte
mismatches; Checkpoint B — 107 sources across 8 repositories, 0 bad; live —
12/12 cli/cli line sources match the pinned raw files. Only Cargo.toml (no TOML
locator) still cites whole files.

**Usefulness** (explicit criteria: kind proof leads when resolved; purpose
present; ≤8 declarations, ≤2 lower-priority; no helper HTTP call; ≤2 helper
claims; ≤3 plain notes): all 8 pass — cli/cli, PocketBase, Glow, Rack (held
out) and starship, excalidraw, plantly, face-recognition (captured before this
work, not used for tuning). Reading the output also found two problems the
counts missed (supporting-path READMEs, non-setup scripts); both were fixed.

**Abstentions** (reported separately): PocketBase (`cmd/serve.go` not in the
default selection — a G16 limit), excalidraw, Rack (Ruby, no supported
manifest), face-recognition. A kind is also withheld when a listing premise has
no recorded tree request (synthetic inspections).

**Before → after, cli/cli live:** 24 claims (17 declarations, 9 from `.github`
helpers, no cobra, a `script/` HTTP call, 1,126-byte limits) → 12 claims: kind
proof citing `cmd/gen-docs/main.go:1`, `cmd/gh/main.go:1`, `go.mod:50` (cobra);
README; description; 8 product declarations with lines; one note.
**PocketBase live:** 20 unpositioned declarations including indirect ones → 6
direct module declarations + 2 UI declarations, UI build/dev at `ui/package.json:6-7`.

**Request economics:** cold = selected files + 4 (metadata, ref, tree,
identity recheck): 20 live for cli/cli, 18 for PocketBase; a warm hit = 1
metadata request and 0 ref/tree/contents (live and fake-HTTP); identical
concurrent misses share one acquisition (deterministic, race-tested 20×).
Retries (≤3, transient only) can add requests; an expired entry reacquires at
the current revision.

**Negative controls passing:** private-on-hit, unknown visibility, rejected
credential, reused name, rename, transfer; transient file failures and
withheld identity never cached; policy rejection at ref/tree/contents/recheck
yields no preview; missing/empty/duplicate/privileged/changed scope headers;
unsupported token families; 401 revocation; cross-origin redirects; caller HTTP
client not mutated; corrupt/oversized/foreign/expired/other-version entries;
symlinked cache directory; last-waiter cancellation; late old generation.

**Archived controls on the final tree:** Nov 4 — 4/5 pass unmodified; the
repeat-cost control constructs a bare `Service` without `UseCache`, so it still
repeats acquisition. The cache is opt-in and enabled by the server wiring; its
maintained replacement and the HTTP test pass. Earlier reviewer controls: 32/32.

**Gates (once, after the last change):** `go test -count=1 ./...`,
`go test -race -count=1 ./...`, `go vet ./...`, `go build ./...`,
`git diff --check 82eb571..HEAD` — all pass.

## Rejected approaches

Parsing classifier prose for ranking or limits; regex rewriting of diagnostics;
text search for manifest positions; a zero-value-cacheable outcome (caught by a
test: an empty `Result` would have been cacheable); caching by name alone;
inheriting the feed token; defaulting every `Service` to a disk cache.

## Remaining limits

- **Authenticated public mode — verified live (Nov 4, after the owner added the
  token):** a mistyped 41-character token was rejected and startup stopped (exit
  1, no anonymous fallback); the corrected token returned an empty
  `X-OAuth-Scopes` header and a 5,000/hour quota, and startup reported the
  verified public-only mode. Live, cold previews used ≤20 GitHub requests
  (cli/cli, PocketBase, fzf, express, fastapi), warm repeats exactly 1 with a
  byte-identical body, and 5 concurrent misses on sharkdp/bat 24 in total (one
  shared acquisition plus 4 extra visibility checks).
- **Classifier coverage on fresh repositories is weak (G15/G16, not this repair):**
  fzf abstains (own-module imports not followed), express abstains (npm default
  `index.js` entry not recognised), bat abstains (a `tests/` fixture
  requirements file blocks the CLI verdict — classification does not treat
  `tests/` as supporting), and fastapi resolves as backend from one `starlette`
  dependency (a framework library). Sources in all of them verified 100%.
- TOML/YAML declarations still cite whole files.
- PocketBase/excalidraw abstain because default selection misses decisive files (G16).
- The cache is per process and on local disk; Render's filesystem is ephemeral.
- No strict total network-byte or CPU budget (G19).
- iOS must render `additional_sources`; JSON compatibility alone does not show
  every citation to a reader.

## Testing an arbitrary repository

`GITHUB_TOKEN=… [GITHUB_PUBLIC_TOKEN=<no-scope classic PAT>] PORT=18080 go run ./cmd/server`
then `GET http://localhost:18080/v1/repos/<owner>/<name>/preview` (or
`<owner>%2F<name>`). In Postman: a GET to that URL; send it twice to see the
warm response (one metadata request, ~30 ms).

## Swarm-review repair pass (November 4, later)

The [independent swarm review](2026-11-04-g17-swarm-review/README.md) of `d0bd691`
returned "changes required" with nine P2 findings; all nine reproduced on that
head before any change. Fixed on the same branch, head `6af0e93` (6 commits):

| Finding | Commit | Fix | Maintained test |
|---|---|---|---|
| SW1 NUL-key location collision | `e46929e` | structural (quoted-segment) location and ranker keys; structural duplicate purge | `TestJSONLocationKeysCannotCollide` |
| SW2 no-command proof omitted clause files | `7eb1c56` | `go.no-command` scope names every consulted root/cmd/ file; validator re-derives clauses; proof cites them | `TestGoNoCommandPremiseNamesEveryInspectedClause`, `TestKindProofCitesEveryConsultedPackageClause` |
| SW3 comparison dropped exclusion provenance | `d0c3585` | basis records each excluded role-bearing component (dependency + declared name, or supporting path); proof cites or withholds | `TestComparisonBasisRecordsEachExclusion`, `TestKindProofCitesEachSupportingExclusion` |
| SW4 truncated listing → confident absence | `21a3297` | npm library premises need a complete listing (`complete-listing`); display gate checks coverage | `TestNPMLibraryAbsenceNeedsACompleteListing`, `TestKindProofAbstainsWhenTheListingCannotShowAbsence` |
| SW5 visibility work outside admission | `4a9855c` | callers admitted at entry (MaxWaiters); refused callers make no GitHub request | `TestAdmissionRefusesExcessCallersBeforeContactingGitHub` |
| SW6 cache/flight handoff reacquired | `4a9855c` | leader work rechecks the cache; a hit is served without re-storing | `TestALeaderRechecksTheCacheBeforeAcquiring` |
| SW7 abstention invented a primary | `6af0e93` | only a resolved verdict has a primary; candidates are peers with an equal dependency quota | `TestInconclusiveProductCandidatesArePeers` |
| SW8 helper imports promoted declarations | `6af0e93` | import relevance ignores supporting-file imports (tool-only fallback kept) | `TestHelperOnlyImportsDoNotPromoteDeclarations` |
| SW9 README demoted a tool product | `6af0e93` | documentation is not competing product evidence | `TestAReadmeDoesNotDemoteTheOnlyProduct` |

Re-verification after the last change: all eight preview-side reviewer controls
pass unmodified (output paths redirected so the archived JSON stays untouched;
the two files rewritten by the baseline reproduction were confirmed
byte-identical to the reviewer's). The reviewer's SW5 control cannot complete
on fixed code by construction — it waits for 80 upstream visibility requests to
start, which the fix prevents — so its ported version asserts the outcome
directly; the SW6 control's overlay replaced `preview.go` with a stale copy, so
its ported version uses an equivalent test hook. Full gates pass (test, race,
vet, build, diff --check, gofmt); earlier controls 32/32, Nov 4 preview
reproductions 4/4, swarm security controls pass under -race; independent audit
of 8 repositories: 0 bad sources, no usefulness violations. Live with the public
token: cli/cli cold 20 / warm 1 requests (identical body); 5 concurrent
sharkdp/fd requests 20 in total.

Still tracked separately (pre-existing G15): a packaged Python library resolves
as backend from one starlette dependency ([boundary evidence](2026-11-04-g17-swarm-review/sources/library-boundary.md)),
and fzf/express/bat abstain for the classifier reasons recorded above.
