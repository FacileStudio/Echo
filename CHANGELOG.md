# Changelog

## [Unreleased]

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
