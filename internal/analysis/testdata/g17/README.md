# G17 held-out acceptance evidence

These are evaluation inputs, not a product repository list. Glow and Rack were
selected before inspecting their claim output; neither belongs to the original
six-repository study corpus. Capture used the default production selector and
anonymous GitHub acquisition, with no manually added files.

Each `.json` snapshot contains allowlisted provider metadata, capture time,
resolved commit, bounded tree (including Git mode/object ID), selected paths and
consumed file bytes. Each `.preview.json` is the final builder's replayable output.
Metadata is mutable; file citations refer to the captured commit.

| Repository | Commit | Selected / consumed | Claims |
|---|---|---|---|
| charmbracelet/glow | `8b0600fa445535b9c53194c0d4f7fe3cf700ef3e` | 16 / 16 | 24 |
| rack/rack | `da90464e3fa93e3cb49dc2d4ede798e6c6b70b4f` | 14 / 14 | 6 |

The initial live capture occurred November 2, 2026 at 13:10 UTC, before the final
repository-ID revalidation hardening. It consequently lacks the later `Repo.ID`
field. The final claim builder was replayed against these retained sources;
canonical-ID reuse, private visibility changes and the final HTTP service were
verified separately with deterministic upstream integration tests. Future captures
retain the repository ID and perform revalidation. Do not invent missing IDs in
these historical snapshots.

Normal offline verification:

```sh
go test ./internal/analysis -run TestHeldoutPreviewsAreGrounded -count=1
```

Explicit network recapture, subject to anonymous GitHub quotas:

```sh
GITTOK_CAPTURE_G17=1 go test -v ./internal/analysis -run '^TestCaptureG17Heldout$' -count=1
```

After reviewing a justified output change, regenerate only the preview JSON from
the retained evidence without network access:

```sh
GITTOK_UPDATE_G17_PREVIEWS=1 go test ./internal/analysis -run '^TestHeldoutPreviewsAreGrounded$' -count=1
```

Inspect source support before updating a golden. Keep failed cases and acquisition
limits; replacing a difficult repository with an easier one invalidates the holdout.
The source audit and coverage limits are in the
[process record](../../../../docs/reviews/2026-11-02-g17-claim-extraction.md).
