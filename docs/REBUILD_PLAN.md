# Echo Rebuild Plan (`v2`) — Go + SvelteKit + LiveKit

Approved 2026-08-24. This document is the cold-start handoff: a fresh reader needs no prior
conversation to execute it.

## Goal

Replace the Jitsi fork with an owned Echo: tronc/porte Go API, SvelteKit 5 + muse client,
LiveKit as deployed media infrastructure — persistent rooms, recording, French transcription,
AI summaries, call history.

## Why

The fork was 397MB / 1,381 files of React+Redux+React Native in a stack zero other suite apps
use, with permanent upstream merge debt. Decision: rebuild on branch `v2`, keep the Jitsi
production deployment running untouched until cutover, then flip DNS and tag `jitsi-final`.

## Approach

Media plane = self-hosted `livekit-server` (+ coturn TURN, Redis from day one so the
multi-node path stays open; each room still pins to one node). App layer = tronc chassis +
porte (`oidc` + `local` kits — matches "SSO + password fallback" exactly). Guests join without
accounts by trading a display name for a scoped LiveKit token. LiveKit webhooks drive call
records, which anchor transcripts, summaries, history, and enveloppe events to Nook.

Checked against: tronc (chassis/migrations/testdb/OpenAPI), porte (v0.2 floor, CSRF rule,
adoption bug page), muse, module-path, events/enveloppe, filet, HARMONIZATION §6.

## Decisions locked

1. Auth: porte SSO + local password fallback + guest access without login
2. Persistent named rooms; Agenda integration planned (separate repo change)
3. Recording at v1: local disk + Nuage opt-in upload
4. Transcription at v1: port the Vosk kaldi-fr pipeline
5. AI summary panel kept, tied to per-call history for logged-in users
6. Scale ambition is high: Redis in compose day one; single node to start
7. Capacitor mobile shell phase 2; v1 web-only but platform-aware client logic
8. Same repo, branch `v2`, contents nuked, name kept
9. Old Jitsi keeps serving prod until parity point, then DNS flip

## Steps

### Phase 0 — scaffold ✅ (this commit)

1. Branch `v2` off main; delete all fork content. `[module-path]`
2. `apps/api/` — Go module, tronc main.go, mise.toml, scripts/check.sh, Dockerfile, filet.yml
3. `apps/client/` — SvelteKit 5, runes mode forced, adapter-node, muse-ready

### Phase 1 — media spike *(exit criterion gates everything after)*

4. `deploy/compose/`: livekit-server, coturn, redis; TLS via Dokploy/Traefik; open UDP
   50000–60000 on la ruche firewall.
5. `apps/api/modules/media/` — mint LiveKit AccessToken via `server-sdk-go/v2`; stub
   `POST /api/rooms/{slug}/token`.
6. Client throwaway `/join/[slug]`: connect two browsers through TURN. **Exit: audio/video
   flows end-to-end on la ruche.**

### Phase 2 — real app

7. Migrations: `rooms` (slug, name, owner_id nullable), `calls`, `transcripts`, `summaries`.
   `[migrations]`
8. porte wiring: oidc + local kits, pg stores, `/auth/config`. `[auth/porte]`
9. `modules/rooms/` CRUD; owner = creator when logged in; unowned rooms allowed.
10. Token endpoint for real: logged-in → moderator grants; guest → display-name +
    publish/subscribe grants only.
11. Client: home (create/join by slug), lobby, grid, controls bar, data-channel chat. `[muse]`

### Phase 3 — recording + transcription (parallel tracks)

12. Track A: livekit/egress service; `POST /api/rooms/{slug}/record/start|stop`
    (RoomComposite → MP4 on local disk volume).
13. Track A: Nuage opt-in per room; egress completion webhook triggers upload, link stored
    on `calls`.
14. Track B: transcriber service. **Spike first**: LiveKit Agents (Python) + custom Vosk STT
    plugin vs Go+pion bot with opus decode. Default: Python agents + existing Vosk container.
15. Captions overlay + transcript view. `[muse]`

### Phase 4 — history + AI summary

16. `modules/webhooks/` — WebhookReceiver (HMAC) → upsert `calls` and participants.
17. History pages (logged-in): calls per room, transcript viewer, recording links. `[auth/porte]`
18. `modules/summarize/` — port old ai-proxy.ts (Bun→Anthropic proxy): one Claude call over
    the transcript, stored, shown post-call. Key via Casier.

### Phase 5 — interconnect

19. Emit `call.started/ended`, `participant.joined` envelopes → Nook via pool. `[events]`
20. Agenda integration (separate mini-plan in the Agenda repo): Echo room links on events.

### Phase 6 — cutover

21. Deploy alongside Jitsi (e.g. `echo-v2.facile.studio`), dogfood ≥1 week.
22. Flip `echo.facile.studio` DNS; tag fork's final state `jitsi-final`; archive.

## Exit criteria

Two browsers join a named room by URL; guest + logged-in flows both work; mute/camera/
screenshare/chat functional; recording lands MP4 on disk (Nuage opt-in verified); live French
captions render and transcripts persist; logged-in user sees history with transcript + AI
summary; `call.*` events visible in Nook; `filet check` clean; Jitsi prod undisturbed until
the deliberate flip.

## Risks / unknown unknowns

- TURN behind Traefik/Dokploy: UDP range + TURN TLS is the classic first wall — that is why
  the spike exists.
- Egress is heavyweight (headless Chrome per recording); watch RAM on la ruche.
- Transcriber architecture (Python agents vs Go/pion) is the one genuinely open choice —
  spike before committing.
- Single node pins each room to one box; Redis included so horizontal growth is config.
- registre (in-house IdP) replacing Authentik later must not touch Echo if we stay on
  porte's contract — monitor registre progress.
- Anthropic dependency degrades gracefully: summary 503s, transcript remains useful.

## Skip (YAGNI)

Capacitor/mobile (phase 2), whiteboard, breakouts, polls, reactions, multi-node deployment,
scheduled-meetings engine inside Echo (Agenda owns scheduling), chat persistence,
SSO_ONLY enforcement, any LiveKit internals patching — ever.
