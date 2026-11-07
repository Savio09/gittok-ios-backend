# SAV-52 implementation contract

Approved by the owner: November6. Builds the reviewed capability map in
gitstack/docs/2026-11-06-walkthrough-realignment. Go1.25.5, existing standard HTTP
and native Swift6; no new application dependency is needed for this slice.

## Ordered increments

1. Typed story/evidence/independent-review gate and source-only section renderer.
   All factual published words derive from accepted statements. Preserve original
   template Claim validation. Exact file/span/quote checks precede semantic review.
2. One bounded investigation session owning commit, cumulative attempted-file,
   retained-byte and physical HTTP budgets, deduplicated reads and an absolute
   deadline. Reserve half the files for targeted follow-up; no target execution.
3. Explicitly initialized durable spend ledger, cumulative generation budget and
   bounded provider/tool loop. Model calls include independent whole-story review.
   No configured key/model/pricing/ledger means no paid work, not fake content.
4. Bounded background jobs, idempotent repo/commit generation, fenced publication,
   local journal/interrupted recovery and current visibility checks. One worker
   process; no claim of distributed durable completion or automatic paid retries.
5. POST /v1/repos/{owner}/{name}/walkthrough returns202 (200 whenready), GET
   /v1/walkthroughs/{id} returns the same job envelope. Fields: id,state,stage?,
   poll_after_ms,preview?,message?. Active states queued/reading/writing/reviewing;
   terminal ready/failed/interrupted/cancelled/unavailable. All responses no-store.
6. iOS retains quick source preview while polling. Accepted story prose becomes
   primary content, with variable pages and exact contextual sources. Optional
   presentation='walkthrough' skips redundant metadata intro; Section.id and
   Claim.id are additive. Legacy/unconfigured APIs retain honest source fallback.

## Stronger-model follow-up

The first real GPT-6.1 Sol control passed both independent publication review and
a separate exact-source and reviewer-agent quality audit. A larger CLI then failed before
any model call because its initial projection exceeded generation preflight.
The resulting narrow repair fits already acquired initial source to the
exact final-writer request allowance, keeping the pinned bundle and quotas
unchanged. It preserves useful complete code context where it fits and drops or
accurately marks clipped secondary context. The oversized case passed RED/GREEN;
a subsequent bounded Glow run passed exact-source and reviewer-agent assessment.
Neither the $2 window nor the $0.50 job cap was increased.

Live acceptance evidence for the library and CLI is in
`docs/evaluations/2026-11-06-sol/`. This establishes the first source-only slice;
broader repository coverage and actual sandbox observations remain further work.

## Remaining review finding

The final committed build produced another exact-citation Glow story, but its
optional width explanation omitted a nested terminal-output condition. Keep
SAV-52 open for that semantic precision gap and owner acceptance. Next work:

1. Add a held-out semantic evaluation pair: a conditional implementation, an
   overbroad statement that omits a nested guard, and a correctly scoped variant.
   Synthetic tests should use generic conditions rather than repository names.
2. Require the semantic reviewer to enumerate necessary branch predicates and
   reject a missing premise even when a vague limit or surrounding prose hints
   at it. Establish actual model behavior on the negative and positive controls;
   a stubbed reviewer cannot prove this property.
3. Reassess live prose under the existing budget before closing the ticket.
   Preserve the recorded counterexample and source-only status. Runtime sandbox
   work remains separate; it does not repair static semantic overclaims.

## Validation

Per-increment RED/GREEN logs, fresh adversarial review, then full Go tests/race/
vet/build, Swift core/native tests and explicit HTTP simulations. Source stories
must explain purpose and action→mechanism→result, not an inventory of names or
dependencies. Fake providers establish contract behavior only. A configured,
authorized real model run plus held-out readable story review is required for
live acceptance. Azure observations remain SAV-53, a separate actual-run gate.

No main merge, push or cloud provisioning is required by these code increments.
Owner's prior research/documentation remains untouched. API secrets stay outside
prompts, version control, the iOS app and target execution environments.

## Visual follow-up — November6

The approved narrated-visual increment supersedes the next-work list above. The real nested-guard negative/positive pair passed, complete retained cited files now reach review, and Folio renders typed visuals from actual speech callbacks. The previous counterexample remains historical evidence; model reliability and owner acceptance remain qualified. See `visual-walkthrough-execution.md` and `evaluations/2026-11-06-visual/` for the current implementation, review repairs, failed controls and verification.
