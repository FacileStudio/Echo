# Integration API

Other suite apps run calls inside themselves through this API. Prisme, the suite chat, is the
first caller: each Matrix room gets one Echo room, Prisme mints a join token per person, and the
call opens in an iframe on Echo's embed route.

The API is server to server. The app's backend calls it with a shared secret; the browser only
ever sees a join token and the embed URL.

## Authentication

Every route takes `Authorization: Bearer <secret>`, where the secret is the app's entry in
`ECHO_INTEGRATION_KEYS`:

```sh
ECHO_INTEGRATION_KEYS=prisme:<secret>,bulle:<secret>
```

- App names are lowercase letters and digits, up to 32 characters.
- Each secret is at least 32 characters. Generate one with `openssl rand -hex 32`.
- A malformed value refuses the boot.
- Empty means the API refuses every caller with 401.
- Echo compares the presented secret against every configured one in constant time, over
  SHA-256 digests, so timing tells a caller nothing.

The routes carry no cookie, so they need no `X-Facile-CSRF` header.

A Registre token-exchange JWT with audience `echo` is not accepted yet. Echo has no OIDC issuer
configured in production, so porte has nothing to verify such a token against. Adding it means
setting the `OIDC_*` variables, adding the `echo` audience to Registre, and mapping the token's
client to an app name.

## Routes

All paths sit under `/api`. Errors use the tronc envelope:
`{"error": {"code": "...", "message": "..."}}`.

### Create or reuse a room

`POST /api/integrations/rooms`

```json
{"external_id": "!abc123:prisme.facile.studio", "title": "#general", "callback_url": "https://prisme.facile.studio/api/echo/events"}
```

| Field | Required | Rules |
|---|---|---|
| `external_id` | yes | The app's own key for the room, at most 255 bytes |
| `title` | no | At most 200 characters; defaults to `external_id` |
| `callback_url` | no | Absolute `http` or `https` URL that receives call events |

Answer, `201` on creation and `200` when the room already existed:

```json
{"room_id": "prisme-4f9c2a1b0d3e5f60", "join_url": "https://echo.facile.studio/embed/prisme-4f9c2a1b0d3e5f60"}
```

The call is idempotent per `(app, external_id)`; a unique index enforces it in the database. A
repeated call updates `title` and `callback_url` when it sends them and keeps the stored
values for the fields it leaves out. Two apps with the same `external_id` get two rooms.

`room_id` is `<app>-<16 hex>` and is also the LiveKit room name. `join_url` is built from the host
and `X-Forwarded-Proto` of the request, so call Echo on its public URL.

### Mint a join token

`POST /api/integrations/rooms/{room_id}/tokens`

```json
{"identity": "@alice:prisme.facile.studio", "name": "Alice", "avatar_url": "https://prisme.facile.studio/_matrix/media/v3/download/x"}
```

| Field | Required | Rules |
|---|---|---|
| `identity` | yes | The app's stable id for the person, at most 128 bytes |
| `name` | yes | Display name, at most 128 characters |
| `avatar_url` | no | Absolute `http` or `https` URL, carried in participant metadata as `{"avatar_url": "..."}` |

Answer `200`:

```json
{
  "livekit_url": "wss://echo-live.facile.studio",
  "token": "<LiveKit JWT>",
  "embed_url": "https://echo.facile.studio/embed/prisme-4f9c2a1b0d3e5f60#token=<JWT>&url=wss%3A%2F%2Fecho-live.facile.studio"
}
```

