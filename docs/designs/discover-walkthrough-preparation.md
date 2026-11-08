# Discover walkthrough preparation — system design

Status: proposed for owner review; no runtime implementation or paid generation enabled.
Baseline: backend 69b94cf, iOS 93f51f0. One capability: reduce time from opening a project to an accepted walkthrough by preparing likely public projects ahead of demand.

## Outcome and limits

Discover must remain responsive and metadata-only. A server-owned coordinator may prepare a small number of leading candidates while the app displays the feed. Opening a project must reuse/promote the same job or accepted artifact instead of purchasing another generation.

This creates a head start; it does not make arbitrary repository analysis instantaneous. An artifact may already be ready, still be running, or be withheld because the evidence/review is insufficient. Source fallback remains usable. No quality gate is weakened for speed. This design is generic across public repositories and repository types, with no demo-repository branches.

No API key, spending authorization, model choice, priority override or generation loop belongs in a feed card. Clerk identity is not currently verified by the backend and is not assumed to protect public generation.

## Current evidence

| Observation | Source at baseline | Design consequence |
| --- | --- | --- |
| Real public discovery serves 25 items/page, caches pages for ten minutes and paces GitHub search | internal/feed/discovery.go; internal/githubclient/search.go; cmd/server/main.go | Do not start 25 model jobs per launch or per pagination request |
| Walkthrough service has a single FIFO worker and a 64-receipt registry | internal/walkthrough/jobs.go; cmd/server/walkthrough.go | Speculative work would otherwise sit ahead of clicks |
| Start creates Investigation before queueing; its absolute deadline already runs while queued | internal/walkthrough/jobs.go; internal/walkthrough/investigation.go | Separate queue lifetime from execution deadline without renewing source quotas |
| Deduplication includes repo identity, commit and a fixed source-story-v1 string | internal/walkthrough/jobs.go | Durable reuse also needs an output/quality revision fingerprint |
| Journal retains receipts, not accepted bodies; restart makes ready bodies unavailable | internal/walkthrough/jobs.go; internal/walkthrough/jobs_journal.go | Add a bounded persistent accepted-artifact store before broad speculative work |
| Ledger atomically reserves each provider call, retains unknown outcomes and is process-exclusive | internal/generation/ledger.go; ledger_storage.go | Reuse this owner; do not create one independent budget per worker |
| iOS currently waits for source preview before submitting walkthrough work | Sources/GitTokCore/Features/Preview/DetailStore.swift in iOS checkout | Ready-first detail delivery must avoid a slow source request hiding a warm story |

Historical accepted library/CLI controls took approximately 30–32 seconds; these are individual recorded examples, not current p50/p95 or a promise for other repositories. See docs/walkthrough-validation.md. Live corpus-wide latency, click distribution and prefetch usefulness have not been measured.

For illustration: a 40-second generation with a ten-second head start can still leave roughly 30 seconds of generation. With one speculative lane, the second candidate waits for the first until requested. A completed cache hit still needs a fresh visibility/identity check and delivery.

## Chosen architecture

One backend process continues to own the job journal and spending ledger. Initially allow two executing repository jobs: at most one speculative job, with capacity reserved for demand. This is two bounded workers in the existing generation service, not two independent services with duplicate ledgers.

~~~mermaid
flowchart TD
    A[App launch] --> B[Public Discover metadata]
    B --> C[Render feed immediately]
    B -. bounded nonblocking snapshot hint .-> D[Warmup coordinator]
    D --> E[Eligibility, cooldown and quota checks]
    E --> F[Demand-priority scheduler]
    G[User opens a repository] --> H[Resolve public identity and pinned commit]
    H --> I{Accepted artifact exists?}
    I -->|yes, fresh public check| J[Return accepted walkthrough]
    I -->|no| F
    F --> K[Atomic job funding and bounded execution]
    K --> L[Existing source inspection, writer and independent review]
    L -->|accepted and acquisition cacheable| M[Persistent artifact store]
    M --> J
    N[One authoritative spend ledger] --- K
    O[Shared GitHub and provider request governors] --- H
    O --- L
~~~

