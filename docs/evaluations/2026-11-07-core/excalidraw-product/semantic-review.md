# Independent semantic and product review

**Verdict: request changes for two P2 issues.** The former missing-product problem is fixed: the opening now identifies Excalidraw as a hand-drawn-style collaborative whiteboard, and sharing an existing drawing is a useful representative task. Most source interpretations are supported. The remaining problems are an overstated import success condition and a narrative that spends a third of its words on import return/error branches after already giving the sharing result.

Reviewed `story.json`, `sources.json`, `audit.json`, and both supplied full pinned files at `701b901a0f1ba3d0b390d9cdf63d112e25abac4b`. No paid/model calls or target execution were performed. Only this review document was written. UI focus was assessed as one-based, with zero meaning none; there is no out-of-range focus issue.

## Required findings

### P2 — Decode and parse success alone do not guarantee the narrated import result

Affected fields: `job.preview.sections[1].body`, paragraph beginning “Successful decoding”; `job.preview.sections[1].claims[5].supports` (claim ID ending `130eeddab`). The visual is titled “Returned scene data.”

The narration says: “Successful decoding and JSON parsing return elements and editor state, each potentially null—not a guaranteed rendered drawing.” The source at `excalidraw-app/data/index.ts:222–229` parses arbitrary JSON and then immediately reads `data.elements` and `data.appState`. Successfully decoding and parsing JSON `null` satisfies the stated prerequisites, but those property reads throw and enter the legacy fallback at line 235. The TypeScript annotation does not validate the parsed value at runtime.

This is a narrower issue than claiming UI rendering, which the story correctly avoids. The current wording still treats decode/parse success as sufficient for returning the fields when the full source does not. Scope it to the normal decoded-scene success path, or say the success branch returns drawing elements and editor state, with null fallbacks. A newcomer story does not need an added malformed-data branch to make that wording accurate.

### P2 — Import branch details crowd out the requested action → mechanism → outcome story

Affected fields: `job.preview.sections[1].claims[4..6]`, their three storyboard beats, and corresponding last three paragraphs of the `how` body. The titles are “Import response guard,” “Returned scene data,” and “Recovery paths.” The `how` storyboard also places “Successful sharing result” before both sharing mechanism beats.

Those three import beats account for 54 of the reported 163 spoken words, approximately one third of the entire story. Their visuals enumerate `response.ok`, `!response.ok`, `return {}`, `data.elements || null`, `legacy_decodeFromBackend`, and “Outer catch.” The narration never introduces a recipient's retrieval task or explains why a newcomer needs these particular return/error branches to understand sharing a drawing. Meanwhile the main sharing outcome has already appeared before the mechanism, and the story ends the large `how` scene on an empty-object error result rather than the user's useful outcome.

This is the explicit product-contract issue about internal branch dumps crowding user benefit, not a complaint about source symbols in general. `exportToBackend`, JSON serialization, image preparation, and the identifier/key fragment can all be useful source anchors. A tighter sequence would carry the existing drawing through preparation and upload to its conditional shareable URL, then use one plain-language retrieval sentence only if needed, and finish with the focused attachment-handling investigation. Keep detailed import guards and recovery branches in evidence or a deeper technical view. If retaining retrieval, introduce what the person is trying to recover and connect the supplied identifier/key to that purpose without inventing an observed canvas restoration.

## Semantic checks that pass

| Claim / visual | Assessment against supplied pinned source |
| --- | --- |
| Product purpose / “Excalidraw” | README 17–18 supports the whiteboard, open-source, hand-drawn-style, and collaborative description. “Explaining ideas visually” is a reasonable interpretation; the full README also describes creating diagrams and wireframes at line 46. The earlier P1 product-definition omission is resolved. |
| Input / “Share a drawing” | The signature and serialization at 248–257 support drawing elements, editor state, and files as inputs. The task's shareable-link purpose is supported by the later cited return path and explicitly by the supplied README at 73–79. Adding README 79 as a citation here would make the task's product premise easier to check directly, but there is no unsupported product capability in the supplied sources. |
| Result / “Successful sharing result” | 280–285 and 288–293 support the identifier condition, awaiting file saving, and returning the URL with no error. There is no false claim of successful execution or of export checking `response.ok`. “Identifier” should continue to mean the truthy `json.id` branch. |
| Mechanism / “Prepare the scene payload” | 253–260 supports generating a key and calling serialization and compression with that key. The limit correctly avoids claiming cryptographic internals were inspected. |
| Mechanism / “Before posting” | 263–279 supports the initialized-image **and** matching-file predicate, awaited file encoding, and subsequent POST. The prerequisite omitted in the earlier narrow story is now stated. |
| Mechanism / “Import response guard” | 202–224 supports fetching by ID and the non-OK early return before keyed decoding. The HTTP success condition is necessary, not a promise of complete import success. |
| Result / “Returned scene data” | The visual's field expressions match 227–228. The narration's stronger sufficiency statement is the first required finding. |
| Mechanism / “Recovery paths” | 230–240 supports attempting legacy decoding for errors in the inner block and alerting/returning `{}` when execution reaches the outer catch. The story does not explicitly claim legacy recovery succeeds or that every failed legacy promise is caught by the outer catch; the source returns that promise without awaiting it at 235, so those would be unsupported additions. |
| Design / “Fragment-based link” | 282–285 supports the fragment syntax and the source comment's rationale for avoiding a server-bound query parameter. The limit correctly distinguishes design intent from a security audit. |
| Handoff / “Investigate attachment handling” | 270–274, 281, and 288–291 support investigating prepared-file handling and the identifier-gated storage call. The narration does not assert the downstream helper implementations were inspected. This is a supported contribution investigation. |

All ten claims and sixteen source spans were considered semantically, not just accepted because `audit.json` reports exact matching. The `not_executed` evidence mode is consistent with the narration. There is no claim that the target application was run, a share link actually worked, a recipient's drawing rendered, or the encryption was independently verified.
