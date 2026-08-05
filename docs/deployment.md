# Echo — Deployment

How Echo is built into an image, what the seven Compose services do, and how it sits behind
Traefik on la ruche.

## The image

`Dockerfile` at the repo root, two stages.

**Stage one — `node:20-bookworm`.** Copies `package.json` and `package-lock.json`, runs
`npm ci --legacy-peer-deps`, copies the rest of the source, runs `make all`, and writes the
short commit SHA to `/tmp/echo-version` (falling back to a Unix timestamp when git metadata is
unavailable, which it is in most build contexts).

**Stage two — `nginx:alpine`.** Removes the default site and `/usr/share/nginx/html`, installs
`docker/nginx.conf` and `docker/entrypoint.sh`, and assembles the app at `/srv/echo`:

| From | To | Source |
|---|---|---|
| `libs/` | `libs/` | stage one |
| `css/all.css` | `css/all.css` | stage one |
| `lang/`, `images/`, `static/`, `sounds/` | same | the repo |
| `index.html`, `interface_config.js`, `head.html`, `base.html`, `title.html`, `fonts.html`, `body.html`, `plugin.head.html`, `manifest.json`, `pwa-worker.js`, `favicon.ico` | same | the repo |
| `config.js` | `config.js.template` | the repo |

```sh
docker build -t echo-web .
```

Build from the repository root; the build context is the whole tree, filtered by
`.dockerignore`.

Echo deliberately does **not** use the stock `jitsi/web` image. That was dropped early in the
fork because overriding its entrypoint defaults for title, favicon and HTML fragments was
fighting the image rather than using it.

## Container start

`docker/entrypoint.sh` runs before nginx and does two substitutions, then `exec nginx -g
'daemon off;'`.

1. **Config generation.** `sed` rewrites the placeholders in `config.js.template` —
   `https://jitsi-meet.example.com` becomes `PUBLIC_URL`, `wss://jitsi-meet.example.com`
   becomes `wss://<host>`, and the bare `jitsi-meet.example.com` becomes `XMPP_DOMAIN` — into
   `/srv/echo/config.js`. `PUBLIC_URL` has any trailing slash stripped first, and defaults to
   `https://${XMPP_DOMAIN}` when unset.
2. **Cache busting.** `__ECHO_VERSION__` in `index.html` is replaced with the baked build
   version. Static assets are served with `Cache-Control: public, immutable` for a year, so
   without this a deploy would leave browsers on the previous bundle. It logs
   `Echo ready: domain=… url=… version=…` — the fastest way to confirm a container picked up
   new config.

## nginx routes

From `docker/nginx.conf`. SSI is enabled for JavaScript MIME types, which is how upstream's
`config.js` templating works.

| Location | Behavior |
|---|---|
| `~* \.(js\|css\|wasm\|otf\|ttf\|woff2?\|png\|jpg\|svg\|ico\|json\|mp3\|ogg\|wav)$` | `expires 1y`, `Cache-Control: public, immutable`, `try_files … =404` |
| `/http-bind` | Proxy to `echo-prosody:5280/http-bind` — BOSH |
| `/xmpp-websocket` | Proxy to `echo-prosody:5280/xmpp-websocket`, upgraded, 900 s read timeout |
| `/colibri-ws/*` | Proxy to `echo-jvb:9090`, upgraded, 900 s read timeout |
| `/ai/summarize` | Proxy to `echo-ai-proxy:3100`, 120 s read timeout |
| `= /config.js` | SSI on, `application/javascript`, `expires 1h` |
| `= /interface_config.js` | `application/javascript`, `expires 1h` |
| `/` | `try_files $uri $uri/ /index.html` — any path is a room name |

The AI proxy route uses `resolver 127.0.0.11 valid=30s ipv6=off` with the upstream in a
variable rather than a literal `proxy_pass` target. That is deliberate: nginx resolves literal
upstreams once at startup and refuses to start if the name does not resolve, so a down or
not-yet-created `echo-ai-proxy` would take the entire web container with it. The variable form
resolves per request.

`gzip` covers text, JSON, JavaScript, XML, WASM and OTF, with a custom `types` block adding
`font/otf` — the Goga files would otherwise be served as `application/octet-stream`.

