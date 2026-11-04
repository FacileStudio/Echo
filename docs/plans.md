# Plans

Echo enforces the plan the room owner pays for. Ardoise computes each entitlement and pushes it to
Registre. Echo pulls it with porte/forfait and never recomputes a plan.

| Feature | Free | Pro | Studio | Enforced at |
|---|---|---|---|---|
| `echo.max_participants` | 4 | 25 | 100 | Join token (`POST /api/rooms/{slug}/token`) and LiveKit room ceiling |
| `echo.max_call_minutes` | 40 | unlimited | unlimited | Server timer that closes the LiveKit room |
| `echo.recording` | off | on | on | `POST /api/rooms/{slug}/record/start` |

A refusal answers HTTP 402 with `{"error":{"code":"payment_required","message","key","limit","upgrade_url"}}`.

## Whose plan

- A room reads the plan of its owner's Registre subject, which is the `porte_identities` row
  stored under `OIDC_ISSUER` at the owner's last SSO login.
- A room without an owner, or whose owner never signed in through Registre, reads the free plan.
- Integration rooms are unlimited. See `docs/integrations.md`.
- Echo enforces no limit without `OIDC_ISSUER`, or before Registre has pushed entitlements once.
  After that, Echo keeps the last values in the `forfait_state` row. If that row cannot be read at
  boot, Echo enforces the free plan until the next snapshot arrives.

## Call length: 40 minutes per meeting

The free rule is 40 minutes per meeting, as in other free video tools. It is not 40 minutes per
day. This is the owner's decision.

- A call starts when LiveKit creates its room. Echo reads that creation time, or the call row's
  `started_at`, never a time the client sends.
- At the deadline the server closes the room for everyone. The client counts down from
  `call_deadline` in the token response and warns 5 minutes before the end.
- After a room is cut at its deadline, nobody can start a new call in it for 2 minutes.
- Timers are re-armed from open calls at boot. Every 5 minutes Echo also compares its open calls
  with LiveKit's room list. That arms sessions no webhook reported, and closes call rows LiveKit
  no longer runs.

## Participants

- Echo counts the people LiveKit lists in the room. When LiveKit cannot be reached, it counts the
  webhook-fed participant rows instead.
- Echo adds every token it minted in the last 60 seconds that has not connected yet, so a burst of
  tokens cannot get past the cap. The 60-second window is shorter than the token's 10-minute
  lifetime on purpose: anyone can ask for a guest token, so a longer hold would let a few anonymous
  requests lock the room. A token used after the window still meets LiveKit's `max_participants`.
- Echo's count never refuses the signed-in room owner. LiveKit's ceiling still applies to them.
  A plan with a limit of 0 refuses everyone, the owner included, with a 402.
- Echo creates every LiveKit room itself, with the plan's `max_participants`, only for a join it
  admitted. If that fails, the join answers 503 and no token is minted.

## Rate limits

`POST /api/rooms` and `POST /api/rooms/{slug}/token` are the two routes that can create a room.
Both allow 10 requests a minute per client address for guests, and 30 per account for signed-in
users. Above that, they answer 429. This keeps junk slugs from creating unlimited room rows,
LiveKit rooms and deadline timers.

## LiveKit configuration

LiveKit must not create rooms on its own. Otherwise an unused token could recreate a closed room
with no ceiling. Set this in `deploy/compose/livekit.yaml`, and in the production `LIVEKIT_CONFIG`
environment variable of the Echo Media compose:

```yaml
room:
  auto_create: false
```