| Component | Owns | Does not own |
| --- | --- | --- |
| Discovery service | Ranked public metadata and its existing page cache | Calling the model or inspecting source inline |
| Warmup coordinator | Candidate policy, coalescing, age, cooldown and speculative eligibility | User navigation or its own provider loop |
| Scheduler/job registry | Atomic deduplication, bounded lanes, promotion, queue expiry and dispatch leases | Semantic story acceptance |
| Existing investigation/engine | One pinned source budget, sequential writer/tool/reviewer stages | Feed selection or cross-job concurrency |
| Accepted-artifact store | Bounded durable acquisition-eligible accepted envelopes and revision/identity keys | Permission to return stored prose |
| Ledger | Global/class limits, job funding and provider-call reservations | Retry policy inferred from network errors |
| Request governors | Shared rate-limit state, priorities, bounded physical concurrency | Additional spending authorization |
| DetailStore | One detail run, ready-first presentation and source fallback | Generating every Discover item |

No Redis, Azure VM or new queue product is required for this first slice. Horizontal replication requires a separate shared transactional ownership design; copying control files to multiple instances is unsafe.

## Launch and candidate policy

1. GET /v1/feed retains its existing metadata response and errors. Do not append accepted prose or make one source/visibility request per card.
2. On a successful current first-page metadata refresh, the feed offers one small snapshot event after releasing its cache lock. Cache hits, later pages, invalid cursors and failed searches do not create new refresh events.
3. Offer is bounded and nonblocking: no GitHub request, disk fsync, model call or wait for a worker in this request path. A full hint buffer may drop/coalesce hints; the feed still succeeds.
4. Coordinator considers at most the leading two eligible entries. One speculative job can execute at a time. Keep at most two queued hints; expire old hints rather than crawling unseen pages.
5. Revalidate current public numeric identity and resolve HEAD at warm dispatch. A hint is not trusted evidence. Skip a candidate already ready, in flight, cooling down, out of date, unfunded or quota constrained.
6. Enforce at most two new speculative starts per ten-minute interval, plus a durable per-repository cooldown of 30 minutes across commit changes. Defaults are proposed canary settings, not measured optima.
7. Never walk the next cursor automatically. Future viewport signals or personalized candidates are a separate measured increment with backend-authenticated, server-clamped admission.

This is event-driven preparation during real feed use. There is no periodic paid sweep while nobody uses the app. Prewarming before the very first feed search would require an independently scheduled discovery refresh, deliberately excluded from the initial slice.

## Identity, deduplication and promotion

Artifact/job key:
(repo numeric ID, canonical owner/name, pinned commit SHA, generation revision digest).

Revision digest is computed by trusted code from model/reasoning configuration and versioned writer, reviewer, output-schema, visual and source-selection/quality policies. It excludes secrets. A changed acceptance/prompt policy must not silently reuse an older story. Keep the existing canonical identity/visibility rules; repo name alone cannot survive deletion/recreation safely.

- A per-repository warm hint is provisional and has no Investigation, funded model call or client receipt yet.
- Foreground Start performs a small bounded identity probe. It checks the accepted cache and registry before creating a new receipt.
- Warm dispatch resolves identity under its own bounded execution/probe context, then competes for that same registry key.
- The registry atomically installs one job. A losing warm probe joins/discards its candidate; it does not start a second engine.
- A click while only a hint exists is ordinary foreground admission and removes/coalesces that hint. There is no queued warm-job state. If speculative probing races the click, atomic pinned-key installation chooses one job. Clicking an already dispatched warm job attaches demand and upgrades its request priority; no restart, second charge or second investigation.
- Financial origin is immutable. A job admitted speculatively stays charged to the speculative allowance after promotion. Demand/first-requested time are separate metrics.
- Cancellation of an individual poller or leaving a screen does not cancel shared server work. Explicit service shutdown does fence all execution.

## Scheduler and deadline contract

Proposed initial bounds:

| Control | Canary value |
| --- | --- |
| Executing repository jobs | 2 total |
| Speculative executing jobs | at most 1 |
| Foreground waiting receipts | at most 8, within the existing overall receipt bound |
| Queued speculative hints | at most 2 |
| Foreground queue age | at most 30 seconds |
| Warm hint age | at most 60 seconds |
| Identity admission probe | at most 10 seconds, separately bounded calls/bytes |
| Execution deadline | existing 120 seconds, starts at worker dispatch |
| iOS overall detail deadline | existing 180 seconds |

Each dispatch first chooses waiting demand. Speculation runs only when its lane is available, no demand waits, and all funding/quota gates allow it. Demand may borrow an idle speculative lane. One running speculative call is not cancelled merely to free capacity: cancellation does not prove it was uncharged.

Keep a worker lease until the actual task terminates, including a promoted warm job. Promotion cannot briefly free a lane and overbook another worker. Mutexes protect registry transitions, not network/model execution. Queue admission and cancellation must not send to a full channel while holding the registry lock.

