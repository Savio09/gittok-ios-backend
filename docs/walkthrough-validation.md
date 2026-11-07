# SAV-52 execution and review record — November 6, 2026

## Scope and decision

Original G15/G16 are merged. Their bounded inspection output did not explain
software workflows. This implementation adds an optional semantic writer and an
independent review boundary, preserving the existing template claim validator.
One generic pinned-source investigation serves arbitrary public repository
identities; taxonomy is optional framing, not an eligibility gate.

Implementation follows `walkthrough-execution.md` and the reviewed capability
map under `gitstack/docs/2026-11-06-walkthrough-realignment`. Provider wire formats
follow OpenAI's official Responses function-calling and structured-output docs.
No target repository is executed. Azure observations remain separate SAV-53.

## Verification evidence

| Check | Result / artifact |
|---|---|
| Complete Go tests | Passed `go test ./...` after dedicated submission, context and model-configuration changes |
| Complete Go race tests | Passed `go test -race ./...` after those changes |
| Go vet / build | Passed `go vet ./...`; live executable built successfully |
| Initial context cost repair | RED/GREEN, full Go tests, focused race checks and vet passed; final independent review GREEN |
| Swift core | 45 passed; `/tmp/gittok-walkthrough-swift-all.log` |
| Native iPhone simulator | 45 core + 14 UI passed; `/tmp/gittok-walkthrough-native-final/Tests.xcresult` |
| Isolated Postman simulation | Baseline 8 requests / 10 assertions passed; explicit source → queued → writing → ready story fixture exercised by native tests |
| Canonical repository alias | RED then GREEN; `/tmp/gittok-ios-canonical-{red,green}.log` |
| Foreground reading position | RED then GREEN; `/tmp/gittok-walkthrough-lifecycle-{red,green}/` |

The fixtures establish transport, publication and presentation behavior. They
are explicitly simulated; no fixture is described as a genuine runtime capture
or a successful live model generation.

## Defects exposed and repaired during independent review

| Defect | Repair and regression |
|---|---|
| Source citations could be exact but semantically wrong | Separate whole-story/per-statement reviewer, mandatory actual cited context; live readability/source evaluation still required |
| Reviewer reasons offered a new factual prose escape | Publish fixed explanation text; injected factual reasons never render or change content IDs |
| Invalid UTF-8 caused quadratic trimming past deadline | Linear bounded trailing-rune handling; internal invalid bytes/NUL withheld; cancellation checked before retaining source |
| Seed reserved files but consumed all source bytes | Both attempted files and bytes reserve follow-up capacity across repeated seeds |
| Credential failure could be swallowed | Fatal identity-withheld propagation through seed/tools; outcome remains terminal |
| Acquisition and acceptance caps drifted | Shared safe source/component path policy; listing ceiling aligned to analysis limits |
| Cached POST bypassed final visibility check | Same final gate as ready polling; failed gates fence concurrent stale checks |
| Disconnected poller destroyed accepted cache | Caller cancellation cannot mutate shared completed state; transient provider checks withhold response while allowing a fresh gate |
| GET ignored completion expiry | Expiry enforced on reads and writes, durably removing receipts |
| Saturated registry rejected duplicates | Bounded canonical probes coalesce existing active work without allocating another worker |
| Restart / journal loss could replay paid work | Interrupted/unavailable receipts only; separate persistent spending ledger; no automatic replay |
| Corrupt/oversized or substituted control files | Strict bounded journal, locks/private files; ledger marker bound to snapshot identity; empty/foreign markers fail closed |
| Directory policy claimed 0700 but allowed 0755 | Enforced owned 0700 at initialization, open and runtime accounting |
| HTTP transport could automatically resend a paid POST | Dedicated fresh HTTP/1 transport, nonreplayable bodies, no redirects; real connection-failure regression |
| Reviewer saw a duplicate quote at a wrong line | Exact cited span is supplied first; surrounding context cannot substitute for cited lines |
| Source search clipped before its match | Matching line prioritized and identified; hidden matches report explicit limit error |
| Story delivery changed card identity and lost metadata | Preserve owner/name card ID and reuse existing provider-value checks for sourced metrics/license |
| iOS rejected canonical GitHub redirects | Validate accepted story against resolved source identity |
| Foregrounding erased accepted story and current page | Load keyed to repository changes; no full reload on foreground; native RED/GREEN confirms no repeated GET/POST |
| Larger helper displaced an affordable complete entry window | Reuse canonical crawler entry/product ranking over acquired paths; regressions cover main/helper, entry/build config and library/example |
| Dropped blank lines were marked as complete | Derive truncation from actual retained lines/content; direct RED/GREEN regression |

