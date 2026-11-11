# Erasure

When Registre erases a person, Echo deletes what that person owns and strips them from what
they shared. This page is the rule. The code is `apps/api/modules/deletions`.

## How a notice arrives

Registre lists erased people at `GET /api/directory/deletions`. Echo polls that list with the
OIDC client credentials (`porte/directory` `Deletions.Run`, one replica). Registre can also push
a signed notice to `POST /api/registre/deletions` (`directory.DeletionsPath`, mounted at the root
before the `/api` group).
Both paths run the same erasure once per person. Registre holds a self-deletion for 30 days
before it notifies apps.

Without `OIDC_ISSUER` there is no Registre and no receiver.

## What happens to each table

The erasure runs in one transaction under porte/kit's lock: `kit.Store.EraseAccount` resolves the
account from `porte_identities` (with or without a trailing slash on the issuer), runs
`deletions.EraseData`, then deletes the `users` row. porte's `EraseIdentity` removes the identity
row afterwards.

| Data | Fate |
|---|---|
| `users` row (email, name, picture, locale) | deleted |
| `porte_identities`, `porte_sessions`, `porte_login_codes` | deleted (cascade from `users`, then `EraseIdentity`) |
| Rooms the person owns | deleted, with their calls, transcripts, transcript lines, summaries, recordings rows, capture answers and seats |
| Recording files of those calls | removed from the recordings volume after the commit, by basename, through `os.Root` |
| Their seat in a call in someone else's room | kept, with `identity` set to `erased-<seat id>`, `name` emptied, `user_id` cleared |
| Their lines in that call | `speaker` set to `erased-<seat id>` |
| Their `user-<id>` in that call's transcript text and AI summary | replaced by `erased-<seat id>` |
| Their capture answers in that call | deleted |

A call in someone else's room stays: it is the other people's record. Nothing kept names the
erased person.

## What is not erased

- What they said in a call they joined stays in the transcript and summary, under
  `erased-<seat id>`. Words that name them inside the text cannot be found mechanically.
- Guest seats and integration seats (`guest-...`, `<app>#...`) carry no Registre link, so a
  deletion notice cannot reach them.
- `forfait_state.document` caches Registre's plan snapshot, which keys a person's entitlements by
  subject (no email or name). The next snapshot from Registre replaces it.
- `directory_deletion_state.state` keeps the subjects already handled, for 60 days, so a
  replayed notice is a no-op. That is the receiver's own record, by design.
- Logs are outside the database and follow their own retention.

## The test

`modules/deletions/scan_test.go` seeds two people with rows in every table above, sends the
erasure through the receiver's poll against a fake Registre, then scans every text, varchar,
json and bytea column in the schema for the erased person's email, name, subject and `user-<id>`
identity. Any match fails the test. The other person's values must still be found. The only
table the scan skips is `directory_deletion_state`.

Add a table that stores a user id, an identity or a name, and this test is where you prove the
erasure reaches it.
