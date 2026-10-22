# Repository curation — selection criteria & notes template (G11)

Status: **draft for Declan's review** (ticket SAV-14, `own:declan`, LO **#rightproblem**).
Defines (1) which public repos qualify for the curated / gold-standard set, and (2) the
notes template used to write each repo's reference ("gold standard") preview — the
ground truth the backend's generated previews (M5, G14–G19) are measured against.

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

## Candidate record (fill one row per repo in G12)

| owner/repo | stars | last push | license | kind | README strength | FE/BE | why it qualifies |
|---|---|---|---|---|---|---|---|
| … | … | … | … | web/service/lib/cli | none / stub / weak / strong | FE/BE | … |

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

## How the gold standard is used (feeds #outcomeanalysis)

- G13 fills this template **by hand** (plus a local run where feasible + a short recording)
  → the **ground truth**.
- The backend (G14–G19) generates previews into the same `apimodel` shape.
- Evaluation compares generated vs gold standard on three axes:
  - **Claim grounding** — does each cited file actually support its claim? (a blind
    LLM judge can score this, as GitDiagram's `diagram-evidence` eval does)
  - **Coverage** — are the important components/claims present?
  - **Calibration** — did the backend over- or under-claim, and did it correctly flag
    what it could not determine?

## Differentiation reminder

The gold standard is written **reader-relative where it matters** (fit, contribution
readiness), not just a neutral architecture description — that framing, plus personalized
ranking, is what separates GitTok from a repo-explainer like GitDiagram.