A queued receipt contains a resolved target, not a live Investigation whose deadline is already ticking. Dispatch constructs the execution context. Winning admission-probe calls/bytes are carried into the job's cumulative source accounting; they are not reset on dequeue. Dispatch's fresh identity gate also consumes that job's remaining allowance. Duplicate request probes remain separately bounded by request/global governors.

Queue timeout is not a failed generation retry. It expires unstarted work without a provider call. Existing wire states remain; use failed/interrupted plus an additive reason code and a plain-language message. Do not add an unrecognized pending state that old clients treat as terminal.

## Spending and rate limits

The current approved capstone window is $50 total and $1 maximum/job; history includes unresolved outcomes. Neither value is changed by this proposal. Proposed canary speculative allowance: at most $2 inside that same window, retaining $2 of unallocated headroom for foreground work. Enablement requires explicit operator configuration after implementation review; this design does not start paid generation.

A background-class counter checked outside the ledger mutex is insufficient. One ledger transaction must enforce total exposure, speculative exposure, per-job bounds and foreground headroom together.

Add durable job funding/escrow in the authoritative ledger:

- Before dispatching a new job, reserve its configured maximum job cost. Queueing a hint or receipt does not reserve money.
- A provider-call reservation transfers capacity from that job's remaining funding to a specific call; it does not debit both amounts.
- Known settlement retains actual cost and returns unused call capacity to the job's remaining funding. On terminal completion, release only provably unissued job capacity.
- Unknown provider outcomes retain their full call reservation across cancellation/restart. Never release them because a browser disappeared.
- Speculative admission requires its complete hold to fit both the speculative sublimit and global budget after foreground headroom. Foreground work cannot spend money already committed to a call.
- Funding protects reviewer/correction capacity from other jobs consuming it mid-story. The engine still enforces existing request-level and cumulative limits. When job funding is enabled, its provider boundary requires a valid funded-job handle, so legacy unfunded paths cannot silently bypass the new rule.
- A versioned, explicit migration preserves old ledger identity, window, entries, ceilings and unknown reservations exactly. No automatic reset/new window. Fail closed if migration or storage validation fails.
- Retry/promotion cannot relabel speculative expenditure into foreground or bypass a per-window allowance.
- In the SAME durable funding transaction, record the speculative numeric-repository cooldown and rolling start-rate marker before returning execution authorization. A journal state update cannot authorize execution by itself. Crash after funding/before execution may conservatively consume a cooldown without spending; it cannot erase the cooldown. Missing/corrupt durable admission state disables speculation and requires reconciliation.
- On restart, provably unissued job capacity may be released only when no durable call reservation could have authorized the network effect. Any call reservation with unknown outcome remains charged at its maximum. Test crashes before/after funding, journal transitions, provider reservation, response and settlement.

