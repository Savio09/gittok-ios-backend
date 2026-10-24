# Evidence pack — `excalidraw/excalidraw`

> Auto-generated raw material for the hand-made gold-standard preview (G13).
> It reproduces what the ingestion client sees; it makes **no claims**. Write the preview against this, citing the files below.

_Generated 2026-10-24 via `cmd/evidencepack`._

## Metadata

- **Full name:** excalidraw/excalidraw
- **Description:** Virtual whiteboard for sketching hand-drawn like diagrams
- **Primary language:** TypeScript
- **Stars:** 134466
- **License:** MIT
- **Default branch:** master
- **Last pushed:** 2026-10-23T15:10:12Z
- **Archived:** false · **Fork:** false
- **Topics:** canvas, collaboration, diagrams, drawing, hacktoberfest, productivity, whiteboard
- **URL:** https://github.com/excalidraw/excalidraw

## Structure (top 2 levels)

```
.codesandbox/
  Dockerfile
  tasks.json
.dockerignore
.editorconfig
.env.development
.env.production
.env.test
.eslintignore
.eslintrc.json
.gitattributes
.github/
  FUNDING.yml
  assets/
  copilot-instructions.md
  workflows/
.gitignore
.husky/
  pre-commit
.lintstagedrc.js
.npmrc
.prettierignore
.watchmanconfig
AGENTS.md
CLAUDE.md
CONTRIBUTING.md
Dockerfile
LICENSE
README.md
crowdin.yml
dev-docs/
  .gitignore
  README.md
  babel.config.js
  docs/
  docusaurus.config.js
  package.json
  sidebars.js
  src/
  static/
  tsconfig.json
  vercel.json
  yarn.lock
docker-compose.yml
examples/
  with-nextjs/
  with-script-in-browser/
excalidraw-app/
  App.tsx
  CustomStats.tsx
  ExcalidrawPlusIframeExport.tsx
  app-jotai.ts
  app-language/
  app_constants.ts
  bug-issue-template.js
  collab/
  components/
  data/
  debugCollaborators.ts
  global.d.ts
  index.html
  index.scss
  index.tsx
  package.json
  sentry.ts
  share/
  tests/
  useHandleAppTheme.ts
  vite-env.d.ts
  vite.config.mts
firebase-project/
  .firebaserc
  .gitignore
  firebase.json
  firestore.indexes.json
  firestore.rules
  storage.rules
package.json
packages/
  common/
  element/
  eslintrc.base.json
  excalidraw/
  fractional-indexing/
  laser-pointer/
  math/
  tsconfig.base.json
  utils/
public/
  Assistant-Regular.woff2
  Cascadia.woff2
  Virgil.woff2
  _headers
  android-chrome-192x192.png
  android-chrome-512x512.png
  apple-touch-icon.png
  favicon-16x16.png
  favicon-32x32.png
  favicon.ico
  favicon.svg
  maskable_icon_x192.png
  maskable_icon_x512.png
  og-image-3.png
  robots.txt
  screenshots/
  service-worker.js
  sidebar-comments-promo-dark.jpg
  sidebar-comments-promo-light.jpg
  sidebar-presentation-promo-dark.jpg
  sidebar-presentation-promo-light.jpg
scripts/
  build-locales-coverage.js
  build-node.js
  build-version.js
  buildBase.js
  buildDocs.js
  buildPackage.js
  buildUtils.js
  buildWasm.js
  locales-coverage-description.js
  release.js
  updateChangelog.js
  wasm/
  woff2/
setupTests.ts
tsconfig.json
vercel.json
vitest.config.mts
yarn.lock
```

_Total entries ingested: 1463._

## Key files

Selected 16 candidate key files; 16 fetched, 0 skipped.

### `package.json`