- The token joins that one room with publish, subscribe and data grants, no admin rights.
- It can open a connection for **10 minutes**. LiveKit refreshes the token of a connected client
  on its own, so a long call is not cut; a leaked token stops working after 10 minutes. On a
  self-hosted LiveKit a short TTL is the only way to stop a removed participant from rejoining
  (https://docs.livekit.io/frontends/authentication/tokens). Mint the token when the person
  clicks Join, not ahead of time.
- The LiveKit identity is `<identity>#<8 hex>`, a new suffix per token. Without it the same person
  on two devices would share an identity, and LiveKit disconnects the older connection
  (`DUPLICATE_IDENTITY`). Echo strips the suffix everywhere it reports an identity back.
- LiveKit records identities in its logs. Prefer an opaque id over an email address.
- A room of another app answers `404`.

`embed_url` opens the call on the embed route with the token in the URL fragment. Browsers never
send a fragment to a server, so the token does not land in access logs. Append `&key=<room key>`
before opening it (see [End-to-end encryption](#end-to-end-encryption)): `embed_url` carries no
key, because Echo never knows it. Apps that run their own LiveKit client use `livekit_url` and
`token` directly, with LiveKit end-to-end encryption on and the same key.

### Read a room

`GET /api/integrations/rooms/{room_id}`

```json
{"room_id": "prisme-4f9c2a1b0d3e5f60", "title": "#general", "participants": [{"identity": "@alice:prisme.facile.studio", "name": "Alice"}]}
```

`participants` lists who is connected to the room's latest call, once per app identity, and is
empty when no call is running. It comes from the call records LiveKit's webhooks maintain, so it
trails reality by the webhook delay, about a second.

## Guests and the public routes

An integration room is joined only with tokens its app mints. Echo's public routes refuse it:
`POST /api/rooms/{slug}/token` answers `403` and `GET /api/rooms/{slug}` answers `404`. A leaked
`join_url` without a token opens nothing.

## End-to-end encryption

Every Echo call is end-to-end encrypted, always. Audio, video, screen shares and chat leave the
browser encrypted with a shared room key, through LiveKit's E2EE
(https://docs.livekit.io/transport/encryption/). The LiveKit server routes ciphertext and never
learns the key.

The integrating app owns the key and its distribution:

- Derive or generate one key per room and give the same key to everyone in that room. Prisme
  derives one per Matrix room and never sends it to Echo.
- Pass it in the fragment as `key`. Any string works: Echo hands it to LiveKit's
  `ExternalE2EEKeyProvider.setKey` as a passphrase (PBKDF2). Use at least 32 random bytes,
  base64url-encoded.
- Never send the key to Echo's API. Echo's backend has no route that accepts one.

Someone who joins with a different key receives nothing they can decode. The call shows "Can't
decrypt this call: your link is missing the right key" instead of crashing. A browser without
insertable streams or `RTCRtpScriptTransform` cannot encrypt, so the page tells the person to
switch browsers rather than join in plaintext.

Server-side features cannot read encrypted media, so integration rooms have no recording, live
captions, transcript or AI summary.

## Embed route

`/embed/{room_id}#token=<JWT>&url=<livekit_url>&key=<room key>` renders the call without Echo's
header: video grid, chat and controls only. The page reads the fragment, then removes it from the
address bar. Without `key` the page refuses to join and says the link has no encryption key.
Leaving posts `{"type": "echo.left", "room": "<room_id>"}` to the parent window, so the app can
close its panel.

Framing is allowed only from the origins in `ECHO_EMBED_ORIGINS`:

```sh
ECHO_EMBED_ORIGINS=https://prisme.facile.studio
```

Echo sends `Content-Security-Policy: frame-ancestors <origins>` on `/embed/*` and
`frame-ancestors 'none'` on every other path, so no other page of Echo can be framed. Empty means
no origin may frame the embed route. Each entry must be a bare origin (scheme and host, no path).
See https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Security-Policy/frame-ancestors.

The parent page must delegate the media permissions to the frame. Browsers block camera,
microphone and screen capture in a cross-origin iframe unless the `allow` attribute names them
(https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Permissions-Policy,
https://www.chromium.org/Home/chromium-security/deprecating-permissions-in-cross-origin-iframes,
https://developer.mozilla.org/en-US/docs/Web/API/Screen_Capture_API/Using_Screen_Capture):

```html
<iframe
  src="https://echo.facile.studio/embed/prisme-4f9c2a1b0d3e5f60#token=...&url=...&key=..."
  allow="camera; microphone; display-capture; autoplay; fullscreen"
></iframe>
```

Screen sharing needs `display-capture` in that list: without it the browser rejects the share
before showing its picker. If the parent page sends its own `Permissions-Policy` header, it must
allow these features for `https://echo.facile.studio` too. Without `autoplay` the call shows an
Enable sound button until the person clicks in the frame.

## Call events

When a room has a `callback_url`, Echo forwards three LiveKit events for it:
`participant_joined`, `participant_left` and `room_finished`.

```http
POST <callback_url>
Content-Type: application/json
X-Echo-Signature: 3b1f...e9

{"id": "6f1c...", "event": "participant_joined", "room_id": "prisme-4f9c2a1b0d3e5f60", "external_id": "!abc123:prisme.facile.studio", "at": "2026-11-03T10:00:00Z", "participant": {"identity": "@alice:prisme.facile.studio", "name": "Alice"}}
```

- `X-Echo-Signature` is the lowercase hex HMAC-SHA256 of the raw body, keyed with the app's
  integration secret. Verify it over the exact bytes received, before parsing, with a
  constant-time compare.
- `participant` is present on the two participant events only.
- Only people who joined with a token from this API are reported. Workers that join every room
  on their own, such as the captions transcriber (`agent-...` identities), are left out, here and
  in `GET /api/integrations/rooms/{room_id}`.
- `at` is when LiveKit says the event happened. `id` is unique per event.
- Delivery is attempted 3 times, 2 then 4 seconds apart, with a 10 second timeout. Any `2xx`
  counts as delivered. Answer fast and do the work afterwards.
- Events can arrive late, twice or out of order (LiveKit itself retries). Use `id` to drop
  duplicates, and reject an `at` far in the past if replays matter to you; the timestamp sits
  inside the signed body, so it cannot be altered.

This follows the common HMAC-over-raw-body pattern (GitHub's `X-Hub-Signature-256`) described in
https://safeguard.sh/resources/blog/webhook-security-best-practices and
https://github.com/standard-webhooks/standard-webhooks/blob/main/spec/standard-webhooks.md.

Verification in Go:

```go
mac := hmac.New(sha256.New, []byte(secret))
mac.Write(body)
ok := hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))), []byte(r.Header.Get("X-Echo-Signature")))
```

## Toile whiteboard

Echo calls Toile here, not the other way round. In a room on echo.facile.studio (not in `/embed`), the organizer gets a whiteboard button next to transcription and recording. It sends `POST https://toile.facile.studio/api/boards/meeting` with `{"room": "<slug>"}`, the Toile session cookie and `X-Facile-CSRF: 1`. Toile returns the room's board, the same one on every call, and an edit share link; Echo opens `share.url` (else `url`) in a new tab, and refuses any link outside Toile's origin. A 401 asks the person to sign in to Toile first.

Toile has to allow `https://echo.facile.studio` through CORS with credentials for this call. `VITE_TOILE_BASE` overrides the Toile origin at build time; the default is production.

## Decisions

Recorded 2026-11-03, when the API was built for Prisme.

- **Integration rooms are ordinary rooms with three extra columns** (`integration_app`,
  `external_id`, `callback_url`). They reuse the webhook pipeline, so calls and participants are
  recorded with no new code path.
- **The room id is the slug**, because it is also the LiveKit room name and the embed path.
- **The signature is plain hex of the body**, with no `sha256=` prefix and no separate timestamp
  header, which is the contract Prisme was built against. Replay data lives in the signed body.
- **The token rides in the URL fragment** rather than a query string or `postMessage`, so it
  stays out of server logs and needs no handshake between frames.
- **The encryption key rides in the fragment too, and the app owns it** (added 2026-11-03). Echo
  cannot hand out a key it is meant never to see.
- **No third-party cookie is involved.** The embed page needs nothing from Echo but the token,
  so browsers that block third-party cookies still work.
- **The public base URL comes from the request**, not a setting. One fewer variable to keep in
  sync; the cost is that apps must call Echo on its public host.
- **Registre JWT auth is deferred** until Echo has OIDC configured.
- **Integration rooms are not limited by any plan** (added 2026-11-04). Echo reads plan limits
  (`echo.max_participants`, `echo.max_call_minutes`, `echo.recording`) from the room owner's
  Registre subject, and an integration room has no owner: the app that created it pays for it
  through its own product. Applying the free plan there would cut every existing Prisme call at
  four people and forty minutes.
