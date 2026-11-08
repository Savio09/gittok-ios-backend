# Classification review — October 30, 2026

Status: findings reproduced; corrective changes proposed, not implemented or
accepted by the project owner in this session.

This record supports GitTok's capstone process documentation. Declan requested
an extensive review of the classification work and a feasibility assessment,
then asked that the investigation be preserved for the process documentation.
Codex performed the inspection and temporary experiments. These are technical
review results, not user-study findings or evidence of product demand.

## Existing process record and earlier agent memory

The existing destination is the Notion
[Process Documentation — Log Book](https://app.notion.com/p/40060d73e6c381f7b44ecd8d7333c70c),
under Fortune's Capstone Hub. It was found through the previous Claude Code
agent's project memory at
`~/.claude/projects/-Users-fdeclan-Public-gittok-ios-backend/memory/`.

Relevant recovered files:

- `MEMORY.md`: index of project context and evidence records.
- `gittok-lo-hc-evidence.md`: running HC/LO evidence log, last recorded update
  October 20; foundation, contract, live feed, bounded ingestion, tests, and
  competitor study.
- `gittok-prototypes-and-contract.md`: three lo-fi concepts, five hi-fi
  directions, contract decisions, and the live-data pivot. These are earlier
  agent reports; this review did not independently inspect those prototypes.
- `notion-capstone-hub.md`: identifies the existing Notion logbook and its
  purpose as a dated process record.

The fetched Notion logbook already covers October 13–24: planning, the API
contract and prototypes, the live-feed pivot, ingestion, competitor research,
two-track curation, and the distinction between study stimulus and later
automated-evaluation reference material. Its historical entries are preserved.
The memory's earlier account of abandoning hand-authored previews must be read
alongside the October 24 clarification that human-reviewed previews are still
needed for the medium-comparison study.

This also corrects a limit of the initial repository-only overview: design
activity was recorded outside this repository, even though Linear design
tickets remained in Backlog. Reported prototypes do not by themselves establish
that a test-ready Figma deliverable or user sessions were completed.

## Scope and question

- Baseline: `3bf15746e053a391e95d7752c9c97684187bb774`.
- Commits: `ddca75d` (G15 classification) and `3bf1574` (G16 file selection).
- Tickets: [G15 / SAV-21](https://linear.app/savios-workspace/issue/SAV-21)
  and [G16 / SAV-22](https://linear.app/savios-workspace/issue/SAV-22).
- Question: can classification from bounded repository evidence reliably guide
  preview format while preserving GitTok's evidence and uncertainty commitments?
- G15 acceptance calls for correct types for all four curated repositories,
  with supporting evidence. Its small synthetic cases do not establish that
  acceptance condition.

## Method and verification

Read the tests, classifier, selector, ingestion completeness fields, preview
schema, existing feed integration, and checked-in evidence packs. Consulted
official Go, Cargo, Python packaging, Docker, and Vite documentation where
language or tool conventions affected the assessment.

The existing `go test ./...` suite passed. GitHub-client tests required local
HTTP-server access outside the sandbox. Crawler statement coverage was 86.4%,
measured with `go test ./internal/crawler -cover`.

Additional checks ran in a temporary copy, leaving implementation and production
tests unchanged. Eleven deliberately selected semantic counterexamples failed.
A separate repeated-input check reproduced nondeterministic classification,
and a stored-evidence check reproduced the PocketBase type error. These are
targeted counterexamples, not a random sample or an accuracy benchmark.

## Reproduced findings

| Finding | Evidence and consequence | Proposed response |
|---|---|---|
| P1: nondeterministic manifest selection | `newRepoIndex` ranges over a Go map and keeps the first package manifest or overwrites other manifests. One run of 1,000 identical root-library + nested-Expo inputs yielded 890 library and 110 mobile classifications. Those proportions are incidental. | Deterministic manifest choice, preserving paths and component scope. |
| P1: nested component overrides repository purpose | PocketBase's `ui/package.json` and HTML entry cause frontend classification before backend evidence is considered. A Go library's example HTTP server also causes backend classification. | Distinguish main product, supporting components, examples, and tooling before choosing a primary label. |
| P1: textual match becomes unsupported factual evidence | A Go comment mentioning `ListenAndServe` produces the statement that Go source calls it; a removed Gin dependency mentioned in a comment is treated as required; project name `flask-utils` becomes a Flask dependency; a comment excluding Flutter becomes an SDK declaration. | Parse declarations and source syntax; attach the actual excerpt; keep static observations separate from execution claims. |
| P1: missing inspection becomes negative evidence | A module and `cmd/service/main.go` path, without fetched source, produce CLI with “no server signal.” The classifier cannot receive truncation/skipping information. | Carry inspection completeness, parse failures, and unavailable evidence; permit an inconclusive result. |
| P2: common ecosystem conventions missed | Implicit Rust binary/library targets become other; Python `[project.scripts]` becomes library; Flask in requirements is ignored; linter-only pyproject becomes library. | Support standard conventions within declared scope, otherwise abstain explicitly. |
| P2: tooling mistaken for frontend product | An npm CLI with a bin entry and Vite development script becomes frontend. | Separate runtime, development, library, executable, and demo signals. |
| Provenance loss | A classification from `apps/mobile/package.json` names only `package.json` in its evidence. | Preserve original file paths and structured references. |

At the reviewed baseline, `Classify` and `SelectFiles` have no production
callers. The live feed still uses `preview.FromMetadata` and its older
metadata heuristic. The evidence-pack command has a separate selector. Thus
unit-level progress does not yet demonstrate the integrated analysis path.

## Stored-evidence results and limits

| Evidence pack | Result | Interpretation |
|---|---|---|
| Excalidraw | frontend | Consistent with expected presentation. |
| Plantly | mobile | Consistent with expected presentation. |
| PocketBase | frontend | Misidentifies its backend role because of the admin UI. |
| Face Recognition | other | Available evidence is insufficient; its implementation was not in the pack. |

Fixtures were extracted from the checked-in packs, including their abbreviated
top-two-level tree displays and fetched file contents. Some files were missing
after fetch failures. This was not a live crawl or a verification of the full
current upstream repositories. In particular, returning other for an
insufficiently inspected ML script is not itself proof of a classification bug.

## Feasibility assessment and proposed next work

The reviewer judged a bounded first version feasible: use classification to
guide further inspection and presentation, and require preview claims to have
their own evidence. This is a technical assessment, not proof that GitTok
improves developers' decisions or a guarantee of the sprint schedule.

Recommended sequence, still pending owner decision and implementation:

1. Make manifest selection deterministic and retain source provenance.
2. Carry evidence completeness and support inconclusive classifications.
3. Validate a versioned corpus containing the curated repositories and mixed
   layouts, including negative and missing-evidence cases.
4. Test the complete ingestion → selection → classification path.
5. Broaden ecosystem support in response to measured failures.

Measure correctness among classified cases and the proportion of cases where
classification can be made. A high abstention rate and an incorrect confident
classification are different problems. Proposed improvements have not been
recorded as completed fixes, user-approved architecture, or closed tickets.

## Process reflection and candidate HC/LO evidence

- **#cs162-testing:** the existing green suite did not expose semantic failures;
  targeted tests added evidence about what the tests were missing. The archived
  checks still need to become maintained regressions when fixes are implemented.
- **#outcomeanalysis:** used explicit expected behavior, repeated identical-input
  checks, and repository examples; recorded sampling limits instead of claiming
  an overall accuracy rate.
- **#modeling / #algorithms:** examined where the one-kind-per-repository model,
  detector precedence, and finite evidence budget lose relevant information.
- **#evidencebased / #sourcequality:** checked ecosystem assumptions against
  official documentation and kept observed failures separate from proposals.
- **#qualitydeliverables:** implementation and statement coverage alone were
  insufficient grounds for declaring the classification deliverable accepted.
- **#navigation / #curation:** Declan requested a second review and preservation
  of failures, connecting the technical work to the later process report.

These are candidate applications for Declan to substantiate and reflect on in
the final submission, not assertions that an academic rubric has been satisfied.
AI disclosure: Claude Code produced earlier implementation/evidence records;
Codex performed this later review and drafted this documentation at Declan's
request. Declan retains ownership of scope, interpretation, and sign-off.

## Preserved artifacts and reproduction

- [Adversarial checks](review_adversarial_test.go.txt): original review-only Go
  test source, stored as text so known failing checks do not enter the normal suite.
- [Evidence fixtures](review_fixtures.json): the exact extracted inputs used.
- [Observed output](review-results.txt): captured failing and diagnostic results.

Run from the repository root. Requires Go 1.25.5 or compatible and a shell with
`mktemp`, `tar`, and `cp`. Tests intentionally return nonzero on the reviewed
baseline. The commands use a temporary archive and do not alter working code.

```sh
review_tmp="$(mktemp -d)"
git archive 3bf15746e053a391e95d7752c9c97684187bb774 | tar -x -C "$review_tmp"
cp docs/reviews/2026-10-30-classification/review_adversarial_test.go.txt "$review_tmp/internal/crawler/review_adversarial_test.go"
cp docs/reviews/2026-10-30-classification/review_fixtures.json "$review_tmp/internal/crawler/review_fixtures.json"
(cd "$review_tmp" && go test ./internal/crawler -run TestReview -v)
```

Official references consulted:
[Go map iteration](https://go.dev/ref/spec#For_range),
[Go parser](https://pkg.go.dev/go/parser),
[Cargo targets](https://doc.rust-lang.org/cargo/reference/cargo-targets.html),
[Python pyproject](https://packaging.python.org/en/latest/guides/writing-pyproject-toml/),
[Vite library mode](https://vite.dev/guide/build.html#library-mode), and
[Docker EXPOSE](https://docs.docker.com/reference/dockerfile/#expose).
