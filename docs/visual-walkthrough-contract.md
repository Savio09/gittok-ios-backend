# SAV-52 visual walkthrough contract — November 6, 2026

Owner approved execution after the GitDiagram investigation. Folio remains the
application design, typography, palette, navigation and collection experience.
The borrowed capability is practical storytelling with narration-synchronized
visual changes. This document is the shared backend/iOS implementation boundary.

## Capability map and build order

| Module | Responsibility | Depends on |
|---|---|---|
| story-contract | Accepted narration, visual facts and ordered beat references | Existing grounded statements and independent review |
| folio-player | Native scene drawing, speech-word cues, pause/replay/source access | story-contract |
| story-generation | Practical-first generic model output, semantic checks and HTTP publication | story-contract |
| experience-validation | Real response on phone, save/reopen, held-out repositories | folio-player, story-generation |

Order: freeze this boundary → contract and native player in parallel → actual
HTTP integration → bounded real generation → independent review and full checks.
The owner's execution instruction authorizes this sequence; no additional
approval pause is needed for routine implementation choices.

## Product requirements

- Explain a concrete user problem, project use/result, a meaningful implementation
  path and a supported next investigation. First-half narration prioritizes use
  over filenames/configuration. Do not invent open issues or maintainer requests.
- Model scenes and statement counts remain variable. Target 120–200 spoken words,
  roughly 8–14 compact beats; these are editorial goals inside existing hard caps.
- Each new generated statement has one visual. Its narration is Statement.Text;
  its visual wording is reviewed with Text, Limit and all source premises.
- Visuals show concrete commands/values/changes or a short connected path. They
  do not merely repeat whole narration paragraphs or display dependency lists.
- Source inspection remains inferred/not_executed. An illustrated result is not
  a captured execution. The existing acquisition, privacy and spending controls
  remain the owners of their respective limits.
- A newcomer can explain the product, describe one use/result, identify a useful
  contribution investigation and save/reopen it with its evidence accessible.

## Internal draft additions

Statement gains optional `visual` for legacy compatibility. New production model
generation requires a valid visual for every referenced statement and an explicit
handoff scene. Existing non-visual accepted/source previews remain readable.

Visual fields (all required in new writer JSON):

```json
{"kind":"flow","title":"Render the document","lines":["Read Markdown","Render text","Write output"],"focus":2,"cue":"render"}
```

- `kind`: concept | terminal | flow | comparison | source.
- `title`: at most 56 Unicode characters. For source visuals the published title
  is derived from the first cited file, not supplied factual prose.
- `lines`: at most four strings, each at most 80 Unicode characters; no control
  characters. concept requires 1–3 lines, terminal 1–4, flow 2–4, comparison
  exactly two (input/result). The fields are factual content subject to review.
- For `source`, the model supplies empty title and lines. Publication extracts
  up to six original lines from the first grounded citation and uses its actual
  path. It must not replace code with model-invented excerpts.
  These published source fields are exempt from generic title/line character
  limits: retain original tabs/CR and the actual path under existing source and
  2400-byte citation bounds. Native presentation may wrap/scroll, not rewrite.
- `focus`: zero for no selected line, otherwise one-based within rendered lines.
- `cue`: a nonempty literal substring of the narrated Text, at most 40 Unicode
  characters. It identifies the first occurrence; clients compute its UTF-16
  offset for native speech callbacks. No model-provided timer or executable code.
- New text counts against existing prose limits. Flow ordering and input/result
  meaning, not only the existence of filenames, must pass semantic review.
- Reviewer instructions explicitly test nested branch predicates. A material
  premise missing from narrated wording is not repaired by an uncertainty label.

## Published additive JSON

Claim gains optional `visual`, the accepted/published visual object. Supports
remains the accepted narration text. Claim identity incorporates the visual.
Section gains optional `storyboard` with version 1 and ordered beat references:

