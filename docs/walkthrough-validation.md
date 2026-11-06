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
| Complete Go tests | Passed before final live-run allocation repair; `/tmp/gittok-walkthrough-all-tests.log` |
| Complete Go race tests | Passed before final live-run allocation repair; `/tmp/gittok-walkthrough-all-race.log` |
| Go vet / build | Passed; `/tmp/gittok-walkthrough-vet.log` |
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
final-output cases. Live reruns and recorded source-quality assessment are pending.

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
No rejected story was published. A maximum of one constrained correction and
fresh independent review is being implemented inside the original job limits;
both paid-call receipts must be reserved before the correction executes. Revision
cannot introduce new source selectors or silently expand the inspected registry.
Live acceptance remains pending until actual accepted stories are assessed.

The first complete eight-call correction attempt remained unpublished after its
fresh review ($0.056069, all settled). A subsequent single private diagnostic
identified an implementation overconstraint: a legitimate citation narrowing
from lines 105–112 to 105–108 was rejected as a changed selector. Both spans
matched the pinned source; narrowing retained the stated empty-argument premise.
Revision now permits one contained subrange per original citation with injective,
ordered matching. Expansion, splitting, duplication and new files remain denied.
The fresh reviewer receives the same authoritative source windows, with only the
candidate replaced, preserving independence and the pre-reserved wire bound.
Independent source comparison found all 17 initial and 16 revised diagnostic
citations exact at commit `8b0600fa445535b9c53194c0d4f7fe3cf700ef3e`.

The local ledger depends on trusted persistent storage. Marker binding detects
foreign snapshots, not deliberate rollback of all control files or an older
snapshot of the same ledger. Restore/reconcile against provider usage before
resuming from a backup. No cloud resources were provisioned.