```json
{
  "private": true,
  "name": "excalidraw-monorepo",
  "packageManager": "yarn@1.22.22",
  "workspaces": [
    "excalidraw-app",
    "packages/*",
    "examples/*"
  ],
  "devDependencies": {
    "@babel/preset-env": "7.26.9",
    "@excalidraw/eslint-config": "1.0.3",
    "@excalidraw/prettier-config": "1.0.2",
    "@types/chai": "4.3.0",
    "@types/jest": "27.4.0",
    "@types/lodash.throttle": "4.1.7",
    "@types/react": "19.0.10",
    "@types/react-dom": "19.0.4",
    "@types/socket.io-client": "3.0.0",
    "@vitejs/plugin-react": "3.1.0",
    "@vitest/coverage-v8": "3.0.7",
    "@vitest/ui": "2.0.5",
    "chai": "4.3.6",
    "dotenv": "16.0.1",
    "eslint-config-prettier": "8.5.0",
    "eslint-config-react-app": "7.0.1",
    "eslint-plugin-import": "2.31.0",
    "eslint-plugin-prettier": "3.3.1",
    "http-server": "14.1.1",
    "husky": "7.0.4",
    "jsdom": "22.1.0",
    "lint-staged": "12.3.7",
    "pepjs": "0.5.3",
    "prettier": "2.6.2",
    "rewire": "6.0.0",
    "rimraf": "^5.0.0",
    "typescript": "5.9.3",
    "vite": "5.0.12",
    "vite-plugin-checker": "0.7.2",
    "vite-plugin-ejs": "1.7.0",
    "vite-plugin-pwa": "0.21.1",
    "vite-plugin-svgr": "4.2.0",
    "vitest": "3.0.6",
    "vitest-canvas-mock": "0.3.3"
  },
  "engines": {
    "node": ">=18.0.0"
  },
  "homepage": ".",
  "prettier": "@excalidraw/prettier-config",
  "scripts": {
    "build-node": "node ./scripts/build-node.js",
    "build:app:docker": "yarn --cwd ./excalidraw-app build:app:docker",
    "build:app": "yarn --cwd ./excalidraw-app build:app",
    "build:common": "yarn --cwd ./packages/common build:esm",
    "build:element": "yarn --cwd ./packages/element build:esm",
    "build:excalidraw": "yarn --cwd ./packages/excalidraw build:esm",
    "build:math": "yarn --cwd ./packages/math build:esm",
    "build:fractional-indexing": "yarn --cwd ./packages/fractional-indexing build:esm",
    "build:laser-pointer": "yarn --cwd ./packages/laser-pointer build:esm",
    "build:packages": "yarn build:common && yarn build:fractional-indexing && yarn build:laser-pointer && yarn build:math && yarn build:element && yarn build:excalidraw",
    "build:version": "yarn --cwd ./excalidraw-app build:version",
    "build": "yarn --cwd ./excalidraw-app build",
    "build:preview": "yarn --cwd ./excalidraw-app build:preview",
    "start": "yarn --cwd ./excalidraw-app start",
    "start:production": "yarn --cwd ./excalidraw-app start:production",
    "start:example": "yarn build:packages && yarn --cwd ./examples/with-script-in-browser start",
    "test:all": "yarn test:typecheck && yarn test:code && yarn test:other && yarn test:app --watch=false",
    "test:app": "vitest",
    "test:code": "eslint --max-warnings=0 --ext .js,.ts,.tsx .",
    "test:other": "yarn prettier --list-different",
    "test:typecheck": "tsc",
    "test:update": "yarn test:app --update --watch=false",
    "test": "yarn test:app",
    "test:coverage": "vitest --coverage",
    "test:coverage:watch": "vitest --coverage --watch",
    "test:ui": "yarn test --ui --coverage.enabled=true",
    "fix:code": "yarn test:code --fix",
    "fix:other": "yarn prettier --write",
    "fix": "yarn fix:other && yarn fix:code",
    "locales-coverage": "node scripts/build-locales-coverage.js",
    "locales-coverage:description": "node scripts/locales-coverage-description.js",
    "prepare": "husky install",
    "prettier": "prettier \"**/*.{css,scss,json,md,html,yml}\" --ignore-path=.eslintignore",
    "release": "node scripts/release.js",
    "release:test": "node scripts/release.js --tag=test",
    "release:next": "node scripts/release.js --tag=next",
    "release:latest": "node scripts/release.js --tag=latest",
    "rm:build": "rimraf --glob excalidraw-app/build excalidraw-app/dist excalidraw-app/dev-dist packages/*/dist packages/*/build examples/*/build examples/*/dist",
    "rm:node_modules": "rimraf --glob node_modules excalidraw-app/node_modules packages/*/node_modules",
    "clean-install": "yarn rm:node_modules && yarn install"
  },
  "resolutions": {
    "strip-ansi": "6.0.1"
  }
}
```

