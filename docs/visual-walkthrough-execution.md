# Visual walkthrough execution — November 6, 2026

The owner rejected paragraph-only stories as insufficient for contribution decisions.
REA inspection of GitDiagram showed a useful pattern: practical narrative → typed
visual beats → deterministic layout → narration progress. The owner explicitly
kept Folio's identity and approved this implementation. The complete investigation
is in the adjacent gitstack workspace's `docs/2026-11-06-gitdiagram-video-study`.

## What changed

Accepted claims now own typed concept, terminal, flow, comparison or source visuals.
Sections reference those claims in ordered versioned storyboards. Narration remains
`Claim.supports`; no independent scene text can drift from accepted evidence.
New generation requires every referenced visual plus a source-supported handoff.
The writer targets practical use/results before architecture, approximately 120–200
spoken words and 8–14 compact beats, with variable scenes inside existing hard caps.

Visual wording, comparison implications and flow direction are part of independent
semantic review. Code panels are extracted from original cited lines. All published
claims remain inferred and all scenes not_executed. This does not run a target,
produce captured screenshots, provision Azure, or export an MP4.

The native Folio player uses AVSpeechSynthesizer word callbacks for captions and
literal cue reveals. Completion advances beats; there is no fixed five-second text
timer. Pause/resume, replay, previous/next, current evidence, background/navigation
fencing, static fallback and persistent Codable visual data are implemented.
Handoff displays actual investigation prose/source instead of masquerading as setup.
Saved details are reopened online under the existing fresh-visibility policy;
persistence does not introduce offline replay or a bypass of public visibility.

## Findings that changed the implementation

Fresh agents reviewed the Go/Core boundary and native behavior independently.
All six actionable findings were repaired and rechecked:

1. Five-line citation neighborhoods hide distant enclosing guards. Review now
   receives complete retained cited files once, with original numbering and honest
   truncation. Requests that cannot fit existing limits fail before a paid call.
2. Repeated content across scenes could produce a film rejected by iOS. Visual
   claim identities are globally unique, including distinct draft IDs with equal
   rendered content. Duplicate generated identities fail before review.
3. Native empty/whitespace visuals and control-character cues could activate blank
   cards. Core mirrors Go required bounded text, preserving valid emoji joiners.
4. Section media could disappear when resolving an illustrated story. Media-bearing
   pages retain the ordinary preview rather than promoting a partial film.
5. Source visuals backed by substring-only quotes could disagree with the excerpt.
   Such visuals now require an exact excerpt prefix. Alternating repeated lines
   exposed an incomplete Contains repair; HasPrefix and a regression close it.
6. Comparison panels always selected RESULT. They now honor zero/input/result focus
   consistently and expose selected state to accessibility.

The first broad native run caught an ambiguous test selector inherited by multiple
visual children. Its failure log was retained; the corrected unique selector keeps
the cue-reached assertion. A fresh complete test run passed.

A further live finding: a fixed 128 KiB correction-review reservation exceeded the
$0.50 job cap at configured Sol rates, so even a small corrected story could never
reach its final review. The repair now chooses an affordable review body after reserving the rewrite’s conservative maximum. Exact final body/cost guards remain enforced, and unfunded pairs fail before a rewrite. Realistic-price, insufficient-budget, overflow and oversized-candidate regressions passed fresh adversarial review; no cap, context requirement or ledger state was relaxed.

## Evidence and its limits

- Complete Go tests, race checks, vet and server build passed for the visual feature.
  Focused RED/GREEN covers exact source code, immutable copies/identities, cues,
  required fields, visual facts, handoff, full review context, HTTP publication and
  seven scripted repository shapes. Scripted fixtures establish contract behavior.
- Final native run: 60 core tests passed; 18 UI tests passed, with the additional
  live-only test skipped until explicit configuration. Separate explicit live UI
  test passed lookup → actual Go response → complete film → real word callbacks →
  online save/reopen → exact current evidence. No extra model calls were incurred
  by that prepared cached-story UI test.
- Existing Postman contract: 8 requests / 10 assertions passed. The new six-beat fixture
  is explicitly synthetic and its deterministic narration driver is visibly labeled.
  Actual native callbacks were checked separately, including no-voice fallback.
- The real nested-guard pair passed through the production reviewer: an export-only
  statement was rejected because it omitted writable; an otherwise identical
  statement naming both nested predicates was accepted. Sources, visuals, citations
  and cues are identical. These are two controlled observations, not proof that a
  model will never miss another guard.
- `chalk/strip-ansi`: ready in29.1s, 4 scenes/10 beats/156 spoken words. All 12 citations
  across 3 pinned files and the original source panel matched independently fetched
  source. Separate reviewer-agent assessment found no concrete unsupported claim
  and supported practical use/result, implementation and grounded next investigation.
- First-attempt Glow and strip-ansi-cli stories were withheld by semantic review
  with correction disabled. Their receipts are preserved; they are not successes.
  Re-enabling the ordinary single correction revealed the reservation bug above.
- `chalk/strip-ansi-cli` (requested through historical alias `sindresorhus/strip-ansi-cli`):
  ready in 49.6s after one correction and fresh review; 3 scenes/9 beats/146 words.
  All 12 citations across 4 pinned files and the original source panel matched the
  independent audit. Four settled calls accounted $0.078776. Native live verification
  and separate semantic assessment are recorded with its artifact.
- Glow's final bounded correction was attempted, then stopped at generation_limit
  before a final paid review. Its unused final reservation was settled at zero;
  $0.126050 was accounted for that job. This is a retained bounded failure, not
  a second CLI success or proof that every larger repository fits the current caps.

The two accepted controls cover library and CLI presentation but belong to the
same ANSI-stripping family. This is limited live diversity; scripted shape tests
and the paired guard check do not establish universal repository performance.

Raw accepted story, separately fetched sources, exactness auditor, semantic review,
controlled pair and rejected receipts are in `docs/evaluations/2026-11-06-visual/`.
Native logs, screenshots and full implementation review/rechecks are in the adjacent
workspace's `docs/2026-11-06-visual-walkthrough/`. These are AI engineering assessments
and simulator checks, not human participant evaluation or universal repository proof.

## Operation

Open the normal iOS `GitTok.xcodeproj` and run its application scheme. The local
backend remains 18082, with public GitHub access and the owner's configured model.
The same pipeline accepts an arbitrary public owner/repository; bounded acquisition,
unsupported evidence or rejected prose yields the readable source fallback.

The original capstone-2026-11-06 spend ledger and $2 window/$0.50 job cap are retained.
Unknown outcomes remain reserved. No reset, new window or refunded ambiguous charge
was used. Accounting uses conservative rates rather than being a provider invoice.
Model credentials remain in local backend configuration, outside the app/artifacts.

Owner acceptance of pacing, narration and contribution usefulness remains distinct
from implementation checks. SAV-53 remains the separate target-runtime/Azure slice.

## Final code and accounting

Application commits: backend 139b309/9f0175e (visual contract/generation), 6bca8d4 (affordable correction); iOS 550be92/56a5c65 (Core/player). Test helper 3ff1fe9 records the opt-in real guard pair; iOS 050b240 documents native operation. Normal feature checkouts are synchronized, preserving the owner's unrelated docs. No main merge/push or cloud deployment is claimed.

Final operator accounting is $1.373401 charged/reserved within the retained $2 window, including the same 2 prior unknown outcomes. No new ambiguous outcome or budget reset was introduced. `accounting.json` distinguishes paid settlements and an unused correction reservation. Default tests never invoke paid evaluation.
