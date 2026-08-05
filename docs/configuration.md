# Echo — Configuration

Every environment variable the deployment actually reads, and the three configuration files
that are baked into the image.

The tables below come from `docker-compose.yml`, `docker/entrypoint.sh`, `ai-proxy.ts` and
`webpack.config.js` — not from `.env.example`, which has drifted. Two variables the deployment
requires are missing from it entirely.

## Required

| Variable | Required | Default | What it does |
|---|---|---|---|
| `PUBLIC_URL` | yes | `https://${XMPP_DOMAIN}` | Public origin. `entrypoint.sh` substitutes it into `config.js` and derives the `wss://` URL from it. A trailing slash is stripped |
| `JVB_ADVERTISE_IPS` | yes | — | Public IP the video bridge advertises to peers. Without it, media never connects from outside the host |
| `CONFIG` | yes | — | Host path holding `prosody/`, `jicofo/`, `jvb/` and `jigasi/` config volumes |
| `JICOFO_AUTH_PASSWORD` | yes | — | Jicofo's XMPP password. Shared by Prosody and Jicofo |
| `JVB_AUTH_PASSWORD` | yes | — | Video bridge XMPP password. Shared by Prosody and the JVB |
| `JIGASI_XMPP_PASSWORD` | yes | — | Transcriber XMPP password, used for both `JIGASI_XMPP_PASSWORD` and `JIGASI_TRANSCRIBER_PASSWORD` on Prosody and Jigasi. **Not in `.env.example`** |
| `ANTHROPIC_API_KEY` | for summaries | — | Read by `ai-proxy.ts`. Missing means a startup warning and a 503 on every summary request. **Not in `.env.example`** |

Generate the passwords with `openssl rand -hex 16`.

## XMPP domains

Defaults are the standard Jitsi ones and rarely need changing. All are passed through to the
containers by name.

| Variable | Typical value | Notes |
|---|---|---|
| `XMPP_DOMAIN` | `meet.jitsi` | Also the fallback for `PUBLIC_URL` |
| `XMPP_AUTH_DOMAIN` | `auth.meet.jitsi` | |
| `XMPP_GUEST_DOMAIN` | `guest.meet.jitsi` | |
| `XMPP_MUC_DOMAIN` | `muc.meet.jitsi` | A mismatch here breaks calls outright — it was a real bug in this fork's history |
| `XMPP_INTERNAL_MUC_DOMAIN` | `internal-muc.meet.jitsi` | Where the JVB and Jigasi breweries live |
| `XMPP_HIDDEN_DOMAIN` | `hidden.meet.jitsi` | Hardcoded on the Prosody and Jigasi services. The transcriber joins here so it is invisible to participants |
| `XMPP_RECORDER_DOMAIN` | `recorder.meet.jitsi` | |
| `XMPP_SERVER` | `xmpp.meet.jitsi` | Network alias for the Prosody container |
| `XMPP_PORT` | `5222` | |
| `XMPP_BOSH_URL_BASE` | `http://xmpp.meet.jitsi:5280` | |

## Optional

| Variable | Default | What it does |
|---|---|---|
| `JITSI_IMAGE_VERSION` | `stable` | Tag for the `jitsi/prosody`, `jitsi/jicofo`, `jitsi/jvb` and `jitsi/jigasi` images |
| `JVB_PORT` | `10000` | UDP media port, published to the host |
| `JVB_AUTH_USER` | `jvb` | |
| `JVB_BREWERY_MUC` | `jvbbrewery` | |
| `JVB_STUN_SERVERS` | `stun.l.google.com:19302` | The one outbound dependency of the media path |
| `TZ` | — | Container timezone |
| `ENABLE_AUTH` | `0` | `1` requires accounts to create rooms |
| `ENABLE_GUESTS` | `1` | `0` requires accounts to join |
| `AUTH_TYPE` | `internal` | Prosody authentication backend |

`HTTP_PORT` appears in `.env.example` but nothing reads it — `echo-web` uses `expose: "80"` and
is reached through Traefik, not a published port.

## Hardcoded in `docker-compose.yml`

These are not variables. Changing them means editing the Compose file.