### `dev-docs/package.json`

```json
{
  "name": "docs",
  "version": "0.0.0",
  "private": true,
  "scripts": {
    "docusaurus": "docusaurus",
    "start": "docusaurus start --port 3003",
    "build": "docusaurus build",
    "swizzle": "docusaurus swizzle",
    "deploy": "docusaurus deploy",
    "clear": "docusaurus clear",
    "serve": "docusaurus serve",
    "write-translations": "docusaurus write-translations",
    "write-heading-ids": "docusaurus write-heading-ids",
    "typecheck": "tsc"
  },
  "dependencies": {
    "@docusaurus/core": "2.2.0",
    "@docusaurus/preset-classic": "2.2.0",
    "@docusaurus/theme-live-codeblock": "2.2.0",
    "@excalidraw/excalidraw": "0.18.0",
    "@mdx-js/react": "^1.6.22",
    "clsx": "^1.2.1",
    "docusaurus-plugin-sass": "0.2.3",
    "prism-react-renderer": "^1.3.5",
    "react": "18.2.0",
    "react-dom": "18.2.0",
    "sass": "1.57.1"
  },
  "devDependencies": {
    "@docusaurus/module-type-aliases": "2.0.0-rc.1",
    "@tsconfig/docusaurus": "^1.0.5",
    "docusaurus2-dotenv": "1.4.0",
    "typescript": "^4.7.4"
  },
  "browserslist": {
    "production": [
      ">0.5%",
      "not dead",
      "not op_mini all"
    ],
    "development": [
      "last 1 chrome version",
      "last 1 firefox version",
      "last 1 safari version"
    ]
  },
  "engines": {
    "node": ">=16.14"
  }
}
```

### `excalidraw-app/package.json`

```json
{
  "name": "excalidraw-app",
  "version": "1.0.0",
  "private": true,
  "homepage": ".",
  "browserslist": {
    "production": [
      ">0.2%",
      "not dead",
      "not ie <= 11",
      "not op_mini all",
      "not safari < 12",
      "not kaios <= 2.5",
      "not edge < 79",
      "not chrome < 70",
      "not and_uc < 13",
      "not samsung < 10"
    ],
    "development": [
      "last 1 chrome version",
      "last 1 firefox version",
      "last 1 safari version"
    ]
  },
  "engines": {
    "node": ">=18.0.0"
  },
  "dependencies": {
    "@excalidraw/random-username": "1.0.0",
    "@sentry/browser": "9.0.1",
    "callsites": "4.2.0",
    "firebase": "11.3.1",
    "i18next-browser-languagedetector": "6.1.4",
    "idb-keyval": "6.0.3",
    "jotai": "2.11.0",
    "react": "19.0.0",
    "react-dom": "19.0.0",
    "socket.io-client": "4.7.2",
    "uqr": "0.1.2",
    "vite-plugin-html": "3.2.2"
  },
  "prettier": "@excalidraw/prettier-config",
  "scripts": {
    "build-node": "node ./scripts/build-node.js",
    "build:app:docker": "cross-env VITE_APP_DISABLE_SENTRY=true vite build",
    "build:app": "cross-env VITE_APP_GIT_SHA=$VERCEL_GIT_COMMIT_SHA cross-env VITE_APP_ENABLE_TRACKING=true vite build",
    "build:version": "node ../scripts/build-version.js",
    "build": "yarn build:app && yarn build:version",
    "start": "yarn && vite",
    "start:production": "yarn build && yarn serve",
    "serve": "npx http-server build -a localhost -p 5001 -o",
    "build:preview": "yarn build && vite preview --port 5000"
  },
  "devDependencies": {
    "vite-plugin-sitemap": "0.7.1"
  }
}
```