Shared governors must cover all GitHub clients using the same credential/egress and all model calls for the configured provider. Per-job quotas alone do not protect a shared token. Bound physical GitHub concurrency (initially serialize a credential's core requests), prioritize demanded work, honor provider backoff and reserve observed core quota for foreground jobs. Search/core rate buckets remain distinct. Unknown quota health pauses speculation; anonymous mode does not perform paid prewarming in the canary. Do not infer available tokens from the user's ChatGPT subscription.

Request-priority mechanics are part of this contract, not an implementation detail:
- Use mutable demand priority keyed by job identity at every upstream waiting queue; immutable financial origin is never the scheduling priority. Promotion upgrades an already waiting request.
- Bound admission probes independently of queued jobs: at most four pending probes globally, at most one speculative probe, and a small bounded wait or immediate 503/Retry-After. No unbounded collection of POST handlers waits ahead of the eight-receipt queue.
- Bound governor waiters (initially at most 16 demanded and two speculative waits per upstream owner) and remove them on cancellation/queue expiry. A waiting/backoff task holds no physical HTTP permit. An already active HTTP request is allowed to finish and is not free to preempt.
- Initial provider physical concurrency is at most two, speculation at most one; demand can borrow idle capacity. Before a speculative call, request/token buckets must leave the configured foreground allowance available. Unknown/insufficient provider RPM/TPM configuration pauses speculation. A job slot alone is not a rate-limit reservation.
- GitHub core requests initially serialize per credential/egress owner. Before the next physical request choose demanded waiters; a delayed speculative waiter cannot occupy the permit during backoff. Retain observed core quota of at least two configured source-call budgets plus visibility-gate allowance for foreground. This is a conservative local gate, not a guarantee against quota consumed by other applications sharing a credential/IP.
- The provider request bound and token preflight use existing exact serialized request/output bounds. Job funding does not grant network-rate capacity, and a rate permit does not grant spending authority.

Provider RPM/TPM limits are deployment configuration and observed responses, not invented constants. A 429 pauses future speculative dispatch; it does not automatically replay an ambiguous paid call.

## Persistent accepted cache and privacy

Use a private persistent directory owned by the same generation process, alongside existing controls. Write through a dedicated trusted CachePublication API after all existing mechanical and semantic checks pass AND acquisition is complete/cacheable. Accepted alone is insufficient: current jobs.go separately requires Investigation.Outcome().Cacheable() and Snapshot().Complete. A partial acquisition can produce a valid one-shot ready result but must never enter reusable memory/disk cache.

CachePublication carries Accepted, pinned identity/revision, complete/cacheable acquisition proof and the current registry completion fence. Only the dispatcher constructs it after verifying those properties. A plain RepoPreview or a caller-supplied accepted flag cannot publish a cache record. Nonreusable ready results remain usable by their existing current receipt but are not a durable hit after restart. Complete means the bounded inspection completed under configured acquisition limits; it does not claim every file in the repository was read.

Initial bounds: 24-hour accepted TTL, at most 64 artifacts / 64 MiB total, at most 2 MiB per envelope. These are proposed operational caps; overlarge artifacts remain usable for their current job if supported but are not reusable and do not bypass limits.

An envelope contains schema/revision, full immutable identity, producer receipt, accepted preview, completion/expiry and integrity checksum. Use strict bounded decoding, matching keys, owner/private path checks, atomic replace and fsync. Store the envelope as an atomic committed cache record; do not infer publication from a partial temporary file or a boolean supplied by an external caller.

On restart:
- Queued/running receipts become interrupted and are never automatically replayed.
- Durable accepted envelopes remain reusable only after key/schema/revision/expiry validation and a fresh public-identity gate.
- A completed envelope can satisfy a new lookup without model work even if its producer receipt expired. Interrupted receipts do not automatically become execution permission.
- Missing/corrupt/expired artifacts are misses. Persisted cooldown/admission records prevent a restart causing repeated speculative regeneration; foreground retry is explicit.
- Artifact failure does not erase or recreate the independent ledger.

No source prose is embedded in feed responses. Every response containing accepted prose passes the existing fresh public visibility/numeric-identity check and its late-check fencing. A stored artifact is not access authority. Failed visibility checks withhold the response; concrete private/deleted/recreated identity or forbidden credentials invalidate reuse. A job ID is not a bearer credential.

Extend fencing beyond the old per-receipt visibilityEpoch: all lookups/receipts for an artifact share a repository-level gate generation (and a credential-policy generation where appropriate). Capture these generations and the artifact/index version before a fresh check; before sending a body, recheck them under the shared store lock. A later failed check fences earlier in-flight successful checks across receipts and commits. Definitive invalidation persists a tombstone/removes index entries before acknowledging that state; restart cannot simply reload the old envelope. Failure to persist/control-journal uncertainty disables cache reuse/speculation until reconciled. Fresh network checking still applies even to an intact envelope. Test slow-public A → private/forbidden B → A completion, receipt expiry/rehydration, and restart after invalidation. Do not emulate a durable gate with only a new jobRecord pointer.

## Ready-first detail delivery and API compatibility

Keep feed ordering, pagination and the existing POST/GET walkthrough routes. No new public batch-generation endpoint is needed.

Add an optional identity field to a job envelope:
~~~
identity: { github_id, slug, commit_sha, generation_revision }
~~~
Also add optional machine-readable acquisition_outcome/reason_code to source previews, and reason_code to walkthrough envelopes for explicit public-identity withholding. Map the actual internal OutcomeIdentityWithheld to this signal even when the existing response remains HTTP 200 metadata-only. Generic timeouts/reading failures are not silently interpreted as definitive privacy outcomes, and prose is never parsed to infer a veto.

Source acquisition can also fail before a preview exists. Add optional reason_code to its existing error envelope without changing HTTP status or the error string. Initial repository-not-found/public-ineligible responses (currently 404) use one non-enumerating repository_unavailable code, without revealing private versus deleted. A concrete credential-policy rejection (currently 503) uses identity_withheld. Carry these typed reasons through APIClient's decoded error and DetailStore's event coordinator, including when accepted content arrived first. Ordinary overload, upstream rate limits, transport failures and timeouts remain non-veto failures. Do not classify every 404/503 as a veto without the typed reason; older servers retain legacy behavior.

These fields are server-generated, never accepted from client options. The identity field is generated from the pinned job. Existing clients ignore it. It lets a modern client validate a ready response for its submitted receipt before a separate source request has completed.

For one DetailStore load:
- Start/join the walkthrough promptly and load source fallback as a second structured child task, with a short configurable grace to avoid redundant source work on very fast ready hits.
- First usable source may render while a job is pending. An accepted walkthrough supersedes source; a late source completion never replaces it.
- A ready result validates against its receipt identity and preview numeric ID/canonical slug. Legacy servers without identity retain the existing source-first canonical validation path, including renames.
- If ready is validated before the source grace period elapses, do not issue the fallback request at all. Once a source request has been issued, let that bounded child finish while the active detail run remains alive, so an explicit privacy result cannot be lost by cancelling it after ready. Ordinary late source results are ignored. Leaving/replacing the detail cancels the whole run and cannot refund or cancel the shared backend generation job.
- Every update is fenced by detail-run generation and account ownership. Hiding the view retires queued retry actions; returning to a retained view never submits another job.
- If source fails but walkthrough succeeds, show the accepted story. If walkthrough fails but source succeeds, keep source. If both fail, show a retry action. An explicit typed visibility/identity-withheld event is an absorbing privacy veto for that active detail run: clear any source/accepted prose already displayed, stop loading/poll presentation, show the public-availability message and reject all later content events. Only an explicit new detail/retry run with fresh checks may clear that state. Linearize events on the state coordinator; cancellation of a source child is never used as a reason to discard a veto already processed. Retired run/account events still cannot mutate a newer owner/run. Test veto-before-ready, veto-after-ready and cancellation races.

All accepted pages and narrative lengths remain evidence-derived. No fixed screen count, fake progress percentage or forced runtime claim. This engine continues to produce static source walkthroughs; Azure execution and historical Figma recordings remain separate evidence modes.

## Measurements and acceptance

Before enabling live speculation, instrument:
feed time-to-first-byte, hint-offer time, queue wait, execution/stage duration, dispatch origin, demand promotion, accepted cache hit/miss, time from detail open to first usable content/accepted story, accounted spend, unknown reservations, rate-limit pauses, expired/dropped work and unrequested speculative cost.

POST demand is a server request signal, not proof a person watched the video or chose to contribute. No user IDs or source/model prose are required in these metrics.

Release checks:
1. With a blocked fake runner, feed response and metadata content match disabled mode; local added offer-path p95 stays below 5 ms. This is server overhead, not a network SLA.
2. Hundreds of concurrent duplicate hints/opens for the same pinned key start one engine; full queues remain bounded and demand admission/promotion still works.
3. Running speculation never occupies both worker slots. A new foreground job can dispatch without waiting for an unrelated speculative job to finish when the reserved lane/provider capacity is healthy.
4. Queue delay does not consume execution deadline; moving a job cannot renew source, token or dollar quotas.
5. Deterministic race tests keep total/class exposure within limits, retain unknown charges and prove no double debit when moving hold → call → settlement.
6. Restart reuses a complete/cacheable accepted artifact without a provider call; accepted partial acquisition never becomes reusable. Revision/commit changes, cross-receipt visibility races, persisted revocation, corruption and interrupted jobs fail closed.
7. Native completion-order tests cover ready-before-source, source-before-ready, either branch failing, delayed stale completions, rename/legacy identity, absorbing typed privacy vetoes from source HTTP 200, typed source 404, credential-policy 503 and walkthrough responses, cancellation and guest/account transitions. Untyped/transient failures never become inferred privacy events.
8. A warm accepted fixture becomes primary UI before a deliberately blocked source request finishes. A cold fixture still provides source fallback.
9. Canary live runs, only after operator enablement, report actual cold/warm distributions, accounted cost per requested walkthrough and speculative waste. Do not publish an arbitrary hit-rate or instant-load promise from synthetic tests.
10. Startup with an unavailable/locked/corrupt ledger, interrupted migration, uncertain artifact controls or invalid generation-only configuration still serves Discover/source metadata when their independent base configuration and public credential policy are healthy; walkthrough routes return 503 without issuing model calls or resetting any durable controls.

Existing historical controls are baseline evidence only. Paid live benchmarks are not part of writing this design.

## Implementation sequence (after owner reviews this proposal)

Each increment must stay deployable with speculation disabled.

1. Add measurement/contract fixtures and deterministic scheduling/funding scenarios; isolate optional generation configuration/initialization from healthy metadata startup. Test degraded startup before changing storage schemas.
2. Implement the bounded CachePublication artifact store and revision identity; test acquisition eligibility, restart/privacy/corruption before integrating reads.
3. Add explicit ledger migration and job-funding primitives; exhaustively test accounting and recovery in isolation.
4. Split target probing from queued Investigation lifetime; preserve all acquisition budgets and restart fencing.
5. Replace FIFO with bounded priority dispatch/leases; test demand borrowing, warm promotion, expiry and shutdown.
6. Integrate job funding and artifact lookup/publication with the existing engine, keeping speculation off.
7. Add shared upstream governors, then the optional feed snapshot sink and warmup coordinator behind operator flags.
8. Add job identity and typed source success/error outcomes, then ready-first native source/story coordination, preserving legacy compatibility and scoped preferences.
9. Run full Go tests/race/vet/build, Postman fixture contracts and native regression checks, then a tiny measured canary within existing approved limits. Raise concurrency/candidate count only on evidence.

Review-driven changes included here: (1) hints are not queued warm jobs; (2) priority and bounded waiting apply at upstream governors as well as worker dispatch; (3) cache publication preserves completed/cacheable acquisition, not just semantic acceptance; (4) cross-receipt persistent visibility fencing is explicit; (5) cooldown/start markers commit with funding before execution; (6) source success and error outcomes carry typed privacy signals with an absorbing active-run veto; and (7) generation-only startup failures preserve independent metadata availability. These are requirements for implementation, not claims of bugs newly fixed in runtime code.

Tasks are to be tracked in the existing Linear project after design review. Split an increment exceeding five implementation files into smaller reviewable tasks. Final file-level work depends on the accepted design; these are not claims of implementation completion.

Build/test commands:
~~~
go test ./internal/walkthrough ./internal/generation ./internal/feed ./internal/githubclient
go test -race ./internal/walkthrough ./internal/generation ./internal/feed ./internal/githubclient
go test ./...
go vet ./...
go build ./cmd/server
~~~
iOS: swift test in gittok-ios-ui, then the checked-in ../scripts/test-ios.sh using an isolated Postman fixture server; signed simulator builds are required for actual Clerk checks.

## Rollout, rollback and alternatives

- Deploy scaffolding with warmup disabled and one execution worker, then verify cache and accounting migration.
- Enable two-worker demand scheduling with speculation still off; validate quotas, funding and thread safety.
- Enable a two-candidate/one-speculative-lane canary only after explicit reviewed operator settings. Preserve the $50 ledger window and all unsettled history.
- Stop speculation immediately for rate-limit pauses, artifact/ledger integrity failures, sustained foreground degradation, or poor accounted benefit.
- Roll back by disabling warmup/using one worker in the new binary. Do not run an old binary against a newer ledger schema or restore an older ledger snapshot to reclaim budget.
- A generation outage, full queue or cache error never takes Discover down.

Generation failure isolation must also hold during startup. Today cmd/server/main.go exits when walkthroughService initialization fails, including ledger-open failure. Separate mandatory base configuration/public GitHub credential validation from optional walkthrough configuration and initialization. Missing, locked, corrupt or uncertain generation controls, failed migration, or invalid generation-only settings disable paid authority and unsafe artifact reuse; expose walkthrough routes as unavailable with 503 while the independently healthy feed/source services continue. Do not reset the ledger or fall back to an unaccounted generation path. Shared base configuration/security failures still fail appropriately; this is not a bypass for invalid public GitHub credentials. Include degraded initialization and recovery in startup fixtures and operations documentation.

Rejected for this slice: launching one task per card on iOS; generating every loaded page; keeping FIFO and merely adding goroutines; cancelling paid warm calls on every scroll; skipping review or lowering evidence quality; a separate autonomous budget per worker; a queue/service dependency before single-owner limits are measured; automatic paid replay after restart.

References:
- Go pipeline cancellation and bounded concurrency: https://go.dev/blog/pipelines
- GitHub request pacing, conditional requests and rate-limit response handling: https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api
- Existing ledger/recovery contract: docs/walkthrough-operations.md
- Existing generation gates: docs/walkthrough-execution.md and docs/walkthrough-validation.md
