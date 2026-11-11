# Echo — Video Conferencing

Suite app, **Go family** (forked pattern from GoSvelteBoilerplate / Journal): Go API on tronc +
SvelteKit 5 client, no `packages/`. Module path: `github.com/FacileStudio/Echo/apps/api`.

Media runs on a self-hosted LiveKit server deployed as infrastructure (compose containers:
livekit-server, coturn, redis, egress). **Never patch LiveKit internals** — it is a
dependency like Postgres, not a fork. The Jitsi fork this repo used to be is tagged
`jitsi-final`; do not resurrect its code.

## Auth model (decided 2026-08-24)

- porte with OIDC SSO only (`porte/oidc`): sign-in goes through Registre, there is no password route. Name and photo come from the Registre claims
- Guest join without an account: display name → scoped LiveKit access token minted by the API;
  guests never touch porte tables
- Logged-in room owner gets moderator grants; guests get publish/subscribe only
- Read `~/Projects/Facile/Code/porte/SPEC.md` before touching auth, and
  `~/.mycelium/memory/bugs/porte-adoption-null-oidc-subject.md` before any adoption work

## Design exception

Echo keeps its own design (Noah's redesign) and is exempt from the muse and no-borders
harmonisation. Do not bump muse, swap in muse AuthScreen or SignInWithFacile, restyle tiles or
remove borders (the speaking tile keeps its border). Changes here are functional or text only.

## Domain rules

- Rooms are persistent and named (slug); calls are recorded per session
- Recordings land on local disk; Nuage upload is an opt-in per room
- Transcripts and AI summaries belong to a call and are visible to its room owner and to the signed-in people who sat in it (`call_participants.user_id`); a recording is the owner's until they share it per recording
- History is scoped by the caller's account in the query (`modules/history`); never accept a user id from the request
- Transcription is on-device (vosk-browser in each consenting browser); only FINAL utterances are
  persisted, over `POST /api/rooms/{slug}/transcript`, and the speaker is the identity the server
  proved (session or seat key), never a body field. Recording is audio-only in the organizer's
  browser, mixing only people who accepted. Nothing is captured without a visible banner and a yes
- livekit-server posts room and egress events to `POST /livekit/webhook`, which sits outside
  `/api` because it authenticates with LiveKit's signed JWT, not a cookie
- AI summaries degrade: no `ANTHROPIC_API_KEY` means the summary endpoint returns 503 and the
  transcript stays useful on its own
- Other suite apps run calls through `/api/integrations/*` (per-app bearer secret from
  `ECHO_INTEGRATION_KEYS`) and frame `/embed/{room}`; integration rooms refuse the public guest
  token route. Contract and decisions: `docs/integrations.md`
- Room events flow to Nook via enveloppe (`call.started`, `call.ended`,
  `participant.joined`), keyed on `actor_email`

## Conventions that apply here

- tronc chassis: error envelope, `/health`+`/ready`, OpenAPI at `/docs`, goose migrations
  embedded in `apps/api/schemas` or `modules/*/migrations` following Journal's shape
- filet gate must stay clean; `requiredFiles` demands `router.go` in every HTTP module
- muse tokens for all UI color/spacing; Svelte 5 runes everywhere (vite config forces runes)
- Capacitor mobile shell is phase 2 — keep client logic platform-aware but web-only for now

## Commands

| Task | Command |
|---|---|
| Dev stack (docker services, API, client) | `mise run dev` |
| Stop dev services | `mise run dev:down` |
| Quality gate | `mise run check` |
| Go-only gate | `mise run check-go` |
| Format Go | `mise run format` |
| Install the pre-push hook (`scripts/check.sh`, `filet gate`) | `mise run hooks` |

Deploy target: Dokploy compose `Echo v2` on gare.facile.studio, source `deploy/compose/app.yml`, autoDeploy from `master`, served at echo.facile.studio. LiveKit runs in the separate `Echo Media` compose (echo-live.facile.studio). Jitsi was removed on 2026-11-02; its last version is tagged `jitsi-final`.
