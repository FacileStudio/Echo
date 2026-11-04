# Changelog

## [Unreleased]

### Added
- Plan limits from Registre (porte/forfait), read for the room owner's Registre subject: `echo.max_participants` (free: 4), `echo.max_call_minutes` (free: 40) and `echo.recording` (free: off). A room with no owner, or whose owner never signed in through Registre, gets the free limits. Integration rooms are unlimited.
- A full call refuses a new participant with HTTP 402 (`payment_required`, with `key`, `limit` and `upgrade_url`), and the join page offers the upgrade link. The count is LiveKit's participant list, falling back to call rows, plus tokens admitted in the last 60 seconds that have not connected yet. That count never refuses the signed-in owner; a limit of 0 refuses everyone with 402.
- `POST /api/rooms` and `POST /api/rooms/{slug}/token` are rate limited: 10 a minute per address for guests (an IPv6 caller per /64), 30 per account for signed-in users (429 above that).
- `TRUSTED_PROXIES` (default `private,cloudflare`) names the hops whose `X-Forwarded-For` is believed, so behind Cloudflare the client address is the visitor, not the Cloudflare edge.
- A call on a limited plan ends at `started_at` plus the limit. The token response carries `call_deadline`; the call shows a countdown and warns five minutes before the end. The server closes the LiveKit room itself and re-arms every deadline from the database at boot.
- Starting a recording answers 402 unless the owner's plan includes recording.
- Free calls last 40 minutes per meeting. A room cut at its deadline cannot start a new call for 2 minutes. See `docs/plans.md`.
- Echo creates every LiveKit room itself with the plan's ceiling, and answers 503 without minting a token when it cannot, and LiveKit's `room.auto_create` is off (`deploy/compose/livekit.yaml`; the production `LIVEKIT_CONFIG` needs the same block). Integration tokens create their room too.
- Every 5 minutes Echo compares open calls with LiveKit's rooms. It closes call rows LiveKit no longer runs and enforces deadlines for sessions no webhook reported.
- Echo pulls Registre's directory snapshot with its OIDC client credentials (every 5 minutes, and on `POST /api/registre/sync`) and keeps the last entitlements in the `forfait_state` table, so a restart does not fail open again. Without `OIDC_ISSUER`, or before Registre ever pushed entitlements, every limit is open. If the stored row cannot be read, Echo enforces the free plan until the next snapshot.

### Changed
- tronc v0.10.1 to v0.11.0, for its named `cloudflare` trusted-proxy set.
- Join tokens for signed-in users and guests live 10 minutes instead of 6 hours. LiveKit refreshes the token of a connected client by itself.

## [2.2.0] - 2026-11-03

### Added
- Every call is end-to-end encrypted with LiveKit E2EE: audio, video, screen shares and chat. The room key lives in the link fragment (`#key=`), which no server sees. A room link opened without a key gets a fresh one, remembered per room in the browser.
- A lock in the call controls shows the call is encrypted. A participant with the wrong key sees "Can't decrypt this call" instead of a broken call, and a browser that cannot encrypt is refused with an explanation.
- `/embed/{room}` requires a `key` in its fragment. The integrating app owns key distribution; see `docs/integrations.md`.

### Fixed
- Screen sharing works: a shared screen shows as a large, uncropped tile for everyone, the sharer included. Before, it was published but never rendered.
- Cancelling the screen picker no longer throws, and stopping from the browser's own "Stop sharing" bar resets the share button.

### Removed
- Live captions, recording and new AI summaries, because the server cannot decode encrypted media. The transcriber service is gone from `deploy/compose/app.yml` and the Record button from the call; the backend routes stay.

## [2.1.1] - 2026-11-03

### Fixed
- The integration API no longer reports the captions transcriber as a call participant, in forwarded events or in the room's participant list.

## [2.1.0] - 2026-11-03

### Added
- Integration API for other suite apps: create or reuse a room per external id, mint 10-minute LiveKit join tokens, and list who is in the call. Apps authenticate with a secret from `ECHO_INTEGRATION_KEYS`.
- `/embed/{room}` renders a call without Echo's header, for an app to show in an iframe. Only origins in `ECHO_EMBED_ORIGINS` may frame it.
- Call events of integration rooms (participant joined or left, room finished) are forwarded to the app's `callback_url`, signed with HMAC-SHA256 in `X-Echo-Signature`.

### Security
- Every page except `/embed/` now sends `Content-Security-Policy: frame-ancestors 'none'`, so no other site can frame Echo.

## [2.0.1] - 2026-11-02

### Fixed
- Remote audio is played. v2.0.0 received microphone audio but never attached it, so participants could not hear each other.
- An Enable sound button appears when the browser blocks audio autoplay.

## [2.0.0] - 2026-11-02

### Added
- LiveKit as the media server, deployed as infrastructure next to the app.
- Persistent named rooms, guest join with a scoped LiveKit token, moderator grants for the room owner.
- Room recording through LiveKit egress, live French captions from a Vosk transcriber, call history, transcripts and AI summaries.
- porte sign-in with OIDC and a local password fallback, LiveKit webhooks for call events.
- Redesigned entry, lobby and call screens on muse.

### Changed
- The API is Go on tronc and the client is SvelteKit 5, served by one container.
- porte moves from v0.3.1 to v0.5.2, because the v0.3.1 tag was moved and no longer builds from a cold cache.

### Removed
- The Jitsi stack. The last Jitsi version is tagged `jitsi-final`.
