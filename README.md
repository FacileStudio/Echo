# Echo

Video conferencing by Facile Studio. Join by URL, talk, share your screen — self-hosted,
one login, zero cloud dependency.

Echo is a suite app in the **Go family**: a Go API on [tronc](https://github.com/FacileStudio/tronc)
with authentication from [porte](https://github.com/FacileStudio/porte), and a SvelteKit 5
frontend built with [muse](https://github.com/FacileStudio/muse). Real-time media is routed by
a self-hosted [LiveKit](https://livekit.io) server deployed alongside the app as infrastructure
— the way PostgreSQL is infrastructure to other suite apps. Echo owns every layer where its
product decisions live; LiveKit owns the WebRTC plumbing nobody should hand-write.

## Stack

| Layer | Tech |
|---|---|
| API | Go 1.25, chi via tronc, porte (OIDC SSO), PostgreSQL |
| Client | SvelteKit 5 (runes mode), muse tokens/components, `livekit-client` |
| Media | livekit-server, coturn (TURN), Redis (multi-node ready) |
| Encryption | LiveKit end-to-end encryption, always on; the key stays in the link fragment |
| Recording | livekit/egress; off, because the server cannot decode encrypted calls |
| Transcription | Vosk transcriber in `services/transcriber`; not deployed, for the same reason |
| Quality gate | `mise run check` (gofmt/vet/test + svelte-check) and filet |

## End-to-end encryption

Every call is end-to-end encrypted with LiveKit's E2EE
(https://docs.livekit.io/transport/encryption/): audio, video, screen shares and chat. The room key
lives in the link, `/room/<slug>#key=<base64url>`. Browsers never send a fragment to a server, so
neither Echo nor LiveKit sees it.

- Opening a room link without a key generates 32 random bytes, writes them into the address bar
  and remembers them for that slug in `localStorage`, so the creator's next visits reuse them.
- Copy link copies the whole address, key included. Share that link, not the bare `/room/<slug>`.
- Someone with a different key sees "Can't decrypt this call" instead of the media.
- A browser that cannot encrypt media is told so and does not join.
- Integration rooms take the key from the app that opens them; see `docs/integrations.md`.

The server cannot read encrypted media, so these are off:

- Live captions: the transcriber service is no longer in `deploy/compose/app.yml`, so it joins no
  room. Its code stays in `services/transcriber`.
- Recording: the Record button is gone from the call. The egress container and the record routes
  stay, unused.
- Transcripts and AI summaries: new calls have none. The call page says why, and older calls keep
  what they already have.

## Development

```sh
mise install
mise run dev        # Postgres + LiveKit in docker, API on :8080, client on :5173
mise run dev:down   # stop the docker services
mise run check      # full quality gate
```

Local overrides such as `ANTHROPIC_API_KEY` or the `OIDC_*` set go in a gitignored `.env`
at the repo root, which mise loads.

## Configuration

The API reads these on top of the tronc core variables (`PORT`, `DATABASE_URL`, `LOG_LEVEL`,
`CORS_ALLOWED_ORIGINS`) and the porte OIDC set (`OIDC_*`, `SSO_ONLY`, `ALLOW_REGISTRATION`).

| Variable | Required | Default | What it does |
|---|---|---|---|
| `LIVEKIT_URL` | yes | n/a | LiveKit signalling URL the API mints tokens against |
| `LIVEKIT_API_KEY` | yes | n/a | LiveKit API key, also the key LiveKit signs webhooks with |
| `LIVEKIT_API_SECRET` | yes | n/a | LiveKit API secret; verifies the webhook signature |
| `TRANSCRIBER_TOKEN` | yes | n/a | Bearer token the transcriber presents to ingest transcripts |
| `ANTHROPIC_API_KEY` | no | empty | Enables AI summaries; without it the summary endpoint returns 503 |
| `ANTHROPIC_MODEL` | no | `claude-sonnet-5` | Model used for summaries |
| `RECORDINGS_DIR` | no | `/recordings` | Directory the recordings volume is mounted at, read-only |
| `ECHO_INTEGRATION_KEYS` | no | empty | `app:secret` pairs, comma-separated, for the integration API; empty refuses every app |
| `ECHO_EMBED_ORIGINS` | no | empty | Origins allowed to frame `/embed/{room}`, comma-separated; empty allows none |
| `TRUSTED_PROXIES` | no | `private,cloudflare` | Hops whose `X-Forwarded-For` is believed: CIDR blocks, addresses, the names `private` and `cloudflare` (tronc's published Cloudflare edge ranges), or `none`. echo.facile.studio sits behind Cloudflare and Traefik. Production's Traefik replaces Cloudflare's `X-Forwarded-For` with the edge address alone, so the join rate limit reads `Cf-Connecting-Ip` only when the peer is private and the rightmost `X-Forwarded-For` hop is a Cloudflare edge; otherwise it keys on the address this setting resolves |

Plan limits (see [docs/plans.md](docs/plans.md)) come from Registre, and Echo syncs them only when
`OIDC_ISSUER`, `OIDC_CLIENT_ID` and `OIDC_CLIENT_SECRET` are set. Without them, every limit stays
open and the API logs a warning at boot. The production deployment has no `OIDC_*` variables yet,
so it does not enforce plans.

The transcriber worker is not deployed (see End-to-end encryption). When run, it reads
`LIVEKIT_URL`, `LIVEKIT_API_KEY`, `LIVEKIT_API_SECRET`,
`VOSK_MODEL_PATH`, plus `ECHO_API_URL` and `TRANSCRIBER_TOKEN`. If either of the last two is
unset it logs a warning at startup and keeps broadcasting captions without persisting them.

Generate a token with:

```sh
openssl rand -hex 32
```

Set the same value on the `echo` service and on the `transcriber` service. The API refuses to
boot without it: an unauthenticated ingestion endpoint is worse than a failed start.

## Machine endpoints

These routes are called by infrastructure or other apps rather than by a browser.

| Route | Caller | Auth |
|---|---|---|
| `POST /livekit/webhook` | livekit-server | LiveKit's signed JWT in `Authorization`, verified with the API secret |
| `POST /api/rooms/{slug}/transcript` | transcriber worker | `Authorization: Bearer $TRANSCRIBER_TOKEN` |
| `/api/integrations/rooms...` | suite apps (Prisme) | `Authorization: Bearer <app secret>` from `ECHO_INTEGRATION_KEYS` |

The integration API, the `/embed/{room}` route and the signed call events it forwards are
documented in [docs/integrations.md](docs/integrations.md).

The webhook sits outside `/api` on purpose: it carries no cookie and no CSRF header, only the
signature. It upserts calls and participants and stamps the recording path when egress ends.

The transcript route takes `{"speaker": "...", "text": "..."}` and answers 204. It appends one
final utterance to the open call of that room, and answers 404 when no call is open, which the
transcriber treats as normal.

To make livekit-server call the webhook, its config needs a `webhook` block pointing at
`https://echo.facile.studio/livekit/webhook`. See `deploy/compose/livekit.yaml`, and read the
comment at the top of that file: production supplies the config through the `LIVEKIT_CONFIG`
environment variable, not by mounting the file.

## Layout

```
apps/api/         Go API: modules/{rooms,media,webhooks,summarize}, migrations/
apps/client/      SvelteKit 5 client
deploy/           compose files for livekit-server, coturn, redis, egress, vosk
docs/             REBUILD_PLAN.md, integrations.md and design notes
```

## History

The previous Echo was a Jitsi Meet fork. It served well but drifted from the suite's stack and
carried permanent upstream merge debt. The fork's last state is tagged `jitsi-final`; this line
of development (`v2`) replaced it outright. See `docs/REBUILD_PLAN.md` for the reasoning.
