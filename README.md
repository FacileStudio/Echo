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
| API | Go 1.25, chi via tronc, porte (OIDC SSO + local passwords), PostgreSQL |
| Client | SvelteKit 5 (runes mode), muse tokens/components, `livekit-client` |
| Media | livekit-server, coturn (TURN), Redis (multi-node ready) |
| Recording | livekit/egress → local disk, Nuage opt-in upload |
| Transcription | Vosk (kaldi-fr) behind a transcriber service |
| Quality gate | `mise run check` (gofmt/vet/test + svelte-check) and filet |

## Development

```sh
mise install
mise run dev        # API
cd apps/client && bun run dev   # client
mise run check      # full quality gate
```

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

The transcriber worker reads `LIVEKIT_URL`, `LIVEKIT_API_KEY`, `LIVEKIT_API_SECRET`,
`VOSK_MODEL_PATH`, plus `ECHO_API_URL` and `TRANSCRIBER_TOKEN`. If either of the last two is
unset it logs a warning at startup and keeps broadcasting captions without persisting them.

Generate a token with:

```sh
openssl rand -hex 32
```

Set the same value on the `echo` service and on the `transcriber` service. The API refuses to
boot without it: an unauthenticated ingestion endpoint is worse than a failed start.

## Machine endpoints

Two routes are called by infrastructure rather than by a browser.

| Route | Caller | Auth |
|---|---|---|
| `POST /livekit/webhook` | livekit-server | LiveKit's signed JWT in `Authorization`, verified with the API secret |
| `POST /api/rooms/{slug}/transcript` | transcriber worker | `Authorization: Bearer $TRANSCRIBER_TOKEN` |

The webhook sits outside `/api` on purpose: it carries no cookie and no CSRF header, only the
signature. It upserts calls and participants and stamps the recording path when egress ends.

The transcript route takes `{"speaker": "...", "text": "..."}` and answers 204. It appends one
final utterance to the open call of that room, and answers 404 when no call is open, which the
transcriber treats as normal.

To make livekit-server call the webhook, its config needs a `webhook` block pointing at
`https://echo-v2.facile.studio/livekit/webhook`. See `deploy/compose/livekit.yaml`, and read the
comment at the top of that file: production supplies the config through the `LIVEKIT_CONFIG`
environment variable, not by mounting the file.

## Layout

```
apps/api/         Go API: modules/{rooms,media,webhooks,summarize}, migrations/
apps/client/      SvelteKit 5 client
deploy/           compose files for livekit-server, coturn, redis, egress, vosk
docs/             REBUILD_PLAN.md and design notes
```

## History

The previous Echo was a Jitsi Meet fork. It served well but drifted from the suite's stack and
carried permanent upstream merge debt. The fork's last state is tagged `jitsi-final`; this line
of development (`v2`) replaced it outright. See `docs/REBUILD_PLAN.md` for the reasoning.
