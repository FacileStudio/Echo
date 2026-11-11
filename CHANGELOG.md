# Changelog

## [2.8.1] - 2026-11-11

### Security

- github.com/go-chi/chi/v5 v5.3.1 to v5.4.0 (CVE-2026-114437, client IP spoofing in `ClientIPFromXFF`; Echo does not call it).
- github.com/google/cel-go v0.28.1 to v0.30.0, indirect through LiveKit's protovalidate (GHSA-gcjh-h69q-9w9g).
- vite ^8.3.8 (8.2.2 to 8.3.8), with `overrides` for devalue ^5.9.3 (5.9.1 to 5.9.4) and source-map-js ^1.2.2 (1.2.1 to 1.2.2).
- An `overrides` entry pins uuid ^11.1.1 (CVE-2026-41907). vosk-browser 0.0.8 pins uuid 9.0.0 but bundles its own copy of the v4 code in `dist/vosk.js`, so the shipped bundle does not change.

## [2.8.0] - 2026-11-11

### Added

- A remote MCP server at `/mcp` on tronc/mcp, behind Registre tokens bound to `MCP_RESOURCE` (scopes `echo.read`, `echo.write`). Nine tools: `echo_list_rooms`, `echo_create_room`, `echo_rename_room`, `echo_delete_room`, `echo_list_meetings`, `echo_get_meeting`, `echo_read_transcript`, `echo_summarize_meeting` and `echo_share_recording`. Each calls the service behind the matching web route, so a client sees what the same person sees in Echo. Deleting a room is confirmed in the client, or on the new `/pending` page when the client cannot ask. A person's first MCP call creates their account. MCP is free; transcripts and summaries keep their `ai.access` gate.
- `echo_create_room` returns the invite link with a fresh room key, which Echo hands over and does not store.
- An erasure also deletes the person's pending MCP requests.

### Changed

- tronc v0.22.1, porte/mcpauth v0.2.1, modelcontextprotocol/go-sdk v1.8.0.

### Migration

- Additive: the `mcp_actions` table and its index (`tronc/mcp.PendingSchema`), created at boot. Old, new, old, new boots against one database pass.

## [2.7.1] - 2026-11-11

### Changed

- Licensed under the GNU Affero General Public License v3.0 only (`AGPL-3.0-only`). See `LICENSE`.
- The compiled binary `apps/api/api`, tracked since v2.0.0, is gone from the tree, and `.gitignore` keeps `apps/api/api` and `apps/api/bin/` out.

### Security

- Go 1.26.11 with the `golang:1.26.11-alpine` builder, for the standard library advisories published on 2026-11-11 (GO-2026-6848 to GO-2026-6859).
- google.golang.org/grpc v1.83.2, github.com/klauspost/compress v1.20.2 and github.com/go-jose/go-jose/v4 v4.1.6, which clear every high advisory in the production binary.

## [2.7.0] - 2026-11-11

### Changed
- Sign-in runs on porte/kit (porte v0.18.3). The account store, the request identity and the session middleware come from porte instead of Echo's own copies (`internal/authcontext`, `internal/middleware/auth.go` and `modules/auth/oidc.go` are gone). Routes, their unauthenticated status codes, the `/api/auth/config` body and the 401 bodies are unchanged. One rule changes: at sign-in the stored picture becomes whatever Registre sends, so a Registre account without a picture now clears Echo's copy.
- `users.password_hash` gets the database default `''` at boot, because the kit's insert does not name it.
- The rate limiter keys on the address tronc's RealIP resolved. tronc now recovers the visitor from `Cf-Connecting-Ip` on the genuine Cloudflare chain (`httpx.Config.CDNProxies`), and Echo no longer reads that header itself. `TRUSTED_PROXIES` must keep `cloudflare`, which is the default, or every visitor shares the edge's bucket. Request logs now show the visitor's address.
- tronc v0.21.0, porte v0.18.3, porte/directory v0.4.0, Go 1.26.10, golang.org/x/net v0.62.0, golang.org/x/crypto v0.58.0.
- Gates: filet v0.19.2 with `preset: facile` (same findings as before), douane v0.7.2 in CI with `-fail critical -scope prod`, a tracked `lefthook.yml` (`mise run hooks`). `douane.yml` permits GO-2026-5932 until 2027-02-08: its only fix is an unreleased x/crypto commit.

### Fixed
- A Registre erasure goes through porte/kit's `EraseAccount`, which matches the issuer with or without a trailing slash, and now also replaces the erased person's `user-<id>` in the AI summary of every call they joined in someone else's room. Before, the summary kept it. `docs/erasure.md` states the rule, and a test scans every text column after an erasure.

### Rollback
- Safe in both directions with no data step. v2.6.2's AutoMigrate removes the `password_hash` default again and does not need it; this version puts it back at boot. Measured old, new, old, new against one database.

## [2.6.2] - 2026-11-07

### Changed
- `GET /version` is baked into the image from the version in `apps/client/package.json` (`scripts/version.sh`), so it matches the release tag without any deploy environment change. The `APP_VERSION` build argument is gone. Nothing in the UI changes.

