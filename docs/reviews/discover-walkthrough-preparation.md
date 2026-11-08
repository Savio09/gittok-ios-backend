# Discover preparation design review — November 7, 2026

Artifact: [system design](../designs/discover-walkthrough-preparation.md). Baselines: backend 69b94cf, iOS 93f51f0. Tracker: [SAV-55](https://linear.app/savios-workspace/issue/SAV-55/design-bounded-discover-walkthrough-preparation).

## Scope and method

Design-only review of the feed-to-generation boundary, execution scheduling, persistence, spending and native completion ordering. No runtime code, provider requests, new cloud resources, budget changes or server restarts were performed. A proposed bound is not a measured production result.

Claim under test: preparing likely Discover projects can reduce detail latency while preserving demand capacity, financial limits, source acceptance and public-identity checks. The artifact and constraints were given to two fresh-context adversarial reviewers without the author's reasoning. The owner previously chose the current reviewer rather than a cross-model escalation. Each reviewer completed an initial pass and two targeted rechecks; review stopped after the third cycle.

## Findings and reconciliation

| Finding | Why it matters | Requirement now in the design |
| --- | --- | --- |
| Ambiguous queued speculative-job state | A queued hint could be confused with a paid receipt and promotion could dispatch twice | Hints have no Investigation or funded job; competing probes install one pinned registry key atomically |
| Priority applied only to workers | A clicked job could still wait behind speculative upstream requests | Mutable demand priority at every bounded governor, including promotion of an existing waiter |
| Admission probes outside queue bounds | Concurrent duplicate POST requests could exhaust handlers before the job queue limit helped | Separate global/speculative probe bounds, bounded waits and overload responses |
| HTTP 200 source privacy represented only by a coverage note | Late source metadata could fail to revoke an already displayed story | Typed OutcomeIdentityWithheld mapping and an absorbing active-run privacy veto |
| Source 404/credential-policy 503 has no preview body | Adding reason codes only to previews would leave an important veto path unhandled | Add optional typed reasons to existing error envelopes and decode them into native events; preserve status/error strings and non-enumeration |
| Semantic Accepted does not prove cacheable acquisition | Current engine can accept partial evidence for one-shot delivery | Trusted CachePublication requires complete/cacheable bounded acquisition plus registry fence; partial ready never becomes a durable or memory hit |
| Cooldown persisted outside funding | Crash between funding and admission-journal writes could enable repeated starts | Funding transaction commits cooldown and rolling start marker before execution authorization |
| Per-receipt visibility epoch is insufficient for durable reuse | Expired/new receipts or a restart could reload a revoked body | Shared repository/credential generations, version fencing and persistent invalidation/control uncertainty handling |
| Ledger/migration initialization failure exits the server | A promised isolated generation outage could also stop Discover | Separate generation-only configuration and degraded initialization; walkthrough 503, independently healthy metadata continues, no ledger reset |

The final scheduler/native recheck found no concrete remaining contradiction in the typed error repair. The final accounting/privacy recheck found no concrete remaining contradiction in startup isolation or acquisition eligibility. These conclusions concern the written contract, not implementation correctness.

## Process documentation

This review changed the proposed solution before paid work or concurrent runtime changes. Separation of concerns is reflected in explicit ownership: discovery offers hints; the coordinator selects candidates; the scheduler owns leases/deduplication; the engine judges source claims; the ledger alone authorizes expenditure; the artifact store never grants access authority; native state coordinates one run. This is evidence for the proposal's HC/LO concern about cohesive modules and limited coupling, not a claim that implementation has already achieved it.

Current limitations remain explicit: two total workers is a canary proposal; provider quotas must be configured/observed; no fresh latency distribution or click/usefulness study exists; no Azure sandbox work is included. Initial generation may still take substantial time, and quality/visibility failures may withhold a walkthrough.

## Next review gates

1. Owner reviews the concrete design and canary defaults.
2. Translate approved increments into small Linear implementation tasks with dependencies and acceptance checks.
3. Prove scheduling, accounting/crash recovery, cache/privacy and native event ordering with deterministic fixtures before paid canary enablement.
4. Measure requested latency and unrequested cost; adjust preparation count only from those results.
