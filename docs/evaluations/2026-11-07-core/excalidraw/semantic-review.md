# Independent semantic and product review: Excalidraw cold story

Verdict: **Request changes for the newcomer product contract.** The nine narrow source interpretations are substantially supported, including their nested conditions. Passing the 17 exact-span checks does not make this a product explanation. The 171-word story introduces an export helper's data types and then narrates helper calls; it does not establish Excalidraw's core product idea or give a newcomer a concrete user task to follow.

Reviewed only `story.json`, `sources.json`, `audit.json`, and the supplied full pinned `excalidraw-app/data/index.ts` at commit `701b901a0f1ba3d0b390d9cdf63d112e25abac4b`. No application execution, UI observation, outside source discovery, or model calls were performed. This review concerns the repository story, not the host application's Folio identity.

## Required finding

**P1 — The opening and resulting story do not explain the repository's product to a newcomer.**

Affected fields:

- `job.preview.description`
- `job.preview.sections[0].body`
- `job.preview.sections[0].claims[0].supports`
- `job.preview.sections[0].claims[0].visual` (`title: "Share a scene"`, line: `"Scene + app state + files → shareable URL"`)
- The following `how` scene's narration and visuals as the continuation of that opening.

The opening is: “To share an Excalidraw scene, this export path accepts elements, app state, and files, and can return a shareable URL.” A reader who does not already know Excalidraw is never told what the product lets a person do, what a scene contains in ordinary user terms, or why they would share it. `elements`, `app state`, `filesMap`, `serializeAsJSON`, `compressData`, and `payload.buffer` are implementation vocabulary. The visual is an API signature rendered as an arrow, so it does not supply the missing product explanation.

The remaining beats have an API-level input/mechanism/result chain, but no concrete user input or example persists through that chain. There is no intelligible object for a newcomer to watch become a result; the user has to supply the meaning of “scene” from prior knowledge. This fails the explicit contract that a newcomer first understand the core idea/product and then understand one use-input → mechanism → result through short visual narration. Concision (171 words, nine beats) does not repair that omission.

The cause is visible in the evidence boundary: all 17 citations are from one backend data helper file. The opening's citations (248–257 and 288–293) establish a function signature, serialization call, and conditional URL return. They do not establish the product's purpose or a user workflow. Repair requires adding pinned product-facing evidence (for example, an appropriate README or actual UI/entry source), then opening with its supported core idea and a concrete user task. Carry that task through the existing, correctly scoped sharing mechanism and its conditional URL result. Preserve the explicit distinction between source interpretation and observed execution. Do not simply assert an uncited whiteboard/UI workflow or claim a recipient's canvas was restored.

## Claim-by-claim semantic check

All narration `supports` fields are duplicated in their scene bodies; both were checked. Visual titles and lines were checked alongside them.

| Claim / beat | Evidence checked | Semantic assessment |
| --- | --- | --- |
| Purpose — “Share a scene” | 248–257; 288–293 | Supported as an API-level capability: takes elements/state/files and can return a URL. “Can” correctly avoids an unconditional success claim. Its product-contract omission is the required finding above. |
| Input — “Select associated files” | 263–268 | Supported. Both `isInitializedImageElement(element)` and truthy `files[element.fileId]` are required by the `&&`. The narration and visual preserve both conditions. This is selection into a map, not proof of a completed upload. |
| Mechanism — “Prepare and send” | 253–260; 276–279 | Supported. The key is generated, database-mode JSON is encoded and passed to `compressData` with that key, then `payload.buffer` is posted. The story does not establish the helper's cryptographic/compression internals or claim they were executed. The full file additionally gates reaching POST on successful file preparation; the story does not promise unconditional POST success. |
| Result — “Successful export branch” | 281–286; 288–293 | Supported. A truthy parsed `json.id` selects this branch. The URL uses the hash, `saveFilesToFirebase` is awaited, and only then is the successful URL result returned. Narration explicitly conditions the return on saving succeeding. It correctly does not infer `response.ok` checking in this export branch. |
| Result — “New-format import success” | 207–216; 217–226; 227–229 | Supported when read with the visual's explicit **new-format** scope. An OK response, successful new-format decoding and parsing, and reaching the return yield `data.elements \|\| null` and `data.appState \|\| null`. The story does not establish application to a visible canvas or image retrieval. The word “Only” in the scene body should retain that new-format scope: the legacy helper at 196–199 can also return those two fields. This is a wording clarification, not an independent false-claim blocker, because the visual already specifies the route and the next beat explicitly introduces fallback. |
| Mechanism — “Import alternatives” | 209–212; 230–235 | Supported. Non-OK HTTP responses alert and return `{}`. Errors inside the inner new-format block invoke the legacy decoder. The story correctly avoids claiming that all import errors trigger legacy decoding or that the fallback necessarily succeeds. |
| Design — hash/source visual | 281–285 | Supported as the source comment's stated rationale: store the key (and ID) in the fragment rather than a query parameter to avoid sending it to the server. The narration attributes that rationale and the limit disclaims a general security guarantee. No security guarantee was established by execution here. |
| Mechanism — “Size-specific failure” | 281; 294–301 | Supported, including the nested condition. This is an `else if` after the truthy-ID branch, so a size-specific error requires both no truthy `json.id` and `json.error_class === "RequestTooLargeError"`. Both narration and visual preserve that conjunction. The limit correctly scopes this to parsed-response handling. |
| Next step — “Next investigation” | 38–39; 270–274 | Supported investigation direction. The import points to `./FileManager`, and the call supplies selected files, the key, and `FILE_UPLOAD_MAX_BYTES`. The narration asks the reader to investigate handling; it does not assert unreviewed helper internals. This meets the contribution-investigation requirement at the source-navigation level. |

## What the audit establishes and does not establish

`audit.json` reports nine claims, 17 citations, no exact-source errors, and `evidence_mode: not_executed` is consistent throughout the story. The narrowed claims avoid the major common overclaims: unconditional export success, fallback on every import failure, demonstrated decryption/security, restored UI, or `RequestTooLargeError` winning over a truthy ID.

The review therefore finds **one required product-quality failure**, rather than fabricating a semantic error in the repaired branch logic. The current artifact is a defensible, concise source walkthrough of sharing helpers. It still needs a supported product introduction and a concrete newcomer-facing task before it meets the requested story contract.