## Live provider evidence and remaining work

The owner configured a key locally. Presence was checked without displaying it;
the backend successfully made authorized model calls. A separate local evaluation
server runs on port 18082 with a $2 operator window and $0.50 per-job hard cap.
Accounting is in the ignored backend `.local/walkthrough/` directory. Its window
is retained across retries; it is never reset to obtain another test run.

First `charmbracelet/glow` attempt: seven settled writer calls, about 13 seconds
of generation, then `generation_limit`. No story was published. The ledger
recorded $0.03624 using configured published token rates. This exposed a real
behavior absent from scripted happy-path tests: the model kept exploring until
it exhausted writer turns. Final submission and independent review need their
own reserved capacity. The repair must also account for bounded tool-result
growth and a usable final output allowance; merely forcing the last call is
insufficient. Regressions reproduce both small-remaining-input and one-token
final-output cases. Later live outcomes are recorded below.

Further live evidence corrected the initial citation-copy hypothesis. A private,
explicit diagnostic captured only public model output and acquired source (never
request headers or credentials): all source handles were real, but the model
selected 41–54-line spans despite the 30-line prose instruction. The provider
contract now uses typed `file_id`, `start_line`, `line_count` selectors with
schema/runtime count bounds of 1–10 and overflow-safe conversion. The backend
extracts exact source bytes; legacy end-line/quote fields are rejected at this
model boundary. Internal literal-quote validation remains intact.

With that repair, both Glow and Rack reached independent semantic review. The
captured Glow review accepted seven of nine statements but rejected an overclaim
about rendering from program-construction code and an inaccurate configuration
merge explanation. Exact citations did not establish those statements' meaning.
No rejected story was published. A maximum of one correction and fresh independent
review was implemented inside the original job limits, with both paid-call
receipts reserved before correction executes. Its initial policy froze candidate
IDs and citation selectors; subsequent review simplified that policy below.

The first complete eight-call correction attempt remained unpublished after its
fresh review ($0.056069, all settled). A subsequent single private diagnostic
identified an implementation overconstraint: a legitimate citation narrowing
from lines 105–112 to 105–108 was rejected as a changed selector. Both spans
matched the pinned source; narrowing retained the stated empty-argument premise.
That intermediate repair permitted one contained subrange per original citation with injective,
ordered matching. Expansion, splitting, duplication and new files remain denied.
The fresh reviewer receives the same authoritative source windows, with only the
candidate replaced, preserving independence and the pre-reserved wire bound.
Independent source comparison found all 17 initial and 16 revised diagnostic
citations exact at commit `8b0600fa445535b9c53194c0d4f7fe3cf700ef3e`.

A further investigation found that the writer saw only the first 40 lines of
each seed file although acquisition retained complete files. For Glow that hid
`executeCLI` and its render/display path behind imports and setup. The initial
projection now prioritizes bounded product code: at most six files, 24 KiB/800
lines per code file, and 64 KiB of raw source in total. Documentation/manifests
remain at 40 lines. Read/search tool windows stay at 8 KiB. This introduces no
new HTTP reads or quota/deadline resets; serialized model input is still checked
before reservation. Independent review found no concrete issue; targeted tests,
race checks and vet passed. Full-context mini-model attempts still did not
produce an accepted story.

## Stronger-model control and current result

The correction policy now keeps repository identity and the acquired-source
registry immutable, while allowing a fresh candidate to choose different retained
source spans, statement IDs and scenes. This removes restrictions unrelated to
factual support. Ordinary grounding and fresh semantic review remain mandatory.
The second reviewer sees only the new candidate and its actual source context.
Both calls are funded in advance with a 128 KiB final serialized review ceiling.
Unknown provider outcomes retain reservations; only known unissued calls release
unused capacity. The conservative pair reservation may not fit a small job cap.

Another defect explained truncated small-repository failures: the writer could
choose `submit_story` during a 512/1024-token exploratory turn. Exploration now
exposes `finish_reading` instead. Submission happens on a dedicated turn with
the remaining initial output allocation. RED/GREEN tests cover both revision
modes and reject unexpected early submissions without retry.

