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
  Without `OIDC_ISSUER` the API logs a warning at boot.
  After that, Echo keeps the last values in the `forfait_state` row. If that row is empty or
  cannot be read at boot, Echo enforces the free plan until the next snapshot arrives.
- To recover from a bad row, delete it: `DELETE FROM forfait_state;`. Echo then fails open until
  Registre's next snapshot. Do not blank the document: an empty row is treated as unreadable and
  enforces the free plan.

## Call length: 40 minutes per meeting

The free rule is 40 minutes per meeting, as in other free video tools. It is not 40 minutes per
day. This is the owner's decision.

- A call starts when LiveKit creates its room. Echo reads that creation time, or the call row's
  `started_at`, never a time the client sends.
- At the deadline the server closes the room for everyone. The client counts down from
  `call_deadline` in the token response, polls `GET /api/rooms/{slug}/deadline` every minute so an
  upgrade or a restart reaches the countdown, and warns 5 minutes before the end.
- After a room is cut at its deadline, nobody can start a new call in it for 2 minutes.
- The cut is recorded before the room closes, and a join that was being admitted during the cut
  is refused, so no join can slip a fresh session in between.
- Timers are re-armed from open calls at boot. Every 5 minutes Echo also compares its open calls
  with LiveKit's room list. That arms sessions no webhook reported, and closes call rows LiveKit
  no longer runs. Each closed row is forwarded to an integrating app as a `room_finished`, as if
  LiveKit had sent it.

## Participants

- Echo counts the people LiveKit lists in the room. When LiveKit cannot be reached, it counts the
  webhook-fed participant rows instead.
- Echo adds every token it minted in the last 60 seconds that has not connected yet, so a burst of
  tokens cannot get past the cap. One client address holds at most one such pending seat per room:
  a new token from it replaces its previous one. A token used after the window still meets
  LiveKit's `max_participants`.
- While the owner is not in the call, one seat stays theirs: guests can fill at most `limit - 1`
  seats. Echo's count never refuses the signed-in owner, and LiveKit's ceiling still applies to
  them. A limit of 0 refuses everyone, the owner included, with a 402.
- Echo creates every LiveKit room itself, with the plan's `max_participants`, only for a join it
  admitted. If that fails, the join answers 503 and no token is minted. An unlimited plan sends an
  explicit ceiling of 10,000 rather than 0, so an owner who upgrades mid-call raises the ceiling
  of the running room.

## Rate limits

Every budget is per minute, per client address for guests (an IPv6 caller per /64, plus four times
the budget per /48) and per account for signed-in users. Above it the route answers 429.

| What | Guests | Signed in | Keyed by |
|---|---|---|---|
| Creating a room: `POST /api/rooms`, or a token for a slug that does not exist | 10 | 30 | address |
| Joining an existing room: `POST /api/rooms/{slug}/token` | 60 | 120 | address and room |
| Reading the call deadline: `GET /api/rooms/{slug}/deadline` | 120 | 240 | address and room |

Creation is strict, so junk slugs cannot create unlimited room rows, LiveKit rooms and timers.
Joining is generous, so a whole office behind one NAT can join one call.

## LiveKit configuration

LiveKit must not create rooms on its own. Otherwise an unused token could recreate a closed room
with no ceiling. Set this in `deploy/compose/livekit.yaml`, and in the production `LIVEKIT_CONFIG`
environment variable of the Echo Media compose:

```yaml
room:
  auto_create: false
```
