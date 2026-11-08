# G17 repair implementation handoff

> **Implementation status (November 4, 2026):** T1–T12 implemented on the G17
> branch, `82eb571` → `d0bd691` (12 commits; not pushed, not merged). Return
> packet: [implementation record](../docs/reviews/2026-11-04-g17-repair-implementation.md).
> Authenticated public mode verified live on November 4 (token added by the owner).
> The independent swarm review (§6) ran on `d0bd691` and required nine fixes
> (SW1–SW9); all are fixed and re-verified at `6af0e93` — see the record. A
> follow-up independent review has not run. The text below is the original
> handoff, unchanged.

Prepared November 4, 2026. **Planning only: repairs are not implemented.**
This document is self-contained instructions for the implementing agent. After
implementation, a separate agent swarm will independently vet the changes. A
passing implementation-agent report is input to that review, not approval to merge.

## 1. Objective, baseline and scope

Make GitTok's previews useful, explainable and practical for a demo while preserving
accurate sources and public-repository access isolation. The same pipeline must
work for arbitrary public GitHub repositories. Study repositories are evaluation
inputs, never a whitelist or conditions in product code.

Known baseline: `82eb571`, branch
`fortune/sav-23-g17-extract-claims-link-each-to-source-filemetadata`.
Before editing, inspect current status/history and agent instructions: another
agent may have advanced this branch. Preserve unrelated/untracked work, including
historical reviews and corpus documentation. Do not restart the merged classifier
repairs or bulk-stage the workspace. Work in focused commits; no merge, deployment,
external messages or Linear/Notion changes are authorized by this handoff.

Linear remains the tracker: G17/SAV-23 is claim extraction; G18/SAV-24 covers broader
uncertainty; G19/SAV-25 covers comprehensive configurable acquisition budgets.
These repairs belong to the G17 implementation. They include the bounded cache and
credential handling needed to address the reported quota failure, not a complete
G19 implementation. Do not create a competing todo database.

Read these local sources first:

- [Reproduced findings and recorded results](../docs/reviews/2026-11-04-g17-product-readiness/README.md).
- [Original G17 design](../docs/g17-claim-extraction.md) and
  [November 2 process report](../docs/reviews/2026-11-02-g17-claim-extraction.md).
- [API contract](../docs/api-contract.md), `internal/preview`, `internal/crawler`,
  `internal/analysis`, `internal/githubclient`, `internal/config`, `cmd/server`.

Apply the available planning, interface design, source-driven development, TDD,
incremental implementation, security, doubt-driven review, code-quality review,
git workflow and documentation skills. Read the applicable SKILL.md files before
using their workflows. Existing user authorization takes precedence over skill
approval conventions. No external model CLI or paid service is part of this plan.

## 2. Failures to close

| ID | Reproduced problem | Required result |
|---|---|---|
| R1 | Dependency selection is mostly lexical; Cobra is omitted; development/indirect declarations occupy useful slots | Preserve classification-relevant and product dependencies before lower-value declarations; bound dependency output |
| R2 | CLI helper manifests get 9 slots; a helper HTTP gateway is the prominent source construct | Main-product evidence leads; supporting tools cannot stand in for the product walkthrough |
| R3 | The card's resolved kind has no visible supporting explanation | The displayed kind has a fully sourced explanation covering every necessary premise |
| R4 | 20 selected PocketBase declarations lack available positions, including UI scripts | Exact Go/JSON declaration locations and valid inclusive line ranges |
| R5 | The CLI limits section contains 1,126 bytes of internal diagnostics | Short, accurate user-facing coverage notes with detailed diagnostics retained internally |
| R6 | Identical previews repeat tree/content acquisition; anonymous quota allows only a few cold previews/hour | Bounded reusable caching, request coalescing, and safe optional authenticated public reads |

The source-accuracy criterion passed earlier. That does not close R1–R6. Preserve
both facts in documentation rather than rewriting the earlier source audit.

## 3. Module and data-flow contract

Keep dependencies one-way:

`HTTP/command → analysis orchestration → GitHub client + crawler + preview builder`

