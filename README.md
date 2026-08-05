# Echo

Self-hosted video conferencing for the Facile Suite, forked from Jitsi Meet and reduced to the
web client.

Everything upstream Jitsi does — WebRTC calls, screen sharing, chat, breakout rooms,
end-to-end encryption — still works, and is documented in the
[Jitsi handbook](https://jitsi.github.io/handbook/). This README covers what **Facile** changed:
the branding, the French default, the self-hosted French transcription stack, and an AI meeting
summary feature that upstream does not have.

Live at [echo.facile.studio](https://echo.facile.studio).

## What it does

- Runs the full Jitsi conferencing stack — Prosody, Jicofo and the video bridge — from one
  `docker compose up`
- Serves a Facile-branded web client: monochrome light theme, Goga typeface, Solar icon set
- Defaults to French, with French and English bundled into the app shell rather than fetched
- Transcribes meetings through Jigasi and a self-hosted Vosk `kaldi-fr` container, so audio
  never reaches a transcription vendor
- Generates a structured meeting summary — topics, decisions, action items, per-speaker notes —
  from the live transcript, through a sidecar that holds the Anthropic API key
- Lets participants download the raw transcript as Markdown from the summary panel
- Ships as its own nginx image built from source rather than the stock `jitsi/web` container

## Stack

| Layer | Tech |
|---|---|
| Client | React 19, TypeScript, Redux, Webpack 5, SCSS, `lib-jitsi-meet` |
| Runtime | Node >= 24 and npm >= 11 to build (the Docker build pins Node 20), nginx alpine to serve |
| Deploy | Docker Compose, seven services, behind Traefik on Dokploy |

## Quick start

```sh
cp .env.example .env
docker compose up -d
```

`.env.example` is incomplete: it omits `JIGASI_XMPP_PASSWORD` and `ANTHROPIC_API_KEY`, both of
which `docker-compose.yml` reads. Fill those in from
[docs/configuration.md](docs/configuration.md) before starting.

### Local development

```sh
npm ci --legacy-peer-deps
make dev
```

Serves at `https://localhost:8080` with a self-signed certificate, proxying what it does not
build to `WEBPACK_DEV_SERVER_PROXY_TARGET` (default `https://localhost:8443`). Certificate
warnings are expected.

```sh
make compile   # production bundles
make all       # compile, then deploy into libs/
npm run lint   # ESLint plus tsc, must be clean
```

## Configuration

| Variable | What it does |
|---|---|
| `PUBLIC_URL` | Public origin. Rewrites `config.js` when the web container starts |
| `JVB_ADVERTISE_IPS` | Public IP the video bridge advertises for WebRTC media |
| `CONFIG` | Host directory holding the Prosody, Jicofo, JVB and Jigasi config volumes |
| `JICOFO_AUTH_PASSWORD` | Jicofo XMPP password |
| `JVB_AUTH_PASSWORD` | Video bridge XMPP password |
| `JIGASI_XMPP_PASSWORD` | Transcriber XMPP password. Required, missing from `.env.example` |
| `ANTHROPIC_API_KEY` | Key for the AI summary sidecar. Without it the proxy answers 503 |

Full reference: [docs/configuration.md](docs/configuration.md).

## Structure

```
react/features/  80+ Redux feature modules, including Facile's own ai-summary/
css/             SCSS mirroring react/features, plus _ai-summary.scss
lang/            i18n JSON — main.json (English) and main-fr.json (French, the default)
modules/         Legacy JS modules: API, devices, translation, UI
static/fonts/    The Goga typeface, declared by fonts.html
docker/          nginx.conf and entrypoint.sh for Echo's own image
doc/             Upstream Jitsi packaging assets: debian/, jaas/, examples/
docs/            This documentation set
tests/           WebDriverIO end-to-end specs
```

## Documentation

| Doc | What's in it |
|---|---|
| [Architecture](docs/architecture.md) | Service topology, the Facile delta, transcript and summary flow |
| [Configuration](docs/configuration.md) | Every environment variable, `config.js`, `interface_config.js` |
| [Development](docs/development.md) | Build targets, the lint gate, tests, working with the fork |
| [Deployment](docs/deployment.md) | The image, Compose, Dokploy and Traefik, ports, upgrades |
| [API](docs/api.md) | The AI summary proxy, the nginx routes, the iframe API |

`CONTRIBUTING.md` and `SECURITY.md` are upstream Jitsi's and describe Jitsi's processes, not
Facile's.

---

Part of the [Facile Suite](https://facile.studio) — self-hosted tools for creative studios
and freelancers. One login, zero cloud dependency.
