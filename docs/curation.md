# Repository curation — selection criteria & notes template (G11)

Status: **draft for Declan's review** (ticket SAV-14, `own:declan`, LO **#rightproblem**).
Defines (1) which repos make up the curated set, and (2) the notes template used to write
each repo's hand-made preview.

**Primary purpose — user testing of the medium (from the proposal).** The four hand-made
previews are the **stimulus material for the GitTok arm** of a user study that compares
GitTok's medium against GitHub's native workflow, to find which better helps a developer
reach a well-calibrated *"worth chasing or skip it"* decision, and with less effort. They
are made **by hand now** because the preview *generator* (M5, G14–G19) does not exist yet,
and validating the medium concept before automating it is deliberate sequencing. This is a
`#medium` deliverable. For the comparison to be valid, each preview must faithfully embody
GitTok's thesis (reader-relative, evidence-grounded, calibrated) — which is why the content
is Declan's judgment, not generated.

**Secondary / later reuse — gold standard for the automated eval (#outcomeanalysis).** Once
G14–G19 produce machine previews, these same hand-made previews double as the **ground
truth** those generated previews are measured against. The "a reference must not be written
by an LLM" constraint applies at *that* later stage, not to making the stimulus now.

## Why README-independence

GitTok's thesis is that a useful preview should **not depend on a repository's README**.
A good preview is earned from the repository's **code and evidence** — the file tree,
dependency manifests, configuration, contribution files, and (after setup/inspection)
observed behavior via a sandbox run or agent crawl — so the app still works when docs are
thin, stale, or missing. The curated set therefore **deliberately includes repos with
weak or missing READMEs** to prove the analysis holds without them.

## Two reasons to engage → two selection tracks

GitTok surfaces repos a developer might engage with for **two different reasons**, so
curation has two tracks:

- **Track A — public discovery (popularity-gated).** Unfamiliar public OSS the user has
  no prior connection to. Popularity is a reasonable proxy for "worth a stranger's time."
- **Track B — personal / known-author.** Repos the user has a *personal* reason to engage
  with — a **friend's or collaborator's project, or your own** — that they may want to read
  or contribute to regardless of stars. Popularity is irrelevant here; the reason is social.
  These repos very often have **no README**, which makes Track B the ideal
  README-independence test.

### Track A hard gates

- **Public** and **≥ 1,000 stars**
- **Pushed within the last 30 days** (actively maintained)
- **Clear OSS license** (a recognized SPDX id; not `NOASSERTION`/none)
- **Not a fork** and **not archived**
- **Enough code signal to analyze** — a real software project, not a docs-only,
  awesome-list, or assets-only repository

### Track B gates (relaxed)

- The **star floor and strict-license gates do not apply** — the reason to surface it is
  personal, not popular.
- Still required: **not archived**, **real code signal** (a genuine project, not an empty
  or assets-only repo), and a **real personal/social reason** to surface it.
- Empirically, even weak-README public repos are hard to find above 1k stars — essentially
  every popular repo has a README. Track B is where the **no/stub-README** cases come from.

## Set composition

- **2 frontend-leaning + 2 backend-leaning** (matches the proposal's FE/BE test plan)
- **≥ 2 with weak or missing READMEs** (the README-independence stress test). "Weak"
  means no README, or a stub that does not explain what the project is or how to run it.
- A spread of **project kinds** (web app / backend service / library / CLI) and
  **languages**, so the analysis is exercised across repo shapes

> Note on the live feed vs the gold-standard set: the Sprint-1 live `/v1/feed` currently
> serves four README-rich **Track A** repos (excalidraw, pocketbase, cli/cli, starship).
> The gold-standard set keeps that README-rich coverage **and adds ≥2 README-poor repos
> drawn from Track B** (personal/known-author repos — e.g. from Declan's own profile),
> since popular repos essentially never lack a README. Final picks are made in G12.

## Selected set (G12 — recorded against the criteria, 2026-10-21)

| owner/repo | track | stars | last push | license | kind | README | FE/BE | why it qualifies |
|---|---|---|---|---|---|---|---|---|
| `excalidraw/excalidraw` | A | 134k | 2026-10-17 | MIT | web app | strong | FE | README-rich, technically deep frontend; passes all Track A gates |
| `pocketbase/pocketbase` | A | 61k | 2026-10-14 | MIT | backend service | strong | BE | README-rich, technically deep backend; passes all Track A gates |
| `Savio09/plantly` | B | 0 | 2026-06-18 | none | mobile app | stub (678 B) | FE | personal repo, stub README → README-independence test (FE) |
| `Savio09/face-recognition` | B | 1 | 2026-05-08 | none | ML / service | stub (330 B) | BE | personal repo, stub README → README-independence test (BE) |

The two Track B repos deliberately trade architectural depth for the no/stub-README case;
depth is covered by the Track A pair. Optional swap on record: `face-recognition` →
`Savio09/twyntunes-backend` (a Go service — more technical, but has a ~4 KB README, so it
would test depth rather than README-independence).

## Notes template — the gold-standard preview per repo (G13)

For each repo, write reference notes that map onto the API contract
(`internal/apimodel`, see [api-contract.md](api-contract.md)). **Every claim must cite the
GitHub file(s) or signal that back it** — this is the evidence-grounding thesis, and it is
what makes the gold standard comparable to the backend's output file-for-file.

1. **Identity** — `owner/name`, kind, primary language, license, repo URL.
2. **What it does** — one-sentence summary. *Evidence:* the file(s)/signals it rests on
   (package manifest, entry point, docs if any).
3. **How it works** — key architecture and stack. *Evidence:* manifests
   (`package.json`, `go.mod`, `Cargo.toml`), entry points, config (`docker-compose.yml`, …).
4. **How to run / setup** — steps and setup burden. *Evidence:* manifest scripts,
   `Dockerfile`, CI config, `.env.example`. Mark **"not verified by execution"** unless the
   project was actually run in the sandbox.
5. **Contribution entry points** — good first issues, `CONTRIBUTING` presence, issue
   labels, recent activity. *Evidence:* `CONTRIBUTING.md`, Issues, commit history.
6. **Tech facts for fit** — the languages/frameworks the repo actually uses (reader-relative
   fit is applied later against a user profile; for the gold standard, record the facts).
7. **Calibrated uncertainty** — what could **not** be determined from the evidence, and
   which statements required execution vs inference. Tag each: `confident | uncertain`, and
   `observed_running | not_executed`.
8. **Evidence index** — the list of files/sources consulted (a "SOURCE INDEX"), so the gold
   standard is reproducible and the backend's cited files can be compared against it.

## How these previews are used

**Now — user-testing stimulus (#medium, primary).** G13 fills this template **by hand**
(plus a local run where feasible + a short recording) to produce the GitTok-arm stimulus.
The study shows participants each repo via GitTok (these previews) vs GitHub's native view
and measures decision quality, confidence calibration, and effort/time on the
*"worth chasing or skip"* call. Declan owns the study design, protocol, sessions, and
interpretation.

**Later — automated-eval ground truth (#outcomeanalysis, secondary reuse).** Once the
backend (G14–G19) generates previews into the same `apimodel` shape, these hand-made
previews become the reference it is scored against on three axes:
  - **Claim grounding** — does each cited file actually support its claim? (a blind
    LLM judge can score this, as GitDiagram's `diagram-evidence` eval does)
  - **Coverage** — are the important components/claims present?
  - **Calibration** — did the backend over- or under-claim, and did it correctly flag
    what it could not determine?

## Differentiation reminder

The gold standard is written **reader-relative where it matters** (fit, contribution
readiness), not just a neutral architecture description — that framing, plus personalized
ranking, is what separates GitTok from a repo-explainer like GitDiagram.
