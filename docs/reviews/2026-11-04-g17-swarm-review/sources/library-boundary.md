# Separate remaining classification boundary: packaged Python library

This is **pre-existing G15 classifier policy**, separate from the four repair findings. List it before claiming broader product readiness.

The independent generic `generic/widgetkit` fixture declares a normal static Python package in `pyproject.toml` with Hatchling as its build backend, name/version, and `dependencies = ["starlette>=0.40"]`. Its only Python source is `src/widgetkit/__init__.py`, exporting an ordinary `double()` utility; there is no application object, server call, command declaration, or application entry point. Both files are fully inspected.

The current classifier returns backend/resolved based on only the dependency field. It also discovers importable-library evidence, retaining that as secondary. Removing only the starlette dependency changes the result to library. `preview.Build` displays backend and the new explanation:

> Classified as a backend from the inspected files: "pyproject.toml" declares "project → dependencies → starlette" as "starlette>=0.40".

The claim carries `confidence: confident` with the ordinary bounded-inspection/not-run limit. The source is accurate and the complete *existing policy basis* really is one dependency; this is not an additional lost-source finding. It does show why accurately citing a classifier's basis is insufficient evidence of useful, semantically correct product classification.

## Exact policy locations

- `internal/crawler/classify.go:504`: starlette is in `pyServerDeps`.
- `internal/crawler/classify.go:525-527`: one mandatory Python web-framework declaration becomes a complete backend evidence group. There is no Python server/entrypoint requirement. Compare Go at lines 511-515, which requires both a command and server signal.
- `internal/crawler/classify.go:1112-1123`: `pyServerObservation` accepts project.dependencies or requirements declarations.
- `internal/crawler/classify.go:714-723`: the same complete project without command/GUI entry points produces library evidence.
- `internal/crawler/resolve.go:62-70`: backend precedes library, so the dependency-only backend group wins over importable-library evidence.
- `internal/preview/kindproof.go:60-61,104`: the repair presents the existing inference explicitly and assigns confident, with a bounded/not-run limit.

The entire `detectBackend` and `pyServerObservation` function blocks are byte-for-byte identical between baseline `82eb571` and current `d0bd691`. The starlette list is unchanged. Thus the unsupported application/library distinction predates the repair; the new explanation exposes it rather than adding a stronger detector premise and then failing to cite it. Treat improving or abstaining on framework/library consumers as remaining G15 work. Generic provenance cautions alone do not resolve that boundary.

## Verification

```
go test -overlay /tmp/g17-sources-review-20261104/library-overlay.json -run '^TestIndependentLibraryBackendBoundary$' -v ./internal/crawler
```

Pass: the test asserts the current backend result and library result after dependency removal. Full current output and overlay test source are alongside this note. An optional isolated baseline runtime attempt was blocked by filesystem permission on Go's default build cache; no baseline runtime pass is claimed. The pre-existing-policy conclusion also has direct, exact baseline source comparison evidence.