The preview builder stays pure, with no network, current-clock reads, environment
or model calls. Capture time is an explicit input.
Crawler owns parsed facts, component scope, witness validation and classification
evidence. GitHub client owns credential-response validation. Analysis owns outcome
eligibility, caching and shared work. Server/config own environment wiring.
The feed remains metadata-first; full preview analysis is on demand.

### A. Product relevance and claim budgets

Use the classifier's selected product component and validated observations as
inputs to ranking. Expose a small typed internal result if needed; do not parse
human `Observation.Detail` or limitation strings for decisions. For inconclusive
classification, preserve plausible product components without arbitrarily choosing
the first sorted component as the product.

Order evidence by semantic priority, then deterministic path/name tie-breaks:

1. Complete kind explanation and attributed product purpose/README.
2. Facts used by the product classification, product entry declarations and
   relevant setup/build commands.
3. Product dependencies referenced by inspected product imports, then other direct
   product declarations. Import matching must respect module/path boundaries.
4. Secondary product components, retaining their ownership and role.
5. Optional/development/peer/indirect declarations and supporting tools, when useful
   and clearly scoped. Path names are ranking hints, not proof of runtime purpose.

Reuse the existing component inventory and scope policy. Account for `.github`,
CI, examples, tests and script/helper paths within a root component, not just
nested manifest roots. A repository whose actual product is a tool must still
work; do not globally ban folders or every development dependency. Do not add
repository-name rules or a second divergent classifier.

Initial selection policy: retain the overall maximum of 24 claims, at most 8
dependency claims total, and at most 2 lower-priority optional/dev/peer/indirect
dependency claims. Reserve the kind explanation, useful purpose and available
product command evidence before allocating the dependency budget. Prefer up to
2 useful metadata claims; put stars/license/last-push in sourced metrics where
appropriate instead of spending six claim slots on card metadata. Never label
`pushed_at` as a verified latest-commit time. Missing values do not become zeros.

These are bounded defaults, not a requirement to fill slots. A shorter useful
preview is valid. Deduplicate before ranking/caps, retain relevant independent
components, and use lexical order only to break genuinely equal priorities.

### B. Source locations and kind explanation

Keep existing `Claim.Source`, `Source.Ref`, `Source.Lines`, `Source.URL` and source
kinds compatible. Add only optional `Claim.AdditionalSources []Source` serialized
as `additional_sources,omitempty` if required for complete kind proofs. Update
the API contract and compatibility tests in the same slice. Adding a slice makes
`Claim` non-comparable: replace `map[Claim]` deduplication with an explicit stable
key; do not use ambiguous formatted strings as the equality definition.

Emit a reserved claim such as “Why this is a CLI,” framed as a classification from
the inspected evidence. Its primary and additional sources collectively cover the
whole validated basis. Do not concatenate untrusted observation prose or present
one dependency as sufficient for a conclusion needing several premises.

| Witness | Required representation |
|---|---|
| Parsed field | Consumed manifest, actual field, verified optional span and pinned file URL |
| Source construct | Exact validated construct span and pinned file URL |
| Tree listing | The retained pinned tree evidence/request and relevant paths; no invented file quote |
| Bounded absence/scope | The recorded inspected scope, completeness premises and all required consumed-file/tree sources; never repository-wide absence from a bounded read |

Preserve acquisition provenance needed to distinguish a captured tree/query from
a synthetic listing. A plausible API/search URL is not proof that that query ran.
Do not manufacture search records, excerpts or live-request provenance to satisfy
a golden. If any necessary premise cannot be represented and verified, withhold
the resolved display kind, use `other`, and give a concise uncertainty note. Keep
the internal classifier result/diagnostics for review. `other` is abstention, not a
positive assertion that the repository lacks a supported product type.

Use `modfile.Require.Syntax` positions from the existing x/mod parser for Go
requirements, preserving declared versions and indirect context. Locate JSON fields
in original bytes with a position-aware pass consistent with the existing parser.
Handle escaped keys, duplicate keys, nested same-name fields, CRLF, Unicode and
multiline values. A duplicate-key location must identify the effective parsed value,
not an earlier overwritten value. Do not locate values through naive text search.

Enrich field witnesses without changing their semantic validation: rederive the
field/value and verify that any claimed span belongs to that field. Excerpts stay
exact contiguous UTF-8 source bytes. Lines are one-based, inclusive; account for
end-exclusive parser spans. Other supported formats may retain whole-file citations
when reliable positions are unavailable. Never execute scripts or target code.

