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
| Recording | In the organizer's browser (MediaRecorder), audio of consenting people only, uploaded in chunks |
| Transcription | On each person's device: Vosk (vosk-browser, WebAssembly), only text is sent |
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

The server cannot read encrypted media, so transcription and recording happen in browsers, and only
for people who agree.

### Transcription and recording

- The organizer (the signed-in room owner) turns transcription and/or recording on from the call
  controls. The API refuses without the owner's plan: `ai.access` for transcription,
  `echo.recording` for recording. Guest-created rooms and integration rooms have no organizer, so
  they have neither.
- Everyone in the call sees a persistent banner. A person is transcribed or recorded only after
  they press Accept; unanswered counts as no. Declining, or pressing Stop for me later, stops it at
  once. Switching a feature off and on again asks everyone again.
- Transcription: each accepting browser runs Vosk (WebAssembly) on its own microphone, on the
  device, and posts only final text lines. The model (about 41 MB) downloads after Accept, with a
  progress line, and stays in the browser cache. The server sets the speaker from the session or
  seat key, never from the body.
- Recording policy: the recording is audio only and mixes only the voices of people who accepted.
  Someone who declines, has not answered or just joined is never connected to the mix, so they are
  excluded rather than blocking the recording; the organizer sees how many are left out. Video is
  never recorded, because compositing video per person in a browser tab is heavy, throttled in
  background tabs, and would need a consent check per frame.
