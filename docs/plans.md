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

## Call length: 40 minutes per call

The free rule is 40 minutes per call, as in other free video tools. It is not 40 minutes per day.

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
- Echo adds every token it minted in the last 10 minutes that has not connected yet, so a burst of
  tokens cannot get past the cap. Join tokens live 10 minutes.
- Echo creates every LiveKit room itself, with the plan's `max_participants`, before it mints a
  token. If that fails on a capped plan, the join answers 503.

## LiveKit configuration

LiveKit must not create rooms on its own. Otherwise an unused token could recreate a closed room
with no ceiling. Set this in `deploy/compose/livekit.yaml`, and in the production `LIVEKIT_CONFIG`
environment variable of the Echo Media compose:

```yaml
room:
  auto_create: false
```