### C. User-facing limits

Do not serialize raw classifier diagnostics into walkthrough prose. Use structured
inspection/outcome state to summarize the actual gap, for example: “Some selected
files could not be read,” “This preview covers part of a large repository,” or
“The project type could not be confirmed from the inspected files.”

Normally render at most 3 distinct short statements, roughly 500 characters total.
Combine related causes while preserving a material privacy, identity or missing-
evidence gap. Keep paths, parser details and technical reasons in internal results
and sanitized developer diagnostics. No regex rewriting of prose as a substitute
for a typed state. No fake uncertain claim/source just to display a generic gap.

### D. Public credential policy

Add optional `GITHUB_PUBLIC_TOKEN`, separate from the feed's `GITHUB_TOKEN`.
The initial supported authenticated mode is a classic PAT with no OAuth scopes,
which GitHub documents as read-only public access. Never inherit the feed token,
accept a token from a repository ID/request, or silently downgrade a rejected
configured credential to anonymous mode. No new login/private-access flow is added.

Validate credential family and actual granted-scope evidence at startup, then on
every successful API response before decoding repository data. A prefix alone is
not sufficient. Distinguish a present empty `X-OAuth-Scopes` value from an absent
header, and validate every value, including duplicate headers. Reject missing,
nonempty/privileged or unsupported evidence. `X-Accepted-OAuth-Scopes` describes
endpoint requirements; it does not establish granted permissions. Fine-grained
PATs and app/user credentials require a separately designed policy; reject them
in this mode rather than guessing from an empty/missing scope header.

Use a typed terminal credential-policy error. It must propagate through retries,
ref/tree/content acquisition, final identity checks and cache orchestration.
`GetFileContents` must not turn it into a skipped file; `Acquire` must not turn it
into metadata-only success. If it occurs after earlier successful reads, return no
preview and make no cache write. Redact secrets and raw provider diagnostics.
Restrict credential-bearing redirects to the trusted API origin; preserve caller
HTTP-client configuration without mutating a shared client in place.

An empty `GITHUB_PUBLIC_TOKEN` retains explicit anonymous fallback for development.
Report the operating mode safely in startup diagnostics. Do not call that mode
demo-ready without explaining its shared-IP quota. The implementation agent must
not create, request in chat, print or commit a real token; the operator configures
it through the deployment secret settings.

Use an existing client request/decode boundary for scope checks so no acquisition
method bypasses them. Close rejected response bodies promptly. Startup validation
is not permanent permission evidence: changed scopes must be caught on later reads.

### E. Acquisition outcome, cache and shared work

Introduce explicit typed acquisition outcome/eligibility. Distinguish successful
bounded inspection from transport/provider/deadline failure, missing/changed
identity and policy rejection. Unsupported classification and declared input caps
can be legitimate bounded results. A nil Go error is not sufficient for caching:
the baseline also returns nil-error results after identity failure or failed reads.

For each public preview, resolve current metadata and verify public visibility,
positive stable repository ID and canonical owner/name before serving cached data.
On a miss, reuse that verified metadata in acquisition if practical. Retain the
post-acquisition identity check. An alias/rename/transfer must not leave cached
source links pointing at a subsequently reused name.

Cache identity: stable repository ID + canonical owner/name + builder/schema
version. Store the resolved commit, capture time and expiry in the entry. A warm
lookup need not resolve the current branch again: within TTL it intentionally
serves a captured snapshot with immutable file links and its original metadata
capture time. Do not blend fresh values into old claims without rebuilding their
provenance. On expiry, reacquire and pin the current revision. Bump the builder
version whenever ranking, schema or claim semantics invalidate stored results.

| Operational default | Initial value |
|---|---|
| Cache TTL | 30 minutes; configurable with `PREVIEW_CACHE_TTL` |
| Cache directory | `PREVIEW_CACHE_DIR`, default a dedicated directory under `os.TempDir()` |
| Maximum entries / total bytes | 128 entries / 32 MiB |
| Maximum serialized entry | 512 KiB; oversized entries are returned uncached |
| Maximum concurrent distinct work | 2 acquisitions per process |
| Maximum joined callers | 32 per key, 64 total |
| Queue | None; fail excess admission with 503 and a short Retry-After |
| Shared operation deadline | 30 seconds, independent of the first caller |