- Withdrawal reaches the organizer's recorder within one poll (3 seconds).
- Uploads are chunked and resumable; the file is finalized at the end of the call or after 15
  minutes without a chunk. A webm written by MediaRecorder has no duration header: it plays, but
  seeking is coarse.

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
| `ANTHROPIC_API_KEY` | no | empty | Enables AI summaries; without it the summary endpoint returns 503 |
| `ANTHROPIC_MODEL` | no | `claude-sonnet-5` | Model used for summaries |
| `RECORDINGS_DIR` | no | `/recordings` | Directory the recordings volume is mounted at. Browser recordings are written here, so it must be writable; empty disables recording |
| `ECHO_INTEGRATION_KEYS` | no | empty | `app:secret` pairs, comma-separated, for the integration API; empty refuses every app |
| `ECHO_EMBED_ORIGINS` | no | empty | Origins allowed to frame `/embed/{room}`, comma-separated; empty allows none |
| `MCP_RESOURCE` | no | empty | Canonical URI of the MCP endpoint, `https://echo.facile.studio/mcp` in production. Set with `OIDC_*` to serve `/mcp`; empty leaves it unmounted |
| `TRUSTED_PROXIES` | no | `private,cloudflare` | Hops whose `X-Forwarded-For` is believed: CIDR blocks, addresses, the names `private` and `cloudflare` (tronc's published Cloudflare edge ranges), or `none`. echo.facile.studio sits behind Cloudflare and Traefik. Production's Traefik replaces Cloudflare's `X-Forwarded-For` with the edge address alone, so the join rate limit reads `Cf-Connecting-Ip` only when the peer is private and the rightmost `X-Forwarded-For` hop is a Cloudflare edge; otherwise it keys on the address this setting resolves |

Plan limits (see [docs/plans.md](docs/plans.md)) come from Registre, and Echo syncs them only when
`OIDC_ISSUER`, `OIDC_CLIENT_ID` and `OIDC_CLIENT_SECRET` are set. Without them, every limit stays
open and the API logs a warning at boot. The production deployment has no `OIDC_*` variables yet,
so it does not enforce plans.

## Machine endpoints

These routes are called by infrastructure or other apps rather than by a browser.

| Route | Caller | Auth |
|---|---|---|
| `POST /livekit/webhook` | livekit-server | LiveKit's signed JWT in `Authorization`, verified with the API secret |
| `/api/integrations/rooms...` | suite apps (Prisme) | `Authorization: Bearer <app secret>` from `ECHO_INTEGRATION_KEYS` |

## Meetings API

Browser routes behind the session cookie. Each checks the caller's own account, never an id from the request.

| Route | Who |
|---|---|
| `GET /api/meetings?page=` | the calls the caller owns or joined, newest first |
| `GET /api/calls/{id}` | the room owner, or someone who sat in the call |
| `GET /api/calls/{id}/recordings` | owner: all; participant: only the shared ones |
| `GET /api/recordings/{id}/file[?download=1]` | same rule, `Range` supported |
| `PATCH /api/recordings/{id}` `{"shared": bool}` | owner only; `true` needs `echo.recording` |
| `GET /api/calls/{id}/transcript[?q=\|format=txt\|vtt]` | owner or participant; the owner's plan needs `ai.access` (else 402) |
| `POST /api/calls/{id}/summary` | owner only; needs `ai.access` |

The integration API, the `/embed/{room}` route and the signed call events it forwards are
documented in [docs/integrations.md](docs/integrations.md).

The webhook sits outside `/api` on purpose: it carries no cookie and no CSRF header, only the
signature. It upserts calls and participants and stamps the recording path when egress ends.

The transcript route takes `{"text": "...", "start_ms": 0, "end_ms": 0}` from a seated, consenting
participant and answers 204. It appends one final utterance to the open call of that room, and
answers 404 when no call is open.

To make livekit-server call the webhook, its config needs a `webhook` block pointing at
`https://echo.facile.studio/livekit/webhook`. See `deploy/compose/livekit.yaml`, and read the
comment at the top of that file: production supplies the config through the `LIVEKIT_CONFIG`
environment variable, not by mounting the file.

## MCP server

`POST /mcp` serves the Model Context Protocol to any MCP client (Claude, ChatGPT, Cursor, Bulle)
holding a Registre access token bound to `MCP_RESOURCE`, with scopes `echo.read` and
`echo.write`. `/.well-known/oauth-protected-resource/mcp` tells a client how to get one. The
first MCP call of a person who never opened Echo creates their account, as a first sign-in
would. MCP is free on every plan; transcripts and summaries still need `ai.access`.

| Tool | Does | Confirmed |
|---|---|---|
| `echo_list_rooms` | the caller's rooms, links without key | no |
| `echo_create_room` | a new room and its invite link with a fresh `#key=`, never stored | no |
| `echo_rename_room` | a room's display name | no |
| `echo_delete_room` | a room with every meeting in it | yes |
| `echo_list_meetings` | the meetings the caller owns or sat in, by cursor | no |
| `echo_get_meeting` | people, summary and recordings of one meeting | no |
| `echo_read_transcript` | transcript lines, filtered by `query`, by cursor | no |
| `echo_summarize_meeting` | writes the AI summary again (owner, `ai.access`) | no |
| `echo_share_recording` | shares a recording with the call's participants or takes it back | no |

A confirmed tool asks in the client when it can elicit (MRTR). Otherwise it returns a
`confirm_url` to `/pending`, where the signed-in person confirms or dismisses the request
(`GET /api/mcp/pending`, `POST /api/mcp/pending/{id}/confirm|dismiss`, session only). A room
renamed or used for a new call after the request cannot be confirmed.

## Layout

```
apps/api/         Go API: modules/{rooms,media,webhooks,summarize}, migrations/
apps/client/      SvelteKit 5 client
deploy/           compose files for livekit-server, coturn, redis, egress
docs/             REBUILD_PLAN.md, integrations.md and design notes
```

## History

The previous Echo was a Jitsi Meet fork. It served well but drifted from the suite's stack and
carried permanent upstream merge debt. The fork's last state is tagged `jitsi-final`; this line
of development (`v2`) replaced it outright. See `docs/REBUILD_PLAN.md` for the reasoning.

## License

Echo is licensed under the GNU Affero General Public License v3.0 only (`AGPL-3.0-only`); see [LICENSE](LICENSE). A commercial license is available from Facile Studio for use that cannot meet the AGPL's terms.
