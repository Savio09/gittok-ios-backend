# Gold-standard preview — `excalidraw/excalidraw` (DRAFT)

> **DRAFT for Declan to revise.** Written by Claude from the evidence pack
> (`docs/gold-standard/evidence/excalidraw__excalidraw.md`), strictly against files in
> that pack. Not executed. Revise freely as you actually use the repo — this is a
> starting point, not the ground truth until you sign off. Track A (public), frontend.

## 1. Identity
- **Repo:** excalidraw/excalidraw · **Kind:** web app (virtual whiteboard)
- **Primary language:** TypeScript · **License:** MIT (`LICENSE`)
- **URL:** https://github.com/excalidraw/excalidraw · default branch `master`
- Stars ~134k; last pushed 2026-10-23 (actively maintained).

## 2. What it does
A browser-based virtual whiteboard for hand-drawn-style diagrams and sketching.
*Evidence:* repo description; `README.md`; topics `canvas, collaboration, diagrams,
drawing, whiteboard`; the deployable app lives in `excalidraw-app/` (`App.tsx`,
`index.tsx`, `index.html`). **Confidence:** confident. **Execution:** not_executed.

## 3. How it works
- **Yarn workspaces monorepo.** Root `package.json` (`"name": "excalidraw-monorepo"`,
  `"workspaces": ["excalidraw-app", "packages/*", "examples/*"]`). The reusable editor is
  split into packages: `packages/excalidraw`, `packages/element`, `packages/common`,
  `packages/math`, `packages/fractional-indexing`, `packages/laser-pointer`,
  `packages/utils`. *Evidence:* `package.json`, `packages/` tree.
- **Stack:** React (dev-dep `@types/react` 19.x) + Vite 5 build, TypeScript 5.9, tested
  with Vitest 3. *Evidence:* root `package.json` devDependencies; `excalidraw-app/vite.config.mts`.
- **Collaboration backend** uses Firebase. *Evidence:* `firebase-project/` (`firebase.json`,
  `firestore.rules`, `firestore.indexes.json`); `excalidraw-app/collab/`, `.../share/`.
  **Confidence:** confident the files exist; uncertain on runtime behavior (not_executed).
- **Tooling:** husky pre-commit + lint-staged + prettier/eslint. *Evidence:* `.husky/pre-commit`,
  `.lintstagedrc.js`, `.eslintrc.json`, root `package.json`.

## 4. How to run / setup
- **Install + run:** `yarn`, then `yarn start` (root script `start` → `yarn --cwd ./excalidraw-app start`).
  Build with `yarn build`. *Evidence:* root `package.json` `scripts`. **Requires Node ≥ 18**
  (`engines.node`). **Not verified by execution.**
- **Docker path exists:** `Dockerfile` + `docker-compose.yml` at root; `.codesandbox/Dockerfile`.
  *Evidence:* those files in the tree. Not verified by execution.
- **Setup burden (inference):** moderate for a newcomer — a multi-package monorepo with a
  package-build chain (`build:packages` builds 6 packages in order). **Confidence:** uncertain.

## 5. Contribution entry points
- `CONTRIBUTING.md` present at root, plus a dev site `dev-docs/` (Docusaurus) with
  `dev-docs/docs/introduction/contributing.mdx`. *Evidence:* those files.
- **Conventional commits** expected (feat/fix/docs/style/refactor…). *Evidence:* CONTRIBUTING excerpt in the pack.
- **Signals it welcomes contributors:** topic `hacktoberfest`; `.github/FUNDING.yml`;
  localization via `crowdin.yml` (translation is often a low-barrier first contribution).
  *Evidence:* topics; `crowdin.yml`; `.github/`.
- ⚠️ **Your "good beginner issues" claim is NOT verifiable from the files.** Whether
  beginner-friendly issues are *currently open* needs the GitHub Issues list, which is not in
  the tree. Tag this **uncertain** until you check Issues directly (label like `good first issue`).

## 6. Tech facts for fit
TypeScript · React 19 · Vite · Vitest · yarn workspaces monorepo · Firebase (collab) ·
Node ≥ 18 · canvas/SVG rendering. Contribution tooling: eslint/prettier/husky, conventional
commits, Crowdin i18n.

## 7. Calibrated uncertainty
- **confident / not_executed:** it's a TS/React whiteboard web app; MIT; monorepo with the
  editor as packages; Firebase files present; CONTRIBUTING exists.
- **uncertain / not_executed:** actual setup smoothness; whether collab works out of the box
  without Firebase credentials; current availability of beginner issues (verify on GitHub).
- **Not determinable from evidence:** issue/PR responsiveness; real first-contribution difficulty.

## 8. Evidence index
`README.md` · root `package.json` · `packages/*` (tree) · `excalidraw-app/` (`App.tsx`,
`index.tsx`, `vite.config.mts`, `collab/`, `share/`) · `firebase-project/*` · `Dockerfile` ·
`docker-compose.yml` · `CONTRIBUTING.md` · `dev-docs/docs/introduction/contributing.mdx` ·
`crowdin.yml` · `.github/FUNDING.yml` · topics/metadata.
