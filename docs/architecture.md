# Echo — Architecture

How the seven containers fit together, and exactly what Facile changed relative to upstream
Jitsi Meet.

## Runtime topology

```
Internet ──▶ Traefik (dokploy-network)
                │  one hostname: echo.facile.studio
                ▼
         echo-web  (nginx:alpine, :80)
           ├─▶ /                 index.html — the SPA, any path is a room name
           ├─▶ /config.js        generated at container start from PUBLIC_URL
           ├─▶ /http-bind        ──▶ echo-prosody:5280   BOSH
           ├─▶ /xmpp-websocket   ──▶ echo-prosody:5280   XMPP over WS
           ├─▶ /colibri-ws/*     ──▶ echo-jvb:9090       media signalling
           └─▶ /ai/summarize     ──▶ echo-ai-proxy:3100  ──▶ api.anthropic.com

         echo-prosody   XMPP server — MUC, lobby, breakout rooms, transcript routing
         echo-jicofo    conference focus, allocates bridges
         echo-jvb       video bridge — UDP :10000 published directly to the host
         echo-jigasi    transcriber, joins as a hidden participant
         echo-vosk      alphacep/kaldi-fr speech recognition, ws://echo-vosk:2700
         echo-ai-proxy  Bun sidecar holding ANTHROPIC_API_KEY

         All on the internal `echo` network. Only echo-web also joins dokploy-network.
```

Media does **not** go through nginx. The JVB publishes UDP `10000` straight to the host, which
is why Echo cannot follow the suite's one-container rule — see
[deployment.md](deployment.md).

## The fork

Upstream at the fork point is commit `1814e37e2` (2026-05-21). Everything after it is Facile's.
The delta is roughly 3,300 insertions against 67,000 deletions across 1,120 files, and most of
the deletions are removals rather than rewrites.

### Removed

- **All mobile and native code.** `react-native-sdk/`, `twa/` (the Trusted Web Activity
  Android wrapper), `index.android.js`, `index.ios.js`, `metro.config.js`,
  `react-native.config.js`, `tsconfig.native.json` and `globals.native.d.ts` are gone. The
  `.native.ts` platform-suffix convention still exists in the tree because unremoved feature
  files use it, but there is no native build target.
- **All GitHub automation** — workflows, issue templates, the pull request template and
  Dependabot config.
- **The stock `jitsi/web` image.** Echo builds its own nginx image from source.

`package.json` still carries React Native dependencies; removing them was not part of the fork
work.

### Added

| Path | What it is |
|---|---|
| `react/features/ai-summary/` | The AI meeting summary feature — 12 files, the only new Redux feature |
| `ai-proxy.ts` | Bun sidecar that forwards summary requests to the Anthropic API |
| `Dockerfile`, `docker/nginx.conf`, `docker/entrypoint.sh` | Echo's own web image |
| `docker-compose.yml`, `.env.example` | The seven-service deployment |
| `static/fonts/GogaTest-*.otf` | Ten weights of the Goga typeface |
| `css/_ai-summary.scss` | Styling for the summary panel |
| `react/features/base/icons/svg/chevron-{up,down}.svg`, `monitor.svg` | Icons the redesign needed |

### Rebranded

- `interface_config.js` — `APP_NAME: 'Echo'`, `PROVIDER_NAME: 'Facile'`,
  `JITSI_WATERMARK_LINK: 'https://facile.studio'`, `DEFAULT_BACKGROUND: '#ffffff'`,
  `MOBILE_APP_PROMO: false`, `LANG_DETECTION: false`, empty `SUPPORT_URL`.
- `react/features/base/ui/tokens.json` — the whole palette replaced with a monochrome
  light-theme set (`#0a0a0a` through `#f0f0f2`, red `#dc2626`, green `#16a34a`, amber
  `#f59e0b`) and every `fontFamily` switched to Goga. `jitsiTokens.json` follows.
- Roughly 120 icons under `react/features/base/icons/svg/` swapped for the Solar set.
- 33 SCSS files retuned for the light theme, `_welcome_page.scss` most heavily.
- `title.html`, `manifest.json`, `index.html`, `favicon.ico`, `images/watermark.svg`.
- `fonts.html` declares ten `@font-face` rules for Goga, weights 100 to 900.

### Localized

`react/features/base/i18n/i18next.ts` changes `DEFAULT_LANGUAGE` from `en` to `fr` and
pre-bundles both languages into the app shell:

```ts
for (const [ lng, main, countries ] of [
    [ 'fr', MAIN_FR, COUNTRIES_FR_MERGED ],
    [ 'en', MAIN_EN, COUNTRIES_EN_MERGED ]
] as const) {
    i18next.addResourceBundle(lng, 'main', main, true, true);
    ...
}
```

Upstream preloads only the default language and fetches the rest over HTTP. Pre-bundling both
was the fix for i18n 404s on first paint. `LANG_DETECTION: false` in `interface_config.js`
keeps the browser locale from overriding French.

## Transcription

Self-hosted end to end. No transcription vendor sees the audio.

1. `config.js` sets `transcription: { enabled: true, useAppLanguage: true,
   autoCaptionOnTranscribe: true }`. Prosody and Jicofo both run with
   `ENABLE_TRANSCRIPTIONS=1`.