Keep limits injectable internally for deterministic tests. G19 can broaden external
budget configuration later. Validate positive TTL and finite capacities. Use a
dedicated owned cache directory, bounded reads/writes, atomic replacement, finite
expiry/eviction and safe handling of corrupt/unknown-version entries. Store public
previews/provenance only, never credentials, private contents or full provider
payloads. Restart persistence is tested; durable Render persistence requires an
explicit persistent mount, not an assumption about its ephemeral filesystem.

Coalesce same-repository requests before expensive work; recheck canonical identity
before cache use. Use a shared bounded context, not the first caller's lifetime.
One departing caller must not cancel remaining callers. When the last caller leaves,
cancel acquisition and remove that generation before a new caller can join it.
Bound active keys and waiter state as well as disk storage. Prevent late completion
of an old generation from deleting or overwriting newer work.

Revalidate public visibility on hits; do not fall back to cached data on failed
visibility/identity/policy checks. No caching of private, fatal, transient-failure,
deadline or identity-invalid results. Send `Cache-Control: no-store` on success and
error responses so intermediaries cannot bypass application checks. Visibility is
verified at the lookup instant; do not promise atomic public visibility afterward.
Preserve the existing JSON error shape and status semantics; use 503 for capacity
or credential-policy unavailability. Keep cancellation from writing fake success.

## 4. Ordered execution tasks

All tasks are pending. Implement/test one slice at a time. Split a task further if
it grows past approximately 5 files or needs a broad parser/orchestration rewrite.
Suggested filenames below are boundaries, not requirements to create abstractions.

### T1 — Establish maintained regression controls

Depends on: baseline inspection. Scope: tests, medium.
Files: new preview relevance tests, analysis cost tests, corpus test adapter.
Use the archived November 4 controls to capture the baseline failures. Promote each
control into maintained tests immediately before its corresponding repair, then
make it pass in that slice. Do not commit unrelated permanent failing tests, hide
them behind skips, or delete their assertions to obtain a green suite. Keep the
other baseline failures archived until their owning slice. Use production-selected
corpus files; do not inject extra files.

Acceptance: baseline failures are observed for displaced Cobra/helper gateway,
missing Go/JSON lines, unexplained kind, verbose limits and repeat acquisition.
Record output before fixes. Add independent usefulness assertions beyond goldens.
Verify: focused `go test -run 'Review|Relevance|Location|Cache' ./internal/preview ./internal/analysis`.

### T2 — Go declaration positions

Depends on: T1. Scope: crawler positions/witnesses/tests, medium.
Reuse existing parser syntax positions and per-inspection caches. Preserve versions,
indirect flags and component ownership. Avoid reparsing an entire manifest per fact.
Acceptance: exact locations for grouped/single-line requirements; wrong-field,
wrong-span, modified quote and clipped/malformed declarations cannot gain validity.
Verify: focused crawler tests, then `go test ./internal/crawler`.

### T3 — JSON declaration positions

Depends on: T2 witness contract. Scope: JSON locator/tests and preview sources/tests.
Acceptance: actual UI build/dev lines and dependency fields carry exact positions;
escaped/duplicate keys, nested lookalikes, multiline/CRLF and Unicode controls pass.
The source renderer supports positioned fields without weakening semantic checks.
Verify: `go test ./internal/crawler ./internal/preview`; inspect real source bytes.

### Checkpoint A — precise provenance

All old tampering, truncation, UTF-8 and crawler regression controls still pass.
New excerpts/lines independently match the original consumed files; the location
lookup must not merely agree with itself through shared helper assertions.

### T4 — Complete kind-proof contract

Depends on: checkpoint A. Scope: split crawler evidence exposure and API/rendering
into two tested substeps. Files: crawler evidence/witness API and tests; claim model,
preview proof renderer/tests and API documentation in a separate substep.
Acceptance: resolved displayed kinds have complete verified proof sources; field,
span, listing and bounded absence cases work. Missing one necessary premise cannot
retain an unsupported resolved display. Old JSON still decodes; ordering/dedup
handles the optional source slice correctly. All generated evidence is not_executed.
Verify: `go test ./internal/apimodel ./internal/crawler ./internal/preview`.

