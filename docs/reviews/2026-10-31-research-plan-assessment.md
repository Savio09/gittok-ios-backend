# Research assessment and implementation-plan revision — October 31, 2026

Status: research assessed and repair instructions revised; application code
remains unchanged.

## Sources and scope

Read both reports in full:
[Pipeline findings](/Users/fdeclan/Public/gittok-research/findings-pipeline.md)
and [Infrastructure/contract findings](/Users/fdeclan/Public/gittok-research/findings-infra-contract.md).
The included GitDiagram checkout is
`7876ab0ca8b5ee46e836235e23c4886d5cdb79ae`, the same source snapshot
used for the [earlier architecture study](2026-10-30-gitdiagram-architecture.md).
This is additional interpretation, not new upstream implementation.

Cross-checked the reports' high-impact claims against source: citation
eligibility, heuristic reference extraction, edge styling, source-read/excerpt
budgets, manifest exclusions, quota admission and rate limiting, artifact keys,
and CI/deploy workflows. No third-party scripts or tests were run, no live
infrastructure was inspected, and the research directory was not edited.

GitTok HEAD remains `3bf15746e053a391e95d7752c9c97684187bb774`.
The [repair plan](../../tasks/plan.md) still addresses the reproduced G15/G16
defects. This review does not authorize implementing the broader product
architecture or changing the public API.

## Decision

Keep the nine repair slices and the evidence-first direction. Strengthen the
acceptance conditions and sequencing, while explicitly separating future
pipeline/infrastructure ideas from current bug repair.

The useful contributions are clear evidence-consumption states, bounded
inspection, fair source budgets, deterministic rendering/contracts, and
preserving useful results outside feed reads. Most already align with the plan.
Several stronger claims about grounding and cost limits do not follow from
the code, so they must not become implementation requirements.

## Corrections before using the research as instructions

### 1. Deterministic reference extraction does not verify semantic truth

The pipeline report describes the reference resolver as the grounding engine
and says a citation is retained only when it genuinely connects endpoints.

The actual resolver uses regexes, path heuristics and name matching. The
citation filter accepts an inspected file within either endpoint component
without checking that its contents establish that particular relationship.
A root README is also exempt from the inspected-file eligibility check.
Backfilling searches references in either direction; it does not establish a
claimed call direction, verb or runtime behavior.

A deterministic false match is still false. This is especially important
because GitTok's original defects already upgraded string matches into facts.

