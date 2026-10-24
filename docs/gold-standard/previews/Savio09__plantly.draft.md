# Gold-standard preview — `Savio09/plantly` (DRAFT)

> **DRAFT for Declan to revise.** Written by Claude from the evidence pack
> (`docs/gold-standard/evidence/Savio09__plantly.md`), strictly against files in that pack.
> Not executed. **Track B (personal / known-author), frontend — README-independence case.**
> Revise as you actually use the repo; you know its real state better than the files do.

## 1. Identity
- **Repo:** Savio09/plantly · **Kind:** mobile app (React Native / Expo)
- **Primary language:** TypeScript · **License:** none declared
- **URL:** https://github.com/Savio09/plantly · default branch `main`
- Stars 0; last pushed 2026-06-18. Personal project.

## 2. What it does
A cross-platform mobile app that reminds you to water/care for plants. *Evidence:* repo
description; `README.md`; routes `app/onboarding.tsx`, `app/new.tsx`, `app/(tabs)/`.
**Confidence:** confident on the intent; **uncertain** on how much is actually implemented
(see §7). **Execution:** not_executed.

## 3. How it works — *this is the README-independence point*
- **Expo + Expo Router (file-based routing).** `package.json`: `"main": "expo-router/entry"`,
  `expo ~51`, `react-native 0.74.5`, `react 18.2`; routes are files under `app/`
  (`_layout.tsx`, `(tabs)/`, `new.tsx`, `onboarding.tsx`). *Evidence:* `package.json`,
  `app.json` (`"plugins": ["expo-router"]`), `app/` tree.
- **State via Zustand.** `zustand` dependency; stores in `store/plantsStore.ts`,
  `store/userStore.ts`. *Evidence:* `package.json`, `store/` tree.
- **UI:** reusable components `components/PlantCard.tsx`, `PlantlyButton.tsx`, `PlantlyImage.tsx`;
  a `theme.ts`. *Evidence:* tree.
- **Device features:** image picking (`expo-image-picker`), haptics, local persistence
  (`@react-native-async-storage/async-storage`), file system, linear-gradient. *Evidence:*
  `package.json` dependencies. TypeScript `strict` on (`tsconfig.json`).

## 4. How to run / setup
- `npm install` / `yarn`, then `expo start` (scripts: `start`, `android`, `ios`, `web`).
  Needs the Expo toolchain / Expo Go or a dev build. *Evidence:* `package.json` `scripts`;
  `app.json`. **Not verified by execution.**
- Setup burden: low for anyone with Expo installed. **Confidence:** uncertain (not run).

## 5. Contribution entry points
- **No `CONTRIBUTING.md`, no issue templates, no CI, and no LICENSE.** *Evidence:* absence in
  the tree. Because there's **no license, the code is not legally open to reuse/contribution
  without the author's permission** — this is a **Track B / personal** repo, so the way in is
  *social*: contact the author (you). That's exactly the friend/own-repo use case.

## 6. Tech facts for fit
React Native 0.74 · Expo SDK 51 · Expo Router · TypeScript (strict) · Zustand · AsyncStorage ·
expo-image-picker / haptics / file-system. No tests, no CI.

## 7. Calibrated uncertainty — *where the README and the code disagree*
- ⚠️ **README over-claims vs. the manifest.** `README.md` lists "Allows for Video uploads",
  but the dependencies include **`expo-image-picker`** (images), with **no video-specific
  dependency** visible. So the README's video claim is **not supported by the code evidence** —
  treat as **uncertain / likely stale**. *This is the core README-independence lesson:* the
  evidence-grounded preview trusts `package.json` over the prose.
- README itself says it's unfinished ("I'll be getting back to this shortly", "More features
  to be added soon") → **the repo is a work in progress.** *Evidence:* `README.md`.
- **confident / not_executed:** it's an Expo/RN/TS plant-care app using Expo Router + Zustand.
- **Not determinable from evidence:** which listed features actually work; app stability.

## 8. Evidence index
`README.md` · `package.json` · `app.json` · `tsconfig.json` · `babel.config.js` ·
`app/` (routes) · `store/` · `components/` · `theme.ts`.