### `CONTRIBUTING.md`

```markdown
# Contributing

Head over to the [docs](https://docs.excalidraw.com/docs/introduction/contributing)
```

### `dev-docs/docs/introduction/contributing.mdx`

```
# Contributing

Pull requests are welcome. For major changes, please [open an issue](https://github.com/excalidraw/excalidraw/issues/new) first to discuss what you would like to change.

We have a [roadmap](https://github.com/orgs/excalidraw/projects/3) which we strongly recommend to go through and check if something interests you.
For new contributors we would recommend to start with *Easy* tasks.

In case you want to pick up something from the roadmap, comment on that issue and one of the project maintainers will assign it to you, post which you can discuss in the issue and start working on it.

## Setup

1. Fork and clone the repo
1. Run `yarn` to install dependencies
1. Create a branch for your PR with `git checkout -b your-branch-name`

> To keep `master` branch pointing to remote repository and make pull requests from branches on your fork, run:
>
> ```bash
> git remote add upstream https://github.com/excalidraw/excalidraw.git
> git fetch upstream
> git branch --set-upstream-to=upstream/master master
> ```

## Pull Request Guidelines

Don't worry if you get any of the below wrong, or if you don't know how. We'll gladly help out.

### Title

Make sure the title starts with a semantic prefix:

- **feat**: A new feature
- **fix**: A bug fix
- **docs**: Documentation only changes
- **style**: Changes that do not affect the meaning of the code (white-space, formatting, missing semi-colons, etc)
- **refactor**: A code change that neither fixes a bug nor adds a feature
- **perf**: A code change that improves performance
- **test**: Adding missing tests or correcting existing tests
- **build**: Changes that affect the build system or external dependencies (example scopes: gulp, broccoli, npm)
- **ci**: Changes to our CI configuration files and scripts (example scopes: Travis, Circle, BrowserStack, SauceLabs)
- **chore**: Other changes that don't modify src or test files
- **revert**: Reverts a previous commit

### Testing

Once you submit your pull request it will automatically be tested. Be sure to check the results of the test and fix any issues that arise.

It's also a good idea to consider if your change should include additional tests. This is highly recommended for new features or bug-fixes. For example, it's good practice to create a test for each bug you fix which ensures that we don't regress the code in the future.

Finally - always manually test your changes using the convenient staging environment deployed for each pull request. As much as local development attempts to replicate production, there can still be subtle differences in behavior. For larger features consider testing your change in multiple browsers as well.

:::note
Some checks, such as the `lint` and `test`, require approval from the maintainers to run. 
They will appear as `Expected — Waiting for status to be reported` in the PR checks when they are waiting for approval.
:::

## Translating

To translate Excalidraw into other languages, please visit [our Crowdin page](https://crowdin.com/project/excalidraw). To add a new language, [open an issue](https://github.com/excalidraw/excalidraw/issues/new) so we can get things set up on our end first.

Translations will be available on the app if they exceed a certain threshold of completion (currently **85%**).
```

### `README.md`