```json
{
  "kind":"how",
  "title":"How it works",
  "claims":[{
    "id":"statement-v1-example",
    "supports":"It reads the Markdown and sends the text through a renderer.",
    "visual":{"kind":"flow","title":"Render the document","lines":["Read Markdown","Render text","Write output"],"focus":2,"cue":"renderer"}
  }],
  "storyboard":{"version":1,"beats":[{"id":"beat-v1-example","claim_id":"statement-v1-example"}]}
}
```

This is a shape excerpt: existing claim source/excerpt/limits/status fields remain
required according to their current contract. Narration and visuals are owned
by the accepted Claim, not duplicated as independently editable scene prose.
Every published beat maps to one same-section accepted claim, in claim order.
No duplicate/dangling IDs. Accepted accessors deep-copy visuals and storyboard
slices. Section/content identities incorporate the published storyboard.

Handoff is published as additive section kind `handoff`, rather than `setup`.
It remains a preview scene and cannot masquerade as verified setup instructions.
The detail view should use that actual handoff text/source as a next investigation;
it must not display a generic claim label as setup evidence.

## Native interface boundary

GitTokCore additions:

- `StoryVisual` Codable/Sendable: kind, title, lines, focus, cue.
- `StoryboardBeat` Codable/Sendable: id, claimId.
- `Storyboard` Codable/Sendable: version, beats.
- Optional `Claim.visual`, `PreviewSection.storyboard`, `PreviewPage.storyboard`.
- `ResolvedStoryBeat`: id, claim, narration, visual, evidenceID, sceneID,
  sceneTitle, sceneKind. Narration comes from Claim.supports.
- `RepoPreview.storyboardBeats: [ResolvedStoryBeat]?`: returns a nonempty resolved
  whole film only when every source page has a supported valid storyboard, refs,
  visual kinds/bounds and cues. Partial/unknown/malformed storyboards fall back
  to the ordinary readable preview; never silently omit a scene.
- `ResolvedStoryBeat.cueUTF16Offset`: first literal cue position in narration;
  handles emoji/Unicode correctly. Provide a pure helper for reveal decisions
  from native speech `NSRange` progress. UI owns the speech lifecycle/cursor.

Use Swift naming above so concurrent authors share one interface. Add public
initializers only where actual UI/test consumers need them. Old decoded/saved
RepoPreview values remain compatible. No wire audio URL is required for this slice.

Native AVSpeechSynthesizer owns narration progress. Word callbacks update caption
highlighting and cue-triggered reveals; beat completion advances the cursor. No
fixed five-second progression. Start paused; support pause/resume, previous/next,
replay and evidence inspection. Opening sources/backgrounding/leaving pauses or
stops speech; stale utterance callbacks cannot advance a new scene/repository.
Reduced Motion keeps information visible while minimizing transitions. Use Folio
tokens, readable concrete visuals, 44-point controls and a static readable path.
If a native voice is unavailable, show a truthful caption/manual-navigation
fallback rather than pretending narration played. No custom generated JavaScript.

## Acceptance and verification

1. Go RED/GREEN: validation, visual-fact publication, deep-copy/content identity,
   exact source visuals, required refs/cues, handoff semantics, generated schema.
2. Swift RED/GREEN: producer JSON, backward/unknown/malformed fallback, Unicode
   cues, variable scene counts, full save/decode/reopen of the storyboard.
3. Native tests: concrete visual content, speech-state progression (explicit
   deterministic driver allowed for tests), pause/evidence/replay/lifecycle,
   save/reopen and Folio layout. Test narration is not evidence of real speech.
4. Appropriate full Go/Swift/native/HTTP gates and independent adversarial review.
5. Authorized bounded real runs through the same generic pipeline on different
   repository types, inspect visual claims/source fidelity and actual simulator
   output. No repository-name branches or fake observed-running labels.

Keep the original $2 operator window and $0.50 per-job cap; no ledger reset. Only
the coordinating agent may perform paid validation, with the configured key kept
out of outputs. Azure, MP4 export, remote collections, auth and new infrastructure
are separate work. Tests and AI review do not establish human participant success.