At the owner's suggestion, the live control switched to `gpt-6.1-sol`, `low`
reasoning, standard service tier, and zero optional revisions. Official model
documentation was checked before calling it. The original $2 window and $0.50
per-job caps were retained. Input accounting conservatively uses $2.50 per million
tokens (the cache-write ceiling), output $10; these local figures are not an
invoice. Two prior unknown provider outcomes remain reserved, not refunded.

`chalk/strip-ansi` reached `ready` in 29.8 seconds with two settled model calls,
recording $0.042028. Its four scenes contain ten claims and eighteen citations
across five files pinned to `622cca7b5dd4621cf85e5b517f396f94d8959d2e`.
Independent public-source retrieval verified every original line span and exact
combined excerpt with zero errors. Raw-host DNS/timeouts required using GitHub's
public contents API with raw media type; no credential was supplied to the audit.

The separate reviewer agent found a coherent purpose → CSI-containing input → type and
introducer checks → regex replacement → test-asserted `foo` explanation. The
fast path/shared-regex discussion and next investigation steps are supported.
Test expectations are attributed rather than presented as observed execution;
the uninspected matching dependency is explicitly limited. Every narrative claim
is inferred and every scene is `not_executed`. This is the first accepted live
control, not evidence that every repository produces a useful story.

A subsequent GPT-6.1 Sol Glow comparison failed initial writer preflight with
`generation_limit` after about four seconds, before any provider HTTP/reservation.
It cost $0 and left the ledger at $0.564875 charged or reserved. Rich initial
source can exceed the writer's conservative $0.25 share of the $0.50 job cap.
At that point no accepted Glow story was claimed; the failure prompted the
projection repair below. Scripted generalization tests could not replace an
actual larger-repository assessment.

The resulting narrow fix preflights the exact final-writer wire request before
any paid action. It preserves the original projection when affordable; otherwise
it prioritizes the crawler-ranked primary entry/product window, then adds useful
secondary context and honestly marks clipped prefixes. Its synthetic oversized
case retains all 479 core lines, including mechanism code at line 309, plus
README context. UTF-8 clipping, unchanged acquired registry and zero-charge
failure under an absurd cap are guarded. Existing exploration-growth reserves
remain conservative; fitting context does not promise that every model/budget
combination can afford exploratory turns.
Final independent review found the helper-selection and blank-line issues above;
both were repaired and rechecked GREEN. Focused race checks after those fixes
passed in 1.784 seconds. The ranking is a source-selection heuristic, not proof
that a selected file represents the whole application; semantic support remains
the separate publication requirement.

The repaired GPT-6.1 Sol Glow run then reached `ready` in 32.35 seconds with four
settled calls and $0.130476 accounted cost. Its nine claims across four scenes
cite twenty-two exact spans in two files at
`8b0600fa445535b9c53194c0d4f7fe3cf700ef3e`. Separate public-source retrieval
verified every excerpt and pinned URL. The reviewer agent found a coherent piped
Markdown → stdin dispatch → read/preprocess → renderer → text sizing → stdout
path, meaningful destination-aware presentation/source-abstraction insights, and
specific implementation handoffs. Helper internals and visual output remain
explicitly unobserved. No material factual defect was found. The audit records
one minor local wording improvement for the interactive no-argument branch.

The two accepted controls cover a library and a CLI, with no repository-name
behavior in production. They establish useful live source stories, not universal
repository success or actual target execution. Total ledger usage is $0.695351,
including the two retained unknown reservations. Public results and independent
audits are committed under `docs/evaluations/2026-11-06-sol/`.
The semantic assessments were performed by AI reviewer agents using the product
rubric. They are not human participant evaluation or owner acceptance; those
remain separate validation work for the capstone.

Process finding: too much defensive implementation preceded the first useful
live result. Future model-backed slices should establish a small genuine source
→ story → reviewed artifact path early, then expand measured coverage and bounds.
Stronger-model selection and source/submission fixes changed together, so this
result is not controlled proof that model strength alone caused success.

The local ledger depends on trusted persistent storage. Marker binding detects
foreign snapshots, not deliberate rollback of all control files or an older
snapshot of the same ledger. Restore/reconcile against provider usage before
resuming from a backup. No cloud resources were provisioned.