```markdown
<a href="https://excalidraw.com/" target="_blank" rel="noopener">
  <picture>
    <source media="(prefers-color-scheme: dark)" alt="Excalidraw" srcset="https://excalidraw.nyc3.cdn.digitaloceanspaces.com/github/excalidraw_github_cover_2_dark.png" />
    <img alt="Excalidraw" src="https://excalidraw.nyc3.cdn.digitaloceanspaces.com/github/excalidraw_github_cover_2.png" />
  </picture>
</a>

<h4 align="center">
  <a href="https://excalidraw.com">Excalidraw Editor</a> |
  <a href="https://plus.excalidraw.com/blog">Blog</a> |
  <a href="https://docs.excalidraw.com">Documentation</a> |
  <a href="https://plus.excalidraw.com">Excalidraw+</a>
</h4>

<div align="center">
  <h2>
    An open source virtual hand-drawn style whiteboard. </br>
    Collaborative and end-to-end encrypted. </br>
  <br />
  </h2>
</div>

<br />
<p align="center">
  <a href="https://github.com/excalidraw/excalidraw/blob/master/LICENSE">
    <img alt="Excalidraw is released under the MIT license." src="https://img.shields.io/badge/license-MIT-blue.svg"  /></a>
  <a href="https://www.npmjs.com/package/@excalidraw/excalidraw">
    <img alt="npm downloads/month" src="https://img.shields.io/npm/dm/@excalidraw/excalidraw"  /></a>
  <a href="https://docs.excalidraw.com/docs/introduction/contributing">
    <img alt="PRs welcome!" src="https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat"  /></a>
  <a href="https://discord.gg/UexuTaE">
    <img alt="Chat on Discord" src="https://img.shields.io/discord/723672430744174682?color=738ad6&label=Chat%20on%20Discord&logo=discord&logoColor=ffffff&widget=false"/></a>
  <a href="https://deepwiki.com/excalidraw/excalidraw">
    <img alt="Ask DeepWiki" src="https://deepwiki.com/badge.svg" /></a>
  <a href="https://twitter.com/excalidraw">
    <img alt="Follow Excalidraw on Twitter" src="https://img.shields.io/twitter/follow/excalidraw.svg?label=follow+@excalidraw&style=social&logo=twitter"/></a>
</p>

<div align="center">
  <figure>
    <a href="https://excalidraw.com" target="_blank" rel="noopener">
      <img src="https://excalidraw.nyc3.cdn.digitaloceanspaces.com/github%2Fproduct_showcase.png" alt="Product showcase" />
    </a>
    <figcaption>
      <p align="center">
        Create beautiful hand-drawn like diagrams, wireframes, or whatever you like.
      </p>
    </figcaption>
  </figure>
</div>

## Features

The Excalidraw editor (npm package) supports:

- 💯&nbsp;Free & open-source.
- 🎨&nbsp;Infinite, canvas-based whiteboard.
- ✍️&nbsp;Hand-drawn like style.
- 🌓&nbsp;Dark mode.
- 🏗️&nbsp;Customizable.
- 📷&nbsp;Image support.
- 😀&nbsp;Shape libraries support.
- 🌐&nbsp;Localization (i18n) support.
- 🖼️&nbsp;Export to PNG, SVG & clipboard.
- 💾&nbsp;Open format - export drawings as an `.excalidraw` json file.
- ⚒️&nbsp;Wide range of tools - rectangle, circle, diamond, arrow, line, free-draw, eraser...
- ➡️&nbsp;Arrow-binding & labeled arrows.
- 🔙&nbsp;Undo / Redo.
- 🔍&nbsp;Zoom and panning support.

## Excalidraw.com

The app hosted at [excalidraw.com](https://excalidraw.com) is a minimal showcase of what you can build with Excalidraw. Its [source code](https://github.com/excalidraw/excalidraw/tree/master/excalidraw-app) is part of this repository as well, and the app features:

- 📡&nbsp;PWA support (works offline).
- 🤼&nbsp;Real-time collaboration.
- 🔒&nbsp;End-to-end encryption.
- 💾&nbsp;Local-first support (autosaves to the browser).
- 🔗&nbsp;Shareable links (export to a readonly link you can share with others).

We'll be adding these features as drop-in plugins for the npm package in the future.

## Quick start

**Note:** following instructions are for installing the Excalidraw [npm package](https://www.npmjs.com/package/@excalidraw/excalidraw) when integrating Excalidraw into your own app. To run the repository locally for development, please refer to our [Development Guide](https://docs.excalidraw.com/docs/introduction/development).

Use `npm` or `yarn` to install the package.

```bash
npm install react react-dom @excalidraw/excalidraw
# or
yarn add react react-dom @excalidraw/excalidraw
```

Check out our [documentation](https://docs.excalidraw.com/docs/@excalidraw/excalidraw/installation) for more details!

## Contributing

- Missing something or found a bug? [Report here](https://github.com/excalidraw/excalidraw/issues).
- Want to contribute? Check out our [contribution guide](https://docs.excalidraw.com/docs/introduction/contributing) or let us know on [Discord](https://discord.gg/UexuTaE).
- Want to help with translations? See the [translation guide](https://docs.excalidraw.com/docs/introduction/contributing#translating).

## Integrations

- [VScode extension](https://marketplace.visualstudio.com/items?itemName=pomdtr.excalidraw-editor)
- [npm package](https://www.npmjs.com/package/@excalidraw/excalidraw)

## Who's integrating Excalidraw

[Google Cloud](https://googlecloudcheatsheet.withgoogle.com/architecture) • [Meta](https://meta.com/) • [CodeSandbox](https://codesandbox.io/) • [Obsidian Excalidraw](https://github.com/zsviczian/obsidian-excalidraw-plugin) • [Replit](https://replit.com/) • [Slite](https://slite.com/) • [Notion](https://notion.so/) • [HackerRank](https://www.hackerrank.com/) • and many others

## Sponsors & support

If you like the project, you can become a sponsor at [Open Collective](https://opencollective.com/excalidraw) or use [Excalidraw+](https://plus.excalidraw.com/).

## Thank you for supporting Excalidraw

[<img src="https://opencollective.com/excalidraw/tiers/sponsors/0/avatar.svg?avatarHeight=120"/>](https://opencollective.com/excalidraw/tiers/sponsors/0/website) [<img src="https://opencollective.com/excalidraw/tiers/sponsors/1/avatar.svg?avatarHeight=120"/>](https://opencollective.com/excalidraw/tiers/sponsors/1/website) [<img src="https://opencollective.com/excalidraw/tiers/sponsors/2/avatar.svg?avatarHeight=120"/>](https://opencollective.com/excalidraw/tiers/sponsors/2/website) [<img src="https://opencollective.com/excalidraw/tiers/sponsors/3/avatar.svg?avatarHeight=120"/>](https://opencollective.com/excalidraw/tiers/sponsors/3/website) [<img src="https://opencollective.com/excalidraw/tiers/sponsors/4/avatar.svg?avatarHeight=120"/>](https://opencollective.com/excalidraw/tiers/sponsors/4/website) [<img src="https://opencollective.com/excalidraw/tiers/sponsors/5/avatar.svg?avatarHeight=120"/>](https://opencollective.com/excalidraw/tiers/sponsors/5/website) [<img src="https://opencollective.com/excalidraw/tiers/sponsors/6/avatar.svg?avatarHeight=120"/>](https://opencollective.com/excalidraw/tiers/sponsors/6/website) [<img src="https://opencollective.com/excalidraw/tiers/sponsors/7/avatar.svg?avatarHeight=120"/>](https://opencollective.com/excalidraw/tiers/sponsors/7/website) [<img src="https://opencollective.com/excalidraw/tiers/sponsors/8/avatar.svg?avatarHeight=120"/>](https://opencollective.com/excalidraw/tiers/sponsors/8/website) [<img src="https://opencollective.com/excalidraw/tiers/sponsors/9/avatar.svg?avatarHeight=120"/>](https://opencollective.com/excalidraw/tiers/sponsors/9/website) [<img src="https://opencollective.com/excalidraw/tiers/sponsors/10/avatar.svg?avatarHeight=120"/>](https://opencollective.com/excalidraw/tiers/sponsors/10/website)

<a href="https://opencollective.com/excalidraw#category-CONTRIBUTE" target="_blank"><img src="https://opencollective.com/excalidraw/tiers/backers.svg?avatarHeight=32"/></a>

Last but not least, we're thankful to these companies for offering their services for free:

[![Vercel](./.github/assets/vercel.svg)](https://vercel.com) [![Sentry](./.github/assets/sentry.svg)](https://sentry.io) [![Crowdin](./.github/assets/crowdin.svg)](https://crowdin.com)
```