## Compose topology

`docker-compose.yml` defines seven services on an internal `echo` network. Only `echo-web` also
joins the external `dokploy-network`, which is how Traefik reaches it.

| Service | Image | Exposure | Role |
|---|---|---|---|
| `echo-web` | built here | `expose: 80` | nginx, the SPA and every proxy route |
| `echo-prosody` | `jitsi/prosody` | `expose: 5222, 5269, 5347, 5280` | XMPP. Network alias `${XMPP_SERVER}` |
| `echo-jicofo` | `jitsi/jicofo` | internal | Conference focus, allocates bridges |
| `echo-jvb` | `jitsi/jvb` | **`ports: 10000/udp`** | Video bridge — the only published port |
| `echo-jigasi` | `jitsi/jigasi` | internal | Transcriber, `JIGASI_MODE=transcriber` |
| `echo-vosk` | `alphacep/kaldi-fr` | `expose: 2700` | French speech recognition |
| `echo-ai-proxy` | `oven/bun:latest` | `expose: 3100` | Runs `ai-proxy.ts`, mounted read-only |

`echo-web` depends on `echo-jvb` and `echo-ai-proxy`; Jicofo, the JVB and Jigasi depend on
Prosody; Jigasi also depends on Vosk. All use `restart: unless-stopped`.

`${CONFIG}` is bind-mounted into Prosody (`/config` plus
`/prosody-plugins-custom`), Jicofo, the JVB and Jigasi. That directory holds generated
configuration and **must persist across deploys** — losing it means losing the Prosody user
accounts.

## Dokploy and Traefik

Echo runs on la ruche through Dokploy, with Traefik routing `echo.facile.studio` to
`echo-web:80` over `dokploy-network`.

**Echo cannot satisfy the suite's one-container rule.** WebRTC media does not travel over
HTTP and cannot pass through Traefik: the JVB publishes UDP `10000` straight to the host, and
that port must be open on the firewall or calls connect and then show black video. Everything
*else* does follow the one-router / one-hostname rule — a single hostname, a single HTTP router,
one nginx as the only web entry point.

`PUBLIC_URL` must exactly match the hostname Traefik serves. It ends up in the generated
`config.js` as both the HTTP origin and the derived `wss://` URL, so a mismatch produces a page
that loads and then fails to connect to XMPP.

`JVB_ADVERTISE_IPS` must be la ruche's public IP. The bridge advertises it in ICE candidates;
an unset or wrong value is the classic "everyone joins, nobody sees anybody" failure.

## Health checking

There is no `/health` endpoint. A green HTTP response from `/` only proves nginx is serving
`index.html` — it says nothing about whether Prosody accepted the XMPP connection or whether
media flows. Verify a deploy by actually joining a room from two browsers, and check
`docker logs echo-web` for the `Echo ready:` line to confirm the right `PUBLIC_URL` and version
were baked in.

Useful checks:

```sh
docker compose logs -f echo-prosody   # XMPP auth and MUC failures
docker compose logs -f echo-jvb       # ICE and media
docker compose logs -f echo-jigasi    # transcriber joining, Vosk connection
docker compose logs -f echo-ai-proxy  # "AI proxy listening on :3100", or the missing-key warning
```

## Upgrading

The four Jitsi images track `${JITSI_IMAGE_VERSION}`, default `stable`. Bumping it upgrades
Prosody, Jicofo, the JVB and Jigasi together — they are version-coupled and must not diverge.
The web client is independent: it is built from this repository and only changes when you
rebuild the image.

That decoupling is a real risk. The client bundles `lib-jitsi-meet` from the fork's
`package.json`, so a large jump in the server images without a corresponding upstream merge can
put the client and the bridge on incompatible protocol versions.

## Data and secrets

Nothing persists except `${CONFIG}`. There is no database, no object storage, no recording
volume — `ENABLE_RECORDING=0` and there is no Jibri service, and
`JIGASI_TRANSCRIBER_RECORD_AUDIO=0` means no audio is written anywhere.

`ANTHROPIC_API_KEY` exists only in `echo-ai-proxy`'s environment and never reaches the browser
— that is the entire reason the sidecar exists rather than calling Anthropic from the client.