### T5 — Product relevance and bounded selection

Depends on: T4. Scope: ranking/claims/tests, medium per substep.
Compute typed priorities, reserve kind/purpose/commands, apply family caps, then
deterministic tie-breaks. Preserve direct/indirect/dev/optional distinctions.
Acceptance: CLI includes its decisive Cobra evidence; helper HTTP gateway does not
stand in for its product; PocketBase's useful main/UI evidence survives. Test root
aggregators, nested apps, library development tools, a product under a tooling-like
path, unknown ecosystems, input permutations and renamed repositories. No rule may
inspect owner/repository names. Replace tests that require 24 dependency-filled slots.
Verify: `go test ./internal/preview ./internal/crawler ./internal/analysis`.

### T6 — Concise coverage presentation

Depends on: T5. Scope: builder/outcome summary/tests, medium.
Acceptance: ordinary gaps show short understandable notes without `.github` paths
or classifier jargon; exact diagnostic causes remain internally available. Missing
positions do not imply missing evidence; incomplete evidence does not imply absence.
Verify: focused preview/partial-result tests and a manual payload inspection.

### Checkpoint B — useful and supported

Independently audit sources AND usefulness on CLI, PocketBase, Glow and Rack.
Do not update goldens until the changed output passes explicit selection criteria.
Keep historical raw captures unchanged; document any newly acquired evidence.

### T7 — Explicit outcome and terminal policy errors

Depends on: T1; implement before caching. Scope: client error type/content handling
and analysis outcome tests, medium per substep.
Acceptance: cache eligibility is deliberate; identity-invalid/deadline/transient
results cannot masquerade as eligible success. A terminal policy error during ref,
tree, any contents read or identity recheck returns no preview/cached partial data.
Verify: `go test ./internal/githubclient ./internal/analysis` with injected failures.

### T8 — Safe public credential client

Depends on: T7. Scope: client validation/tests, medium.
Acceptance: supported no-scope credentials work; missing vs empty header, duplicate
empty/nonempty values, private scopes, changed scopes, unsupported token type,
invalid authentication and cross-origin redirects fail safely before data decode.
Test revocation/scope expansion after earlier successful reads. No token literals
or sensitive headers appear in output, fixtures, diagnostics or persisted state.
Verify: GitHub-client tests with fake HTTP; recheck the linked official docs.

### T9 — Bounded persistent cache

Depends on: T7, T8. Scope: analysis cache/store module and tests, medium.
Acceptance: cold/warm/expired/version-changed cases, restart reuse, bounded eviction,
corruption/oversized entries and atomic writes work. Public/ID/canonical-name checks
gate hits; private, changed-ID, renamed/transferred/reused-name and failed-visibility
cases never serve old data. Unsupported but successfully inspected repos can cache.
Verify: deterministic fake-clock/temp-directory tests; no ordinary live network.

### T10 — Coalescing and bounded admission

Depends on: T9. Scope: shared-work orchestration/tests, medium.
Acceptance: simultaneous same-key callers share one analysis; a follower survives
leader cancellation; last-waiter cancellation stops work; the next request gets a
fresh generation. Distinct-key overload and waiter limits return bounded errors;
late completion cannot remove newer generations. The race suite passes.
Verify: barrier/channel-based tests without sleep-based timing assumptions,
then `go test -race ./internal/analysis`.

### T11 — Server configuration and HTTP behavior

Depends on: T6, T10. Scope: config/examples/tests, then server/API docs as a second
substep. Wire the public-only credential and cache independently from feed auth.
Acceptance: documented defaults and invalid configuration are tested; configured
unsafe credentials fail rather than downgrade; anonymous mode is explicit. Real
ServeMux tests verify IDs, errors, overload, cancellation and no-store headers.
The feed never performs preview analysis. No real credential is needed for tests.
Verify: `go test ./internal/config ./cmd/server ./internal/analysis ./internal/feed`.

### Checkpoint C — request economics and isolation

