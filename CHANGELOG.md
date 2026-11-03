# Changelog

## [Unreleased]

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