## [2.6.1] - 2026-11-07
### Security
- Registre tokens are sealed at rest. porte v0.12.1 to v0.13.0: `porte_identities.access_token` and `refresh_token` are encrypted with AES-256-GCM under `PORTE_TOKEN_KEY`. Existing rows are sealed once, in the background, after boot, and the count is logged. An invalid `PORTE_TOKEN_KEY` stops the boot. With the key unset nothing changes.
- Losing `PORTE_TOKEN_KEY` makes the stored tokens unreadable: people sign in again, nothing else is lost. Do not roll back to a build older than porte v0.13.0 after the first reseal.

## [2.6.0] - 2026-11-07

### Added
- In-browser transcription and recording that keep end-to-end encryption intact. The organizer switches them on for the call (`PUT /api/rooms/{slug}/capture`, owner only; transcription needs `ai.access`, recording needs `echo.recording`, checked on the server). Every participant sees a banner and must accept for their own voice to be transcribed or recorded (`PUT .../capture/consent`); `GET .../capture` is the state they poll. Nobody is captured until they answer yes, and switching a feature off and on again asks everyone again.
- Transcription runs on the device: each consenting browser turns its own microphone into text with Vosk compiled to WebAssembly (vosk-browser 0.0.8, `vosk-model-small-fr-0.22` and `vosk-model-small-en-us-0.15`, about 41 MB each) and posts only final lines to `POST /api/rooms/{slug}/transcript`. The model downloads after the person accepts, with progress, and is kept in the browser's Cache API. The speech language is French or English (the session language, changeable in the banner).
- The speaker of a line is never read from the request. A signed-in caller is identified by the session; a guest sends the `seat_key` the token response now carries, signed under the LiveKit secret, in `X-Echo-Seat-Key`. Either way the caller must hold an open seat in the room's open call. Lines are limited to 4 KB, 16 KB per request and 90 per minute per seat.
- Recording is audio only and made in the organizer's browser: a Web Audio mix of the voices of people who accepted, recorded with MediaRecorder (webm/opus, ogg/opus or mp4/aac) and uploaded in 5 second chunks to `POST /api/rooms/{slug}/capture/recording`, `PUT .../recording/{id}/chunks/{n}` and `POST .../recording/{id}/finalize`. Chunks append strictly in order, a retry is acknowledged, a gap answers 409 with the chunk to resume from. A call's recordings are capped at 2 GiB in total and 8 MiB per chunk. It is finalized when the organizer stops, when the call ends (the `room_finished` webhook), or after 15 minutes without a chunk, so a tab closed mid-call keeps what was uploaded.
- `recordings` carries `content_type`, `status`, `size` and `chunks`; `calls` carries `transcribe_on` and `record_on`; `capture_consents` holds the answers. A recording still uploading is hidden from every list.
- The image downloads the two Vosk models at build time (pinned by sha256, `scripts/vosk-models.sh`) and serves them from `/models/`; `mise run models` does the same for development.

### Security notes
- Transcript lines are enforced server-side per speaker: a withdrawal stops them at once. The recording mix is built in the organizer's browser, because calls are end-to-end encrypted and the server hears nothing: the server cannot stop a modified organizer client from recording a voice that declined. Withdrawal reaches an honest client within one 3 s poll.

### Changed
- The CSP allows a blob worker, `blob:` connects and `'wasm-unsafe-eval'`, which the on-device recognizer needs.

### Removed
- The server transcriber (`services/transcriber`) and `TRANSCRIBER_TOKEN`. The ingestion route is now the browser one above.

## [2.5.0] - 2026-11-07

### Added
- Your meetings: `/history` lists the calls a signed-in person owns or joined, newest first, 20 per page, with date, duration and participants (names and Registre photos). `GET /api/meetings` scopes by the caller's account in the query.
- Seats in `call_participants` carry the account (`user_id`), backfilled at boot for `user-<id>` identities whose account still exists.
- Recordings are rows (`recordings`), one per egress, backfilled once from `calls.recording_path`. `GET /api/calls/{id}/recordings`, `GET /api/recordings/{id}/file` (range-capable, inline or `?download=1`, private, no-store) and `PATCH /api/recordings/{id}` with `{"shared": bool}`. Only the owner sees a recording until they share it with the call's participants; turning sharing on needs `echo.recording`.
- A participant now opens the call page: participants, transcript and summary. Only the owner generates the summary.
- Transcripts keep one timed row per utterance (`transcript_lines`). `GET /api/calls/{id}/transcript` searches with `?q=` and exports with `?format=txt` or `?format=vtt`. Transcripts written before this release stay readable and searchable but have no timings, so no `.vtt`.
- GDPR erase also deletes transcript lines and recording rows with the owner's rooms, and on calls the person only joined it replaces their `user-<id>` identity by `erased-<seat>` in the seat, the transcript text and the lines, and clears `user_id`.