Sources: [reference extraction](https://github.com/ahmedkhaleel2004/gitdiagram/blob/7876ab0ca8b5ee46e836235e23c4886d5cdb79ae/src/server/generate/source-references.ts#L128-L158),
[Go path heuristics](https://github.com/ahmedkhaleel2004/gitdiagram/blob/7876ab0ca8b5ee46e836235e23c4886d5cdb79ae/src/server/generate/source-references.ts#L290-L309),
[citation rules](https://github.com/ahmedkhaleel2004/gitdiagram/blob/7876ab0ca8b5ee46e836235e23c4886d5cdb79ae/src/server/generate/edge-evidence.ts#L35-L90).

**Plan change:** added a provenance-versus-support invariant; valid-but-irrelevant
source tests; quote/span validity tests; and an explicit prohibition on adopting
the heuristic resolver or automatic citation backfilling as proof of a conclusion.
The maintained parser tests must exercise the actual declaration/construct, not
just the existence of a cited file.

### 2. Read, shown, excerpted and accepted are different states

The research sometimes equates a file being read with its text being shown to
the model. GitDiagram reads some full files only to derive the source index.
The model receives selected excerpts and derived reference lists, not every
byte used to build those lists. Files can also be known only by path.

Sources: [source index statuses](https://github.com/ahmedkhaleel2004/gitdiagram/blob/7876ab0ca8b5ee46e836235e23c4886d5cdb79ae/src/server/generate/source-context.ts#L59-L111),
[model-visible excerpts and returned provenance](https://github.com/ahmedkhaleel2004/gitdiagram/blob/7876ab0ca8b5ee46e836235e23c4886d5cdb79ae/src/server/generate/source-context.ts#L450-L521).

**Plan change:** Slice 1 now distinguishes retrieved bytes from analyzer-consumed
bytes and preserves original source offsets/ranges. A future model-facing index
is not required to fix classification. An omitted region cannot justify an
absence claim.

### 3. Uncited edges are not guaranteed to appear uncertain everywhere

The completed Connections panel may call uncited relationships inferred, but
the evidence filter only changes `evidencePath`; the compiler independently
uses `edge.style` for solid/dashed arrows. Removing a source can therefore leave
a solid relationship.

Sources: [evidence stripping](https://github.com/ahmedkhaleel2004/gitdiagram/blob/7876ab0ca8b5ee46e836235e23c4886d5cdb79ae/src/server/generate/edge-evidence.ts#L76-L90),
[edge rendering](https://github.com/ahmedkhaleel2004/gitdiagram/blob/7876ab0ca8b5ee46e836235e23c4886d5cdb79ae/src/server/generate/graph.ts#L362-L370).

**Plan consequence:** retain the existing rule that invalid/removed supporting
evidence must downgrade or remove a dependent conclusion. A renderer or prompt
must not be the only place that enforces uncertainty. Do not inherit diagram
repair semantics in which dropping a citation is merely cosmetic.

### 4. Execution mode is not a global confidence hierarchy

The research proposes treating `observed_running` as a higher-confidence tier.
An observation can support one scenario under one configuration while leaving
other claims unsupported. GitDiagram's video path renders its own stage; it is
not evidence that the target repository ran.

Source: [own-stage renderer](https://github.com/ahmedkhaleel2004/gitdiagram/blob/7876ab0ca8b5ee46e836235e23c4886d5cdb79ae/src/server/explainer/render.ts#L183-L217).
GitTok's existing `internal/apimodel` already separates confidence, stance and
execution mode.

**Plan change:** explicitly preserve these distinctions. Do not replace the
public schema with the report's proposed `kind` fields or treat runtime
observation as an automatic upgrade to every claim. Target execution remains
outside this repair.

### 5. The reports conflate fail-open rate limits with quota admission

The infrastructure report says all quota failures allow requests because a
daily budget still bounds spend. That is not what the inspected enabled
diagram-quota path does:

- The IP rate limiter catches storage errors and returns `allowed: true`.
- Enabled complimentary admission awaits a Redis reservation; reservation
  errors propagate before the model stage rather than granting admission.
- Quota activation depends on configuration/caller path. Estimates and
  reservations are not inherently hard output or monetary limits.

Sources: [rate-limit failure policy](https://github.com/ahmedkhaleel2004/gitdiagram/blob/7876ab0ca8b5ee46e836235e23c4886d5cdb79ae/src/server/generate/rate-limit.ts#L168-L193),
[quota admission](https://github.com/ahmedkhaleel2004/gitdiagram/blob/7876ab0ca8b5ee46e836235e23c4886d5cdb79ae/src/server/generate/complimentary-gate.ts#L221-L259),
[admission before model work](https://github.com/ahmedkhaleel2004/gitdiagram/blob/7876ab0ca8b5ee46e836235e23c4886d5cdb79ae/src/app/api/generate/stream/route.ts#L491-L549),
[reservation estimate caveat](https://github.com/ahmedkhaleel2004/gitdiagram/blob/7876ab0ca8b5ee46e836235e23c4886d5cdb79ae/src/server/generate/complimentary-gate.ts#L145-L164).

**Plan consequence:** no quota system is added. Any later paid-generation design
must specify enabled/disabled paths, storage-outage behavior, concurrency,
reservation cleanup and provider-side limits explicitly. Do not copy a generic
fail-open policy based on an assumed independent cap.

### 6. A repository cache key and a database recommendation solve other problems

GitDiagram's artifact keys identify repository and visibility namespace, not
source commit and analyzer version. Reuse reduces repeated work, but freshness
still needs a policy. The suggestion that personalized feeds require Postgres
is a design recommendation, not a conclusion established by this study.

Sources: [artifact key identity](https://github.com/ahmedkhaleel2004/gitdiagram/blob/7876ab0ca8b5ee46e836235e23c4886d5cdb79ae/src/server/storage/cache-key.ts#L32-L69),
[artifact fields](https://github.com/ahmedkhaleel2004/gitdiagram/blob/7876ab0ca8b5ee46e836235e23c4886d5cdb79ae/src/server/storage/types.ts#L5-L18).

**Plan consequence:** keep revision information honest in evidence, but defer
storage selection/migration and cache architecture. No Postgres, Redis, SSE,
new error envelope, cost endpoint or authentication flow belongs in these
classification repairs.

## Changes made to the plan

| Area | Change | Acceptance effect |
|---|---|---|
| P1 sequencing | Execute Slice 8 after Slice 4, before Checkpoint A and P2 work | Verify the actual acquisition handoff preserves the repaired evidence model |
| Evidence model | Separate fetched and consumed content; preserve original ranges | Partial evidence cannot masquerade as a fully inspected file |
| Support checks | Existing-but-unrelated source, wrong quote, invalid span and reversed-call negative controls | Citation eligibility does not substitute for supporting observations |
| Acquisition lifecycle | Total deadline includes retry/backoff; cancellation regression required | Per-request timeouts do not silently exceed the caller's acquisition budget |
| Selection | Preserve component diversity and eligible contribution/test/CI evidence; record skipped candidates | No one-manifest global cap or runtime-only evidence policy |
| P2 acceptance | Require true-positive counterparts and separate inconclusive counts | Returning other for everything cannot satisfy the repair |
| Scope | Explicitly defer the wider research recommendations | No universal resolver or infrastructure rebuild before core repair |

The lifecycle clarification also follows direct inspection of GitTok:
`internal/githubclient/client.go` uses `time.Sleep` during retry backoff.
Passing a cancelled context to a later HTTP request does not interrupt that
sleep. Slice 8 calls for a focused regression and the smallest required fix;
it does not claim to implement the entire future G19 analysis-budget system.

## What remains useful later

- A bounded second read based on observed references, measured against useful
  evidence coverage. Preserve a read/skip ledger now; implement adaptive crawling
  later if representative failures justify it.
- Fair excerpt allocation with explicit omissions, if model synthesis is added.
  The reports do not establish that per-claim retrieval outperforms a shared
  bounded context for GitTok.
- A validated intermediate preview document rendered by SwiftUI; the existing
  API contract already supports that architectural direction.
- Durable accepted artifacts, last-good-result behavior, explicit schema/source
  versions, terminal generation states and controlled retries when the preview
  generation path is built.
- CI/deploy dependencies and cost-outage policy when deployment/paid generation
  becomes scope. The checked-in CI and Cloudflare deploy workflows run
  independently; static inspection does not establish live branch protections.

## Verification of this documentation change

Check local document links, slice dependencies/order, source citations against
the pinned checkout, and that only plan/review documentation changed. No
application tests are needed solely for these prose edits. Implementation test
commands remain part of the other agent's handoff, not results claimed here.

The other agent's original research is retained unchanged. Corrections are
recorded here so later process documentation can distinguish source evidence,
research interpretation, proposed architecture and implemented repairs.