| Setting | Value | Service |
|---|---|---|
| `ENABLE_TRANSCRIPTIONS` | `1` | Prosody, Jicofo, Jigasi |
| `ENABLE_BREAKOUT_ROOMS`, `ENABLE_LOBBY`, `ENABLE_END_CONFERENCE`, `ENABLE_XMPP_WEBSOCKET` | `1` | Prosody |
| `ENABLE_RECORDING` | `0` | Jicofo — there is no Jibri in this deployment |
| `ENABLE_AUTO_OWNER`, `ENABLE_SCTP` | `1` | Jicofo |
| `JIGASI_MODE` | `transcriber` | Jigasi. The image defaults to SIP; leaving it unset breaks transcription |
| `JIGASI_TRANSCRIBER_CUSTOM_SERVICE` | `org.jitsi.jigasi.transcription.VoskTranscriptionService` | Jigasi |
| `JIGASI_TRANSCRIBER_VOSK_URL` | `ws://echo-vosk:2700` | Jigasi |
| `JIGASI_TRANSCRIBER_RECORD_AUDIO` | `0` | Jigasi — no audio is written anywhere |
| `JIGASI_TRANSCRIBER_SEND_TXT` | `1` | Jigasi |
| `JIGASI_BREWERY_MUC` | `jigasibrewery` | Jicofo, Jigasi |
| Vosk image | `alphacep/kaldi-fr:latest` | A **French** model. Swap the image for another language |

## `config.js`

Baked into the image as `config.js.template` and rewritten at container start. Facile's
additions on top of upstream:

```js
transcription: {
    enabled: true,
    useAppLanguage: true,
    autoCaptionOnTranscribe: true
},

ai: {
    enabled: true,
    autoSummarize: true,
    provider: 'claude',
    proxyUrl: '/ai/summarize'
},
```

The `ai` block's type lives in `react/features/base/config/configType.ts`:

| Key | Type | What it does |
|---|---|---|
| `enabled` | boolean | Master switch. When false the middleware ignores every action and the toolbar button never appears |
| `autoSummarize` | boolean | Generate automatically on `CONFERENCE_LEFT` instead of only on click |
| `proxyUrl` | string | Where the client POSTs. `/ai/summarize` is proxied by nginx to the sidecar |
| `language` | string | Appended to the prompt as "generate the summary in X" when set and not `en` |
| `provider` | `'claude' \| 'openai' \| 'custom'` | Declared but never branched on in the client |

Also note the peer-to-peer `stunServers` list is a single entry,
`stun:stun.l.google.com:19302`, matching `JVB_STUN_SERVERS`. There is no TURN server in this
deployment, so participants behind symmetric NAT may fail to connect media — and Google's public
STUN server is a third-party dependency of the media path.

## `interface_config.js`

Static, no substitution. The Facile values:

| Key | Value |
|---|---|
| `APP_NAME` | `'Echo'` |
| `PROVIDER_NAME` | `'Facile'` |
| `JITSI_WATERMARK_LINK` | `'https://facile.studio'` |
| `DEFAULT_BACKGROUND` | `'#ffffff'` |
| `AUDIO_LEVEL_PRIMARY_COLOR` | `'rgba(11,11,12,0.4)'` |
| `AUDIO_LEVEL_SECONDARY_COLOR` | `'rgba(11,11,12,0.2)'` |
| `MOBILE_APP_PROMO` | `false` |
| `LANG_DETECTION` | `false` — keeps the browser locale from overriding the French default |
| `SUPPORT_URL` | `''` |

## Build-time variables

Read by the toolchain, not by any container.

| Variable | Default | What it does |
|---|---|---|
| `WEBPACK_DEV_SERVER_PROXY_TARGET` | `https://localhost:8443` | Where `make dev` proxies anything it does not build. Upstream defaulted this to `https://alpha.jitsi.net` |
| `ANALYZE_BUNDLE` | unset | Enables `webpack-bundle-analyzer` |
| `DETECT_CIRCULAR_DEPS` | unset | Enables circular dependency detection |
| `CODESPACES` | unset | Serves the dev server over plain HTTP |

The WebDriverIO suite reads `BASE_URL` (default `https://localhost:8080/`), `MAX_INSTANCES`,
`HEADLESS`, `ALLOW_INSECURE_CERTS`, `RESOLVER_RULES`, `VIDEO_CAPTURE_FILE`, `GRID_HOST_URL` and
`BROWSER_CHROME_BETA`.