### `dev-docs/README.md`

```markdown
# Website

This website is built using [Docusaurus 2](https://docusaurus.io/), a modern static website generator.

### Installation

```
$ yarn
```

### Local Development

```
$ yarn start
```

This command starts a local development server and opens up a browser window. Most changes are reflected live without having to restart the server.

### Build

```
$ yarn build
```

This command generates static content into the `build` directory and can be served using any static contents hosting service.

### Deployment

Using SSH:

```
$ USE_SSH=true yarn deploy
```

Not using SSH:

```
$ GIT_USER=<Your GitHub username> yarn deploy
```

If you are using GitHub pages for hosting, this command is a convenient way to build the website and push to the `gh-pages` branch.
```

### `examples/with-nextjs/README.md`

```markdown
This is a [Next.js](https://nextjs.org/) project bootstrapped with [`create-next-app`](https://github.com/vercel/next.js/tree/canary/packages/create-next-app).

## Getting Started

First, run the development server:

```bash
npm run dev
# or
yarn dev
# or
pnpm dev
# or
bun dev
```

Open [http://localhost:3000](http://localhost:3005) with your browser to see the result.

You can start editing the page by modifying `app/page.tsx`. The page auto-updates as you edit the file.

This project uses [`next/font`](https://nextjs.org/docs/basic-features/font-optimization) to automatically optimize and load Inter, a custom Google Font.

## Learn More

To learn more about Next.js, take a look at the following resources:

- [Next.js Documentation](https://nextjs.org/docs) - learn about Next.js features and API.
- [Learn Next.js](https://nextjs.org/learn) - an interactive Next.js tutorial.

You can check out [the Next.js GitHub repository](https://github.com/vercel/next.js/) - your feedback and contributions are welcome!

## Deploy on Vercel

The easiest way to deploy your Next.js app is to use the [Vercel Platform](https://vercel.com/new?utm_medium=default-template&filter=next.js&utm_source=create-next-app&utm_campaign=create-next-app-readme) from the creators of Next.js.

Check out our [Next.js deployment documentation](https://nextjs.org/docs/deployment) for more details.
```

