# Echo — API

The HTTP surface Echo exposes: the routes nginx serves, the AI summary proxy Facile added, and
where the upstream iframe API is documented.

## HTTP routes

Everything is served from one hostname by `echo-web`. See
[deployment.md](deployment.md) for the caching and timeout details.

| Method | Path | Auth | What it is |
|---|---|---|---|
| `GET` | `/` and any other path | none | `index.html`. The path is the room name — `/standup` opens the `standup` room |
| `GET` | `/config.js` | none | Runtime configuration, generated at container start from `PUBLIC_URL` |
| `GET` | `/interface_config.js` | none | Static branding configuration |
| `GET` | `/external_api.js` | none | The iframe API client library, built into `libs/` |
| any | `/http-bind` | XMPP | BOSH transport, proxied to Prosody |
| `GET` | `/xmpp-websocket` | XMPP | XMPP over WebSocket, proxied to Prosody |
| `GET` | `/colibri-ws/<id>` | signed | Media signalling, proxied to the video bridge |
| `POST` | `/ai/summarize` | **none** | The AI summary proxy |

There is no REST API, no health endpoint, and no authentication in front of any of these.
Access control is XMPP's: with the default `ENABLE_AUTH=0` and `ENABLE_GUESTS=1`, anyone who
can reach the hostname can create and join any room.

## `POST /ai/summarize`

Facile's addition. nginx proxies it to `echo-ai-proxy:3100`, which runs `ai-proxy.ts` — a
single-file Bun server. It is the only thing in the deployment holding `ANTHROPIC_API_KEY`,
which is why the browser calls it instead of Anthropic directly.

**Request**

```json
{
  "system": "You are a meeting analyst...",
  "message": "Participants:\n- Alice (moderator)\n\nTranscript:\n[Alice]: ..."
}
```

Both fields are passed through verbatim. The client sets neither a model nor a token budget;
the proxy hardcodes `claude-sonnet-4-6` and `max_tokens: 4096` against
`https://api.anthropic.com/v1/messages` with `anthropic-version: 2023-06-01`.

**Response**

```json
{ "content": "{\"summary\": \"...\", \"topics\": [...], ...}" }
```

`content` is the first text block of the Anthropic response, or an empty string if there is
none. The client parses it as JSON, stripping a surrounding Markdown fence first.

**Status codes**

| Status | When |
|---|---|
| `200` | Success |
| `405` | Any method other than `POST` or `OPTIONS` |
| `503` | `ANTHROPIC_API_KEY` is unset. The proxy logs a warning at startup and answers `{"error": "ANTHROPIC_API_KEY not configured"}` |
| upstream status | Anthropic's status and body are passed through on failure |

`OPTIONS` returns `Access-Control-Allow-Origin: *` with `POST` and `Content-Type` allowed. The
route is unauthenticated and permissively CORS'd — anything that can reach the hostname can
spend the Anthropic budget. It is only as private as the network in front of it.

**Client behavior.** `react/features/ai-summary/summarizer.ts` posts here with a 90-second
`AbortController` timeout and up to three retries with exponential backoff plus jitter, retrying
only on `429`, `500`, `503` and `529`. Transcripts estimated over 100,000 tokens are chunked and
summarized in several calls, then merged in a final one — so a long meeting produces a burst of
requests, not one.

**Swapping the model.** The proxy is 60 lines and mounted read-only from the repo root, so
changing provider means editing `ai-proxy.ts` and restarting `echo-ai-proxy`. The client's
`config.ai.provider` field accepts `'claude' | 'openai' | 'custom'` in its type but nothing in
the client branches on it; only `proxyUrl` matters.

## Endpoint messages

Not HTTP, but the other integration surface worth knowing. Transcription results arrive over
the conference data channel as endpoint messages with JSON type `transcription-result`, handled
by `ENDPOINT_MESSAGE_RECEIVED` and `NON_PARTICIPANT_MESSAGE_RECEIVED` in the Redux store. The
transcriber sends them from the hidden domain, so they arrive as non-participant messages.

Both the built-in closed captions and Facile's `ai-summary` middleware consume that same
stream. Anything else that wants the live transcript should subscribe to those action types
rather than adding a second consumer path.

## Upstream APIs

Unchanged by the fork and documented by Jitsi, not here:

- **iframe API** — embedding Echo in another page, `JitsiMeetExternalAPI`, its commands and
  events: [Jitsi handbook, iframe API](https://jitsi.github.io/handbook/docs/dev-guide/dev-guide-iframe).
  `doc/examples/api.html` is upstream's working example, still in the tree.
- **`lib-jitsi-meet`** — the low-level WebRTC API:
  [handbook, ljm-api](https://jitsi.github.io/handbook/docs/dev-guide/dev-guide-ljm-api).
- **`config.js` reference** — every upstream option:
  [handbook, configuration](https://jitsi.github.io/handbook/docs/dev-guide/dev-guide-configuration).

Facile changed `modules/API/API.js` and `modules/API/external/external_api.js` only for the
rebrand; no commands or events were added or removed.
