# Product relevance review, November 4, 2026

Reviewed `82eb571..d0bd691`, tests first, then the implementation. No application,
shared fixture, git, or plan edits. Code-review skill applied; nested and external
model review skipped under the review task's explicit prohibition.

## Findings

### P2: Inconclusive candidates receive an arbitrary primary/secondary split

Location: `/Users/fdeclan/Public/gittok-ios-backend/internal/preview/relevance.go:76-80`.

`newRanker` picks `Products[0]` even when `Classification.Status` is inconclusive.
`Products` then has no deciding component: it contains the competing candidates
in inventory order. This violates the handoff's express requirement not to pick
the first sorted candidate. It also discards import priority for every later
candidate and allocates six of eight dependency slots to the arbitrary primary.

Minimal repro: an independent Cobra CLI at `a/` (nine direct dependencies) and
a React frontend at `m/` (nine dependencies). Both are competing product candidates;
the classifier abstains. Rename only `a/` to `z/`. The dependency allocation flips
from six CLI/two frontend to six frontend/two CLI. Cobra disappears despite being
the decisive evidence for a plausible product, while both CLI slots are ordinary
lexically earlier dependencies. Root owner/repository identity never changes.

Fix: distinguish resolved deciding components from inconclusive peers; preserve
component-local import/classification relevance among all plausible products and
apply the bounded budget without inventing a primary from the candidate order.

Regression: `TestSwarmInconclusiveCandidatesArePeers` in `review_test.go.txt`.

### P2: Helper imports are promoted as inspected product imports

Location: `/Users/fdeclan/Public/gittok-ios-backend/internal/preview/relevance.go:87-89`.

`CollectImports` retains the importing file, but the ranker drops `Import.File`
and indexes every import solely by manifest component. A helper below `script/`,
`.github/`, `examples/`, or `tests/` within the root Go module can therefore
promote a declaration from root `go.mod` to the product-import tier. The later
supporting-path check sees the declaration path `go.mod`, not its helper import.
This leaves the R1/R2 supporting-tool displacement defect open for shared modules.

Minimal repro: `cmd/app/main.go` imports Cobra and `example.com/zproduct`;
`script/helper.go` imports nine `example.com/ahelper0..8` dependencies declared
in the same root `go.mod`. The output is Cobra plus seven helper-only dependencies;
the actually imported product dependency is absent. All eight declarations
escape the lower-priority budget because their witness path is root `go.mod`.

Fix: retain importing-file scope when deriving import priority and apply the
same product/support policy used for fact selection, including tool-product
fallback. Do not globally discard all imports under tooling-like paths.

Regression: `TestSwarmHelperImportsDisplaceProduct` in `review_test.go.txt`.

### P2: Adding a root README suppresses the actual tool product's source construct

Location: `/Users/fdeclan/Public/gittok-ios-backend/internal/preview/relevance.go:90-93`
and `:119`.

`anyPrimary` means any fact outside a supporting-looking path, including an
ordinary root README. It is then used as proof that code under `tools/` is
supporting. The classifier can have no chosen product in this layout, so the
explicit-primary exception does not apply.

Minimal repro: the only executable lives in `tools/app/main.go`, with Cobra,
`http.ListenAndServe`, and a `tools/app/go.mod` declaring three direct dependencies.
Without a README, the preview contains the server construct and all three
declarations. Add only `README.md` containing the product description. The server
construct disappears and declarations are cut to two. This exceeds a harmless
3-to-2 declaration change: adding documentation removes the sole inspected
product behavior from the walkthrough. The maintained tool-only-path test omits
a root README and misses this case.

Fix: determine whether another actual product exists from typed product evidence;
do not let an unrelated documentation fact turn a path hint into a helper verdict.

Regression: `TestSwarmToolProductWithREADME` in `review_test.go.txt`.

## Reproduction and coverage

```
go test -count=1 -overlay docs/reviews/2026-11-04-g17-swarm-review/product/overlay.json ./internal/preview -run TestSwarm -v
python3 docs/reviews/2026-11-04-g17-swarm-review/product/audit.py
```

The Go command exits 1 on the three intentional regression assertions. Exact
output: `results.txt`. The overlay adds a virtual test file and leaves live Go
files untouched. Initial sandbox cache denial was rerun with test escalation.

Rebuilt previews are saved as `cli__cli.json`, `pocketbase__pocketbase.json`,
`charmbracelet__glow.json`, and `rack__rack.json`, using recorded production-selected
inputs without extras or network reads. Observed:

| Repository | Kind | Claims | Sources, including additional | Dependencies |
| --- | --- | ---: | ---: | ---: |
| cli/cli | cli | 11 | 13 | 8 |
| pocketbase/pocketbase | other | 13 | 13 | 8 |
| charmbracelet/glow | cli | 12 | 13 | 8 |
| rack/rack | other | 3 | 3 | 0 |

CLI retains Cobra and omits the helper gateway. PocketBase keeps six main-module
dependencies, both UI dependencies and UI build/dev commands. Glow retains Cobra.
Rack gives an attributed complete README paragraph and a short abstention note.
No helper gateway construct appeared in these recorded outputs.

The independent Python audit reads source bytes directly rather than calling the
production validator. Across all 42 sources, file sources name consumed files,
carry the recorded pinned commit, line ranges fit raw file lines, primary quotes
fit both source bytes and cited lines, and named declarations appear in their
cited lines. Additional kind-proof sources are included in those checks. Both
CLI and Glow show Cobra plus package-main evidence. All sections are
`not_executed`; each output has at most two short notes with no leaked paths or
classifier diagnostics. Missing line positions do not produce a false gap here.

No repository-name whitelist found. Existing permutation tests cover input order;
the adversarial directory rename exposes semantic dependence beyond equal-priority
tie-breaking. This review did not run the full suite, live captures, credential,
cache or concurrency checks; those belong to the other reviewers/integrator.