### `Dockerfile`

```dockerfile
FROM --platform=${BUILDPLATFORM} node:24@sha256:8530f76a96d88820d288761f022e318970dda93d01536919fbc16076b7983e63 AS build

WORKDIR /opt/node_app

COPY . .

# do not ignore optional dependencies:
# Error: Cannot find module @rollup/rollup-linux-x64-gnu
RUN --mount=type=cache,target=/root/.cache/yarn \
    npm_config_target_arch=${TARGETARCH} yarn --frozen-lockfile --network-timeout 600000

ARG NODE_ENV=production

RUN npm_config_target_arch=${TARGETARCH} yarn build:app:docker

FROM nginx:stable-alpine-slim@sha256:2c605dbeab79a6b2a63340474fe58119d0ef95bdc4b1f41df0aa689659b3d13b

COPY --from=build /opt/node_app/excalidraw-app/build /usr/share/nginx/html

HEALTHCHECK CMD wget -q -O /dev/null http://localhost || exit 1
```

### `docker-compose.yml`

```yaml
services:
  excalidraw:
    build:
      context: .
      args:
        - NODE_ENV=development
    container_name: excalidraw
    ports:
      - "3000:80"
    restart: on-failure
    stdin_open: true
    healthcheck:
      disable: true
    environment:
      - NODE_ENV=development
    volumes:
      - ./:/opt/node_app/app:delegated
      - ./package.json:/opt/node_app/package.json
      - ./yarn.lock:/opt/node_app/yarn.lock
      - notused:/opt/node_app/app/node_modules

volumes:
  notused:
```

### `.codesandbox/Dockerfile`

```dockerfile
FROM node:24-bullseye

# Vite wants to open the browser using `open`, so we
# need to install those utils.
RUN apt update -y && apt install -y xdg-utils
```

### `examples/with-script-in-browser/.codesandbox/Dockerfile`

```dockerfile
FROM node:18-bullseye

# Vite wants to open the browser using `open`, so we
# need to install those utils.
RUN apt update -y && apt install -y xdg-utils
```

### `tsconfig.json`

