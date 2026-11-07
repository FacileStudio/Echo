# Changelog

## [Unreleased]

### Changed
- Libraries: porte v0.5.2 to v0.12.1, porte/directory v0.2.1 to v0.3.0, tronc v0.11.0 to v0.17.1, muse from the untagged commit 006adfc to v0.13.0.
- Sign-in goes through Registre only. `/api/auth/register` and `/api/auth/login` are gone, `ALLOW_REGISTRATION` and `SSO_ONLY` are no longer read, and `/auth/config` no longer reports `allow_registration`. Guests still join by link with a display name.
- An OIDC login links to an existing account only by a verified email, and refuses when that account already holds another subject of the same issuer. An unverified claim never rewrites a stored address.
- tronc security headers replace the local frame middleware. Camera, microphone and display-capture stay allowed; `/embed/*` can be framed only by `ECHO_EMBED_ORIGINS`; the CSP allows the LiveKit origin and the built client's inline scripts (report-only).
- The UI is in fr, en, es and de, follows the Registre locale, then the locale cookie, then the browser. Dates use Intl in that locale. Moved to muse v0.13 tokens, no borders.

### Added
- GDPR deletion: Echo polls Registre's deletions and accepts the signed push at `POST /api/registre/deletions`. It deletes the person's account, rooms, calls, transcripts, summaries and recordings, and blanks their name on seats in other people's calls.
- `users.locale`, `porte_login_codes.provider` and `.subject`, and `directory_deletion_state`, all added idempotently at boot.
- `GET /version` from the `APP_VERSION` build arg.
- CI: build, tests on Postgres, svelte-check, message catalog check, filet.

## [2.3.2] - 2026-11-06

### Fixed
- The owner's seat holds for the whole call, not only at join time. On every `participant_joined` of a capped room whose owner is absent, Echo disconnects everyone above `limit - 1`, guests first and the most recently joined first. A client can no longer keep its LiveKit-refreshed identities and reconnect them into the seat freed for the owner.

### Changed
- `deadline_key` now carries its expiry and is valid for 2 hours after the join that issued it. A key from v2.3.1 is refused; the client gets a new one on its next join.

### Notes

- A downgrade while the owner is away trims the room to the new limit on the next join, like the call-length deadline ends calls on downgrade.
- The countdown stops receiving server updates two hours after a join (deadline keys expire); the server still ends the call on time.

## [2.3.1] - 2026-11-06

### Changed
- porte/directory v0.2.0 to v0.2.1, and porte/forfait v0.1.0 to v0.1.1.
- Joining an existing room has its own rate limit, 60 a minute per address and room (120 per account), so a whole office behind one NAT can join one call. Creating a room, through `POST /api/rooms` or a token for an unknown slug, keeps the strict 10 a minute per address (30 per account).
- While the owner is not in the call, guests can fill at most `limit - 1` seats. When the owner arrives at a room LiveKit reports full, Echo disconnects the most recently joined guest first. One client address (an IPv6 /64) holds at most one pending seat per room.
- Guest join tokens live 90 seconds instead of 10 minutes. LiveKit refreshes the token of a connected client.
- One address may join at most 120 rooms' worth of tokens a minute overall (240 per account), on top of each room's own budget.
- An unlimited plan creates its LiveKit room with an explicit ceiling of 10,000 instead of 0, so upgrading mid-call raises the ceiling of the running room.

### Added
- `GET /api/rooms/{slug}/deadline` returns the running call's deadline to a caller holding the room's `deadline_key`, which every token response carries. The budget is per address, across rooms. The call countdown polls it every minute, so an upgrade or a restart reaches it.
- Without `OIDC_ISSUER` the API logs at boot that plans are never synced and every limit stays open. The README says so too.

### Fixed
- An existing but empty `forfait_state` row now enforces the free plan, like an unreadable one. forfait v0.1.1 imports empty input as a no-op, which would have failed open. To recover from a bad row, delete it rather than blanking it; see `docs/plans.md`.
- A call row the 5-minute reconcile closes, because LiveKit forgot its session, is now forwarded to integrating apps as a `room_finished`, unless LiveKit runs a newer session of the same room; that row is closed silently so Prisme does not end the running call.
- A join admitted while the limiter was cutting the room at its deadline is refused, instead of starting a fresh session. The cut is recorded before the room closes.
- The limiter forgets cuts older than 10 minutes instead of keeping them forever.

## [2.3.0] - 2026-11-04

### Added
- Plan limits from Registre (porte/forfait), read for the room owner's Registre subject: `echo.max_participants` (free: 4), `echo.max_call_minutes` (free: 40) and `echo.recording` (free: off). A room with no owner, or whose owner never signed in through Registre, gets the free limits. Integration rooms are unlimited.
- A full call refuses a new participant with HTTP 402 (`payment_required`, with `key`, `limit` and `upgrade_url`), and the join page offers the upgrade link. The count is LiveKit's participant list, falling back to call rows, plus tokens admitted in the last 60 seconds that have not connected yet. That count never refuses the signed-in owner; a limit of 0 refuses everyone with 402.
- `POST /api/rooms` and `POST /api/rooms/{slug}/token` are rate limited: 10 a minute per address for guests (an IPv6 caller per /64, and 40 per /48), 30 per account for signed-in users (429 above that). The limiter keeps at most 50,000 buckets and evicts the least recently used.
- Behind Cloudflare the limiter keys on the visitor. `Cf-Connecting-Ip` is believed only when the peer is on the private network and the rightmost `X-Forwarded-For` hop is a Cloudflare edge, which is the chain production's Traefik passes. Every other request keys on the address tronc resolves from `TRUSTED_PROXIES` (default `private,cloudflare`).
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