Record HTTP request counts by endpoint, not fake method-call counts alone:
unretried cold analysis stays at the documented bounded path (normally no more
than selected-file-count + 4 metadata/ref/tree/revalidation requests, excluding
one-time credential setup); a valid warm hit uses at most one visibility lookup
and zero ref/tree/contents calls; simultaneous identical misses share one pipeline.
Explain any extra request, retries and TTL refresh behavior explicitly. No private
or credential-policy failure may produce a successful stale-cache response.

### T12 — Independent acceptance and handoff

Depends on: all preceding tasks. Scope: tests, samples and process documentation.
Run full gates once after the last code change:

```sh
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go build ./...
git diff --check
```

Use required sandbox permissions for loopback/network tests; report blocked checks
accurately. Re-run only for new changes/failures. Ordinary tests stay offline.
Any live capture is opt-in, uses normal selection without manual extras, records
commit/provenance and stays separate from historical snapshots.

## 5. Required implementation return packet

The implementation agent must return:

- Branch, base SHA, exact final head SHA, focused commit list and worktree status.
- A compact R1–R6 → changed files → regression tests → result mapping.
- Before/after preview JSON for CLI and PocketBase, plus replay results for Glow,
  Rack and at least one new repository/layout selected before tuning. Keep source
  accuracy, relevance/coverage and abstentions as separate reported measures.
- A claim-by-claim audit covering primary AND additional sources, quotes/lines,
  classification premises and unchanged runtime-evidence guarantees. Do not use
  the production validator as the only proof of correctness.
- Cold/warm/concurrent/expired request-count evidence, cancellation/race results,
  cache limits/persistence behavior and negative privacy/credential test results.
- Exact verification commands/results, config/Postman instructions, and remaining
  limitations. Show how a user tests an arbitrary owner/repository, not only fixtures.
  Document the Swift consumer's obligation to render additional proof sources;
  backward-compatible JSON alone does not prove the mobile UI exposes every citation.
- A new dated process record linking the November 4 review, failed baseline controls,
  actual fixes, rejected approaches and final evidence. Update the design/API docs
  and this plan's actual task status without erasing historical findings.

No “done” claim based only on green tests, matching goldens or two valid citations.
No claim of demo readiness unless the authenticated public mode and cache economics
were exercised; missing live credentials must be reported as an unverified operating
condition, not bypassed or fabricated.

## 6. Later independent swarm review

The reviewing agent will spawn separate reviewers after implementation:

| Reviewer | Independent responsibility |
|---|---|
| Product relevance | Reproduce R1/R2 on real captures and adversarial unfamiliar layouts; inspect ranking/caps without relying on goldens |
| Source and kind proof | Audit exact lines/quotes, complete multi-premise explanations, listing/absence provenance, API compatibility and fallback kinds |
| Privacy and credentials | Try token/header/redirect attacks, mid-flight policy failures, private visibility changes, identity reuse and cache disclosure |
| Cache and concurrency | Challenge persistence/TTL/eviction, request counts, admission limits, cancellation/generation races and degraded upstream behavior |
| Integrator | Reconcile findings against actual code, independently run full gates, check scope/history/docs and return a severity-ranked verdict |

Reviewers receive the contract and final artifacts with fresh context. They must
try to disprove the implementation; implementation-agent confidence is not evidence.
Run in waves if concurrency slots are limited. Fix required correctness/security
findings before acceptance. Scope or source uncertainty must be stated explicitly;
the review does not merge or deploy the branch.

## 7. Primary sources and implementation limits

Verify framework/provider details against the versions in `go.mod` before coding.
Relevant authoritative sources:

- [GitHub primary/secondary rate limits](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api).
- [Granted scopes, no-scope public access and scope headers](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/scopes-for-oauth-apps).
- [GitHub token families](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/about-authentication-to-github).
- [x/mod syntax positions](https://pkg.go.dev/golang.org/x/mod@v0.37.0/modfile).
- [JSON decoder offsets](https://pkg.go.dev/encoding/json@go1.25.5#Decoder.InputOffset).
- [Render outbound IP sharing](https://render.com/docs/outbound-ip-addresses).

Do not introduce an LLM provider, universal language resolver, target execution,
private-user authorization, database/queue platform or a broad framework upgrade.
The existing retained-byte/tree/file caps and acquisition deadline remain; this
repair does not establish strict total network-byte or pure CPU bounds. Record
that G19 limitation rather than hiding it behind cache hit rates.
