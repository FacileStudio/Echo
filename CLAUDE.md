# Echo — Video Conferencing

Suite app, **Go family** (forked pattern from GoSvelteBoilerplate / Journal): Go API on tronc +
SvelteKit 5 client, no `packages/`. Module path: `github.com/FacileStudio/Echo/apps/api`.

Media runs on a self-hosted LiveKit server deployed as infrastructure (compose containers:
livekit-server, coturn, redis, egress, vosk). **Never patch LiveKit internals** — it is a
dependency like Postgres, not a fork. The Jitsi fork this repo used to be is tagged
`jitsi-final`; do not resurrect its code.

## Auth model (decided 2026-08-24)

- porte with OIDC SSO **and** local password fallback (`porte/oidc` + `porte/local`)
- Guest join without an account: display name → scoped LiveKit access token minted by the API;
  guests never touch porte tables
- Logged-in room owner gets moderator grants; guests get publish/subscribe only
- Read `~/Projects/Facile/Code/porte/SPEC.md` before touching auth, and
  `~/.mycelium/memory/bugs/porte-adoption-null-oidc-subject.md` before any adoption work

## Domain rules

- Rooms are persistent and named (slug); calls are recorded per session
- Recordings land on local disk; Nuage upload is an opt-in per room
- Transcripts and AI summaries belong to a call and are visible to logged-in users only
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
| Dev API | `mise run dev` |
| Dev client | `cd apps/client && bun run dev` |
| Quality gate | `mise run check` |
| Go-only gate | `mise run check-go` |
| Format Go | `mise run format` |

Deploy target: Dokploy on la ruche, alongside the still-running Jitsi deployment until cutover.
