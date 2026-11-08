# Discover walkthrough preparation — proposed execution plan

Status: system design complete and independently reviewed; awaiting owner review. No runtime implementation or paid preparation has started.

Tasks are tracked in the existing Linear GitTok — Capstone (CP193) project. Current task: [SAV-55 — design/review](https://linear.app/savios-workspace/issue/SAV-55/design-bounded-discover-walkthrough-preparation). Create implementation issues from the increments below after owner review; they are proposed scope, not an approved execution backlog.

Read the [system design](../docs/designs/discover-walkthrough-preparation.md) and [review/process record](../docs/reviews/discover-walkthrough-preparation.md) first. Backend baseline69b94cf; native93f51f0. Branch: fortune/walkthrough-preparation-design. Preserve unrelated dirty files in normal checkouts.

## Decisions and dependency order

Keep metadata delivery independent of generation. Establish safe startup and durable reuse first, then funded concurrent demand execution, then limited speculation, then ready-first native presentation. Keep warmup disabled throughout the foundations. Do not lower evidence gates or hard-code example repositories.

The following increments describe acceptance and likely ownership. Split a stage into smaller tickets before editing if its actual diff would exceed five implementation/test files. New file names are proposals; use the repository's existing module boundaries.

| Stage | Result and acceptance | Likely files/modules | Depends on |
| --- | --- | --- | --- |
| A | Independently healthy metadata starts when generation config/ledger cannot initialize; walkthrough 503, no model calls/reset. Add baseline timing events without source/model prose. | cmd/server/main.go, walkthrough.go and startup tests; internal/config/walkthrough.go; separate focused instrumentation ticket | Owner review |
| B1 | Trusted CachePublication requires accepted output, pinned revision, bounded complete/cacheable acquisition and completion fence. Partial acquisition never reuses. | internal/walkthrough/contract.go; new cache publication/revision unit and tests | A |
| B2 | Bounded atomic artifact persistence survives receipt expiry/restart; shared visibility fences and durable invalidation reject stale/private/recreated results. Corrupt controls disable reuse. | New internal/walkthrough artifact storage/gate units and tests; jobs_journal.go only if necessary, separately reviewed | B1 |
| C1 | Explicit versioned ledger migration preserves all old accounting and unknown reservations; no automatic reset. | internal/generation/ledger_storage.go, new migration unit, storage/migration tests | A |
| C2 | Atomic job funding enforces global/class/headroom bounds; call reservations transfer funding without double debit. Unknown calls retain maximum; cooldown/start markers commit before authority. Crash fixtures prove ordering. | internal/generation/ledger.go, new funding/admission units and focused tests | C1 |
| D | Resolve target under bounded probes, then start execution deadline at dispatch; winning probe budgets carry forward, restart never replays active work. | internal/walkthrough/jobs.go, investigation.go and focused tests | A |
| E | Two leased workers with demanded queue first, at most one speculative execution; bounded queue/probes/expiry; atomic install and promotion starts one engine. | Extract scheduler/registry units from jobs.go and focused tests; service wiring as a separate ticket if needed | D |
| F | Existing engine requires funded-job authority and uses acquisition-eligible cache; fresh identity checking precedes every prose response. Deterministic concurrent demand proves accounting and cache gates together. | internal/walkthrough/jobs.go, openai.go and focused integration tests; cmd/server/walkthrough.go | B2, C2, E |
| G1 | Shared GitHub/provider governors bound physical requests/waiters and promote demand; speculation pauses on unknown/insufficient quota and 429. | Focused governor units/tests; integration with githubclient and walkthrough/OpenAI split into separate tickets | E, F |
| G2 | Successful current first-page refresh offers hints after releasing locks; feed never waits for disk/network/generation. At most top2 hints/one speculation, persisted cooldown/start gates and operator flags. | internal/feed/discovery.go and test; new coordinator unit/test; separate cmd/server wiring task | G1 |
| H1 | Add server-generated job identity and source success/error reason codes without changing existing statuses/error strings; non-enumerating unavailability. | internal/analysis/handler.go; preview outcome wire unit; walkthrough envelope unit and focused handler/contract tests, split as required | D |
| H2 | Native structured source/story tasks deliver validated ready first, preserve legacy validation and source fallback. Absorbing typed privacy veto and run/account fences cover every completion order. | APIClient.swift, wire/model unit, DetailStore.swift and focused tests; fixture contracts separately | F, H1 |
| I | Offline gates pass, demand-only concurrency is measured, then operator enables a tiny paid canary within existing ledger limits. Record real latency/cost/waste; disable on foreground degradation. | Evaluation/operations documentation and fixture results; no fabricated benchmark | G2, H2 |

## Checkpoints

- After A–B: metadata failure isolation and cache eligibility/visibility/restart fixtures pass with warmup off.
- After C–F: accounting/crash tests and concurrent-demand race checks pass; no duplicate provider execution and no quota renewal.
- After G–H: blocked fake generation cannot slow the feed; demanded work takes reserved capacity; native ready-before-source and privacy/cancellation permutations pass.
- Before I: implementation review, full appropriate checks, and explicit reviewed operator enablement. The design alone authorizes no paid canary.

Use Go focused tests, race tests, vet and build listed in the design; Postman fixture contracts; portable native tests plus simulator integration for DetailStore/UI behavior. Do not run paid model calls to test scheduling or UI. Do not repeat historical runtime clips as evidence of current automatic execution.

## Risks, rollback and unresolved measurements

The highest risks are duplicated spend, stale prose after privacy changes, upstream quota starvation and generation failure stopping metadata. Their acceptance scenarios are mandatory. One process owns the durable controls; deployment needs a private persistent volume. Horizontal replicas require a separate ownership design.

Proposed settings: two candidates, one speculative worker within two total, $2 speculative allowance inside the existing $50 window, $1/job and $2 foreground headroom. None were applied. Missing provider rate configuration disables speculation. Candidate usefulness, actual cold/warm latency and speculative waste require measurement.

Rollback uses flags/one-worker mode in the new schema-compatible binary. Do not restore an older ledger snapshot or run an incompatible old binary. Source preview remains the fallback and all screen counts stay evidence-derived.