### Changed
- Transcripts and summaries are AI features and follow the room owner's `ai.access` entitlement (free: off). Reading, searching and exporting a transcript, generating a summary and the transcriber's ingestion answer 402 with `upgrade_url` without it; the client shows Echo's plan notice. Fails open without Registre, like every other limit.
- `GET /api/calls/{id}` no longer returns the raw transcript. It returns `has_transcript`, `role`, `room_slug` and `room_name`. `GET /api/calls/{id}/recording` is replaced by the recordings routes above.
- Recording streams get a two hour write budget instead of the server's 30 seconds.

## [2.4.0] - 2026-11-07

### Changed
- Libraries: porte v0.5.2 to v0.12.1, porte/directory v0.2.1 to v0.3.0, tronc v0.11.0 to v0.17.1. muse stays at the untagged commit 006adfc: Echo keeps its own design.
- Sign-in goes through Registre only. `/api/auth/register` and `/api/auth/login` are gone, `ALLOW_REGISTRATION` and `SSO_ONLY` are no longer read, and `/auth/config` no longer reports `allow_registration`. Guests still join by link with a display name.
- An OIDC login links to an existing account only by a verified email, and refuses when that account already holds another subject of the same issuer. An unverified claim never rewrites a stored address.
- tronc security headers replace the local frame middleware. Camera, microphone and display-capture stay allowed; `/embed/*` can be framed only by `ECHO_EMBED_ORIGINS`; the CSP allows the LiveKit origin and the built client's inline scripts (report-only).
- The UI is in fr, en, es and de, follows the Registre locale, then the locale cookie, then the browser. Dates use Intl in that locale. Text only: markup, classes and colours are unchanged.

### Added
- Signed-in people take their name and photo from the Registre claims, refreshed at every login (`users.picture`). Calls use that name and put the photo in the LiveKit participant metadata (`avatar_url`), which the tile shows in place of the initials.
- The GDPR erase also deletes the person's recording MP4 files. The API now mounts the recordings volume writable, and a one-shot `recordings-perms` service opens the directory to its non-root user.
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
- Signed-in people take their name and photo from the Registre claims, refreshed at every login (`users.picture`). Calls use that name and put the photo in the LiveKit participant metadata (`avatar_url`), which the tile shows in place of the initials.
- The GDPR erase also deletes the person's recording MP4 files. The API now mounts the recordings volume writable, and a one-shot `recordings-perms` service opens the directory to its non-root user.
- `GET /api/rooms/{slug}/deadline` returns the running call's deadline to a caller holding the room's `deadline_key`, which every token response carries. The budget is per address, across rooms. The call countdown polls it every minute, so an upgrade or a restart reaches it.
- Without `OIDC_ISSUER` the API logs at boot that plans are never synced and every limit stays open. The README says so too.

### Fixed
- An existing but empty `forfait_state` row now enforces the free plan, like an unreadable one. forfait v0.1.1 imports empty input as a no-op, which would have failed open. To recover from a bad row, delete it rather than blanking it; see `docs/plans.md`.
- A call row the 5-minute reconcile closes, because LiveKit forgot its session, is now forwarded to integrating apps as a `room_finished`, unless LiveKit runs a newer session of the same room; that row is closed silently so Prisme does not end the running call.
- A join admitted while the limiter was cutting the room at its deadline is refused, instead of starting a fresh session. The cut is recorded before the room closes.
- The limiter forgets cuts older than 10 minutes instead of keeping them forever.

## [2.3.0] - 2026-11-04

### Added
- Signed-in people take their name and photo from the Registre claims, refreshed at every login (`users.picture`). Calls use that name and put the photo in the LiveKit participant metadata (`avatar_url`), which the tile shows in place of the initials.
- The GDPR erase also deletes the person's recording MP4 files. The API now mounts the recordings volume writable, and a one-shot `recordings-perms` service opens the directory to its non-root user.
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
- Signed-in people take their name and photo from the Registre claims, refreshed at every login (`users.picture`). Calls use that name and put the photo in the LiveKit participant metadata (`avatar_url`), which the tile shows in place of the initials.
- The GDPR erase also deletes the person's recording MP4 files. The API now mounts the recordings volume writable, and a one-shot `recordings-perms` service opens the directory to its non-root user.
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
- Signed-in people take their name and photo from the Registre claims, refreshed at every login (`users.picture`). Calls use that name and put the photo in the LiveKit participant metadata (`avatar_url`), which the tile shows in place of the initials.
- The GDPR erase also deletes the person's recording MP4 files. The API now mounts the recordings volume writable, and a one-shot `recordings-perms` service opens the directory to its non-root user.
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
- Signed-in people take their name and photo from the Registre claims, refreshed at every login (`users.picture`). Calls use that name and put the photo in the LiveKit participant metadata (`avatar_url`), which the tile shows in place of the initials.
- The GDPR erase also deletes the person's recording MP4 files. The API now mounts the recordings volume writable, and a one-shot `recordings-perms` service opens the directory to its non-root user.
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
