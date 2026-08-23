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
