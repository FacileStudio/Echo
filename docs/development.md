# Echo — Development

Building and running Echo locally, the quality gate, and the things that only bite you because
this is a fork.

## Prerequisites

- **Node >= 24 and npm >= 11** per `package.json` `engines`. Note the Docker build uses
  `node:20-bookworm` regardless, so a change that only works on Node 24 will fail in CI-free
  silence at image build time.
- **GNU make** — the build is orchestrated by the `Makefile`, not by npm scripts.
- **Docker and Docker Compose** for anything involving a real conference. The web client alone
  cannot host a call; it needs Prosody, Jicofo and the JVB.

## Install

```sh
npm ci --legacy-peer-deps
```

The `--legacy-peer-deps` flag is not optional — the dependency tree does not resolve without it,
which is why the Dockerfile uses it too. `postinstall` runs `patch-package --error-on-fail`,
`jetify` and `npm run android-autolinking`. The last two are React Native leftovers; the native
code is gone but the dependencies and this hook are not.

## Build

| Command | What it does |
|---|---|
| `make dev` | webpack-dev-server on `https://localhost:8080`, self-signed cert |
| `make compile` | Clean, then production webpack bundles |
| `make all` | `compile` then `deploy` — what the Dockerfile runs |
| `make clean` | Remove build output |
| `make deploy` | Copy bundles and vendored assets into `libs/` |
| `make source-package` | `compile`, `deploy`, then tar the result |

`deploy` is a fan-out of a dozen sub-targets that stage `lib-jitsi-meet`, olm, the TensorFlow
WASM blobs, the face-landmarks and segmentation models, rnnoise, Excalidraw and the compiled
CSS. `libs/` is generated and gitignored — never edit anything in it.

The dev server proxies unbuilt paths to `WEBPACK_DEV_SERVER_PROXY_TARGET`, default
`https://localhost:8443`. Point it at a running Compose stack (or at the deployed instance) to
develop the client against a real backend. Browser certificate warnings on `localhost:8080` are
expected.

## Quality gate

```sh
npm run lint      # eslint --max-warnings 0 . && tsc --noEmit
npm run lint:ci   # eslint only
npm run lint-fix  # eslint --fix
npm run tsc:web   # tsc --noEmit --project tsconfig.web.json
npm run lint:lang # jsonlint every lang/*.json
npm run lang-sort # sort the lang files
```

Zero warnings is the bar. TypeScript strict mode is on.

**`npm run tsc:native` and `npm run tsc:ci` are broken in this fork.** They reference
`tsconfig.native.json`, which was deleted along with the native code; only `tsconfig.web.json`
remains. Use `npm run tsc:web`, or plain `npm run lint`, which runs `tsc --noEmit` without a
project flag.

## Tests

WebDriverIO end-to-end specs under `tests/`, split into `specs/` and `pageobjects/`. They need
a running deployment — `BASE_URL` defaults to `https://localhost:8080/`.

```sh
npm test                         # Chrome
npm run test-single -- <spec>    # one spec
npm run test-ff                  # Firefox
npm run test-dev                 # against make dev
npm run test-grid                # against a Selenium grid
```

Configuration is read from `tests/.env` via `DOTENV_CONFIG_PATH`. Useful switches:
`HEADLESS=true`, `ALLOW_INSECURE_CERTS=true` (needed for the self-signed dev cert),
`MAX_INSTANCES`, `VIDEO_CAPTURE_FILE` for a fake camera feed.

There are no unit tests and no CI — all upstream GitHub workflows were removed when the fork
was made.

## Working with the fork

The upstream fork point is commit `1814e37e2` (2026-05-21). To see everything Facile changed:

```sh
git diff --stat 1814e37e2..HEAD
git log --oneline 1814e37e2..HEAD
```

That is the fastest way to answer "is this ours or Jitsi's?" before touching a file.

Pulling upstream changes is a merge against a tree that deleted `react-native-sdk/`, `twa/`,
`.github/` and every native entry point, and rewrote `tokens.json` and ~120 icons wholesale.
Expect conflicts concentrated in exactly those places. Nothing in the repo automates it.

### Feature module conventions

Every feature under `react/features/<name>/` follows the upstream layout: `actionTypes.ts`,
`actions.ts`, `reducer.ts`, `middleware.ts`, `functions.ts`, `constants.ts` and `components/`.
Reducers and middleware self-register through `ReducerRegistry` and `MiddlewareRegistry`; there
is no central store file to edit. New features must be TypeScript, and there are no `index`
files — import directly from the source file.

`react/features/ai-summary/` is the only Facile-authored feature and is a good template: it
registers its reducer in `react/features/app/reducers.web.ts`, its middleware in
`middlewares.web.ts`, its state key in `app/types.ts`, and its toolbar button in
`toolbox/constants.ts` and `toolbox/hooks.web.ts`.

### Platform suffixes

The `.web.ts` / `.native.ts` / `.any.ts` file-suffix convention is still all over the tree even
though the native build is gone. Write `.web.tsx` for new UI. Do not add `.native.*` files —
nothing builds them.

### Styling

SCSS in `css/` mirrors `react/features`. Colors come from
`react/features/base/ui/tokens.json`, which Facile replaced entirely with a monochrome light
palette; do not hardcode hex values in components. The Goga typeface is declared in
`fonts.html` with ten weights and referenced from the tokens' `fontFamily` values.

Bundle size limits are enforced in production builds, so a new dependency has to earn its place.
`ANALYZE_BUNDLE=true npx webpack` shows what is in there.

### The AI summary sidecar

`ai-proxy.ts` is mounted read-only into a stock `oven/bun:latest` container by
`docker-compose.yml` and run with `bun run /app/index.ts`. There is no build step and no
`package.json` for it — it uses only `Bun.serve` and `fetch`. Editing the file and restarting
`echo-ai-proxy` is the whole deploy cycle.

To exercise it locally without Compose:

```sh
ANTHROPIC_API_KEY=sk-... bun run ai-proxy.ts
```

Then point `config.js`'s `ai.proxyUrl` at `http://localhost:3100`.

## The `doc/` directory

`doc/` (singular) is not documentation despite the name. It is upstream Jitsi packaging payload:
`doc/debian/` and `doc/jaas/` are referenced by exact path from `debian/*.install` and
`debian/*.docs`, so moving or renaming them breaks the Debian package build. Facile's
documentation lives in `docs/` (plural).