```json
{
  "compilerOptions": {
    "rootDir": "./",
    "target": "ESNext",
    "lib": ["dom", "dom.iterable", "esnext"],
    "types": ["vitest/globals", "@testing-library/jest-dom"],
    "allowJs": true,
    "skipLibCheck": true,
    "esModuleInterop": true,
    "allowSyntheticDefaultImports": true,
    "strict": true,
    "forceConsistentCasingInFileNames": true,
    "noFallthroughCasesInSwitch": true,
    "module": "ESNext",
    "moduleResolution": "bundler",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",
    "baseUrl": ".",
    "paths": {
      "@excalidraw/common": ["./packages/common/src/index.ts"],
      "@excalidraw/common/*": ["./packages/common/src/*"],
      "@excalidraw/excalidraw": ["./packages/excalidraw/index.tsx"],
      "@excalidraw/excalidraw/*": ["./packages/excalidraw/*"],
      "@excalidraw/element": ["./packages/element/src/index.ts"],
      "@excalidraw/element/*": ["./packages/element/src/*"],
      "@excalidraw/fractional-indexing": ["./packages/fractional-indexing/src/index.ts"],
      "@excalidraw/fractional-indexing/*": ["./packages/fractional-indexing/src/*"],
      "@excalidraw/laser-pointer": ["./packages/laser-pointer/src/index.ts"],
      "@excalidraw/laser-pointer/*": ["./packages/laser-pointer/src/*"],
      "@excalidraw/math": ["./packages/math/src/index.ts"],
      "@excalidraw/math/*": ["./packages/math/src/*"],
      "@excalidraw/utils": ["./packages/utils/src/index.ts"],
      "@excalidraw/utils/*": ["./packages/utils/src/*"]
    }
  },
  "include": ["packages", "excalidraw-app"],
  "exclude": ["examples", "dist", "types", "tests"]
}
```

### `.github/workflows/autorelease-excalidraw.yml`

```yaml
name: Auto release excalidraw next
on:
  push:
    branches:
      - release

jobs:
  Auto-release-excalidraw-next:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # v4
        with:
          fetch-depth: 2
      - name: Setup Node.js
        uses: actions/setup-node@49933ea5288caeca8642d1e84afbd3f7d6820020 # v4
        with:
          node-version: 20.x
      - name: Set up publish access
        run: |
          npm config set //registry.npmjs.org/:_authToken ${NPM_TOKEN}
        env:
          NPM_TOKEN: ${{ secrets.NPM_TOKEN }}
      - name: Auto release
        run: |
          yarn add @actions/core -W
          yarn release --tag=next --non-interactive
```

### `.github/workflows/build-docker.yml`

```yaml
name: Build Docker image

on:
  push:
    branches:
      - release

jobs:
  build-docker:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # v4
      - run: docker build -t excalidraw .
```

### `.github/workflows/cancel.yml`

```yaml
name: Cancel previous runs

on:
  push:
    branches:
      - release
  pull_request:

jobs:
  cancel:
    runs-on: ubuntu-latest
    timeout-minutes: 3
    steps:
      - uses: styfle/cancel-workflow-action@ce177499ccf9fd2aded3b0426c97e5434c2e8a73 # 0.6.0
        with:
          workflow_id: 400555, 400556, 905313, 1451724, 1710116, 3185001, 3438604
          access_token: ${{ secrets.GITHUB_TOKEN }}
```

## Source index (cite against these)

- `package.json`
- `dev-docs/package.json`
- `excalidraw-app/package.json`
- `CONTRIBUTING.md`
- `dev-docs/docs/introduction/contributing.mdx`
- `README.md`
- `dev-docs/README.md`
- `examples/with-nextjs/README.md`
- `Dockerfile`
- `docker-compose.yml`
- `.codesandbox/Dockerfile`
- `examples/with-script-in-browser/.codesandbox/Dockerfile`
- `tsconfig.json`
- `.github/workflows/autorelease-excalidraw.yml`
- `.github/workflows/build-docker.yml`
- `.github/workflows/cancel.yml`