2. Jicofo invites `echo-jigasi` into the room as a hidden participant on
   `hidden.meet.jitsi`, via the `jigasibrewery` MUC.
3. Jigasi runs with `JIGASI_MODE=transcriber` — not the SIP default — and
   `JIGASI_TRANSCRIBER_CUSTOM_SERVICE=org.jitsi.jigasi.transcription.VoskTranscriptionService`
   pointed at `ws://echo-vosk:2700`.
4. `echo-vosk` is `alphacep/kaldi-fr`, a French model. English audio will transcribe poorly.
5. `JIGASI_TRANSCRIBER_RECORD_AUDIO=0` and `JIGASI_TRANSCRIBER_SEND_TXT=1`: no audio is stored,
   only text is emitted.
6. Results come back to participants as `transcription-result` endpoint messages over the
   conference data channel, which is what drives closed captions — and what the summary
   feature listens to.

## AI summary

The one genuinely new product feature in the fork. It lives in `react/features/ai-summary/`
and is enabled by `config.js`:

```js
ai: {
    enabled: true,
    autoSummarize: true,
    provider: 'claude',
    proxyUrl: '/ai/summarize'
}
```

The shape is declared in `react/features/base/config/configType.ts` as an optional `ai` block
with `enabled`, `autoSummarize`, `language`, `provider` and `proxyUrl`. `provider` accepts
`'claude' | 'openai' | 'custom'` in the type, but nothing in the client branches on it — the
proxy decides.

**Collection.** `middleware.ts` registers on `CONFERENCE_JOINED` (reset, status `collecting`),
`ENDPOINT_MESSAGE_RECEIVED` and `NON_PARTICIPANT_MESSAGE_RECEIVED` (append entries whose JSON
type is `transcription-result`), and `CONFERENCE_LEFT` (auto-summarize when `autoSummarize` is
on and there is anything to summarize). Every handler returns early when
`config.ai.enabled` is false, so the feature is inert by default.

**Summarization.** `summarizer.ts` formats entries as `[Speaker]: text`, estimates token count
at 1.33 tokens per word, and picks a strategy:

- Under 100,000 estimated tokens — a single call with a system prompt demanding JSON with
  `summary`, `topics`, `decisions`, `actionItems` and `perSpeaker`.
- Over that — map-reduce: chunk into ~12,000-token groups with a 5-entry overlap, summarize
  each, then merge the segment summaries in a final call.

Calls go to `proxyUrl` with a 90-second `AbortController` timeout and up to three retries with
exponential backoff plus jitter, retrying only on 429, 500, 503 and 529. The response is
unwrapped from a Markdown fence if present, parsed as JSON, and normalized into
`ISummaryResult`. The stored `model` field is hardcoded to the string `'claude'`.

**Proxy.** `ai-proxy.ts` is a ~60-line Bun server on port 3100. It accepts `POST` with
`{ system, message }`, forwards to `https://api.anthropic.com/v1/messages` with model
`claude-sonnet-4-6` and `max_tokens: 4096`, and returns `{ content }`. Without
`ANTHROPIC_API_KEY` it warns at startup and answers 503 to everything. It sets permissive CORS
headers on preflight.

**Privacy consequence.** Audio never leaves the deployment, but the *transcript text* does:
when a summary is generated, the full meeting transcript is sent to Anthropic. Setting
`ai.enabled` to `false` in `config.js` disables collection and the button; setting
`autoSummarize` to `false` keeps the feature but requires an explicit click.

**UI.** `AISummaryButton` is registered as toolbar button `ai-summary` in
`react/features/toolbox/constants.ts` and wired through `hooks.web.ts`. `AISummaryPanel`
renders the result and offers two actions: copy the summary as Markdown to the clipboard, and
download the raw transcript as `transcript-YYYY-MM-DD.md` via a Blob object URL.

## The web image

`Dockerfile` is two-stage. Stage one is `node:20-bookworm`: `npm ci --legacy-peer-deps`, then
`make all`, then the short commit SHA written to `/tmp/echo-version`. Stage two is
`nginx:alpine` with the app rooted at `/srv/echo` — built `libs/` and `css/all.css` from stage
one, plus `lang/`, `images/`, `static/`, `sounds/` and the HTML fragments copied from source.
`config.js` is copied as `config.js.template`.

`docker/entrypoint.sh` runs before nginx and does two substitutions:

1. Rewrites the template's `jitsi-meet.example.com` placeholders to `PUBLIC_URL`,
   `wss://<host>` and `XMPP_DOMAIN`, producing the real `config.js`.
2. Replaces `__ECHO_VERSION__` in `index.html` with the baked commit SHA, which is the cache
   -busting mechanism for asset URLs — static files are served with `Cache-Control: public,
   immutable` for a year, so without it a deploy would serve stale bundles.

nginx runs with SSI enabled for JavaScript types, which is how upstream's `config.js`
templating works.

## Suite integration

None. Echo does not federate to Authentik at `porte.facile.studio`, emits no `pool` /
`enveloppe` events, and ships no logs to Journal. Authentication is Prosody's, and by default
`ENABLE_AUTH=0` with `ENABLE_GUESTS=1` — anyone with the URL joins.
