package schemas

import (
	mcpkit "github.com/FacileStudio/tronc/mcp"
	"gorm.io/gorm"
)

// Migrate brings the schema up to date on every boot. Echo is a fresh porte
// adoption: there are no legacy sessions to carry over and no pre-porte
// password hashes on the user row to re-adopt, so the only hand-written
// statements are porte/pg's Schema with its foreign keys repointed at Echo's
// own users table — kept verbatim otherwise, statement for statement,
// including the v0.3.0 re-key of a local identity onto the account id.
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&User{}, &Room{}, &Call{}, &Transcript{}, &Summary{}, &CallParticipant{}, &ForfaitState{}, &Recording{}, &TranscriptLine{}, &CaptureConsent{}); err != nil {
		return err
	}

	statements := []string{
		userDefaults,
		porteSchema,
		porteBindingColumns,
		callIndexes,
		integrationIndexes,
		deletionState,
		historyBackfill,
		mcpkit.PendingSchema,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

// userDefaults gives the legacy password_hash column the default its struct
// tag declares. porte/kit inserts an account naming only the columns it
// knows, and GORM's AutoMigrate never adds a default to a column that exists.
const userDefaults = `ALTER TABLE users ALTER COLUMN password_hash SET DEFAULT '';`

// porteBindingColumns are the two columns porte v0.10.0 added to
// porte_login_codes so a code is bound to the identity that vouched for it.
// Echo keeps porte's DDL in porteSchema and never applies pg.Schema, so porte
// cannot add them. Without them the CLI and mobile logins fail.
const porteBindingColumns = `
ALTER TABLE porte_login_codes
	ADD COLUMN IF NOT EXISTS provider text NOT NULL DEFAULT '',
	ADD COLUMN IF NOT EXISTS subject text NOT NULL DEFAULT '';
`

// deletionState is the one row porte/directory's Deletions receiver keeps its
// cursor and handled subjects in.
const deletionState = `

CREATE TABLE IF NOT EXISTS directory_deletion_state (
	id         integer PRIMARY KEY,
	state      bytea NOT NULL,
	updated_at timestamptz NOT NULL DEFAULT now()
);
`

// callIndexes carries the one index GORM's struct tags cannot express.
//
// The uniqueness that matters is on a real LiveKit session id, not on the
// empty string a row gets from the column default, so the index is partial.
// That is also what makes it safe to add to a database that already holds
// call rows: every pre-existing row carries the empty string and is excluded,
// so the
// statement cannot fail on legacy data. A full unique index would have
// depended on `calls` being empty at deploy time.
const callIndexes = `
CREATE UNIQUE INDEX IF NOT EXISTS calls_livekit_room_sid_key
	ON calls (livekit_room_sid) WHERE livekit_room_sid <> '';
CREATE UNIQUE INDEX IF NOT EXISTS recordings_egress_key
	ON recordings (egress_id) WHERE egress_id <> '';
`

// integrationIndexes makes the integration API idempotent at the database:
// one room per app and external id, enforced only on integration rooms.
const integrationIndexes = `
CREATE UNIQUE INDEX IF NOT EXISTS rooms_integration_external_key
	ON rooms (integration_app, external_id) WHERE integration_app <> '';
`

const porteSchema = `
CREATE TABLE IF NOT EXISTS porte_identities (
	user_id         bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	provider        text NOT NULL,
	subject         text NOT NULL,
	password_hash   text NOT NULL DEFAULT '',
	access_token    text NOT NULL DEFAULT '',
	refresh_token   text NOT NULL DEFAULT '',
	token_expiry    timestamptz,
	roles           jsonb,
	roles_synced_at timestamptz,
	synced_at       timestamptz,
	created_at      timestamptz DEFAULT now(),
	PRIMARY KEY (provider, subject)
);
CREATE INDEX IF NOT EXISTS porte_identities_user_idx ON porte_identities (user_id);
ALTER TABLE porte_identities ADD COLUMN IF NOT EXISTS created_at timestamptz;
ALTER TABLE porte_identities ALTER COLUMN created_at SET DEFAULT now();

UPDATE porte_identities SET subject = user_id::text
 WHERE provider = 'local' AND subject <> user_id::text;

CREATE TABLE IF NOT EXISTS porte_sessions (
	id           bigserial PRIMARY KEY,
	token_hash   text NOT NULL UNIQUE,
	user_id      bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	label        text NOT NULL DEFAULT '',
	created_at   timestamptz NOT NULL DEFAULT now(),
	last_used_at timestamptz NOT NULL DEFAULT now(),
	expires_at   timestamptz
);
CREATE INDEX IF NOT EXISTS porte_sessions_user_idx ON porte_sessions (user_id);
CREATE INDEX IF NOT EXISTS porte_sessions_expiry_idx ON porte_sessions (expires_at)
	WHERE expires_at IS NOT NULL;

CREATE TABLE IF NOT EXISTS porte_login_codes (
	code_hash   text PRIMARY KEY,
	user_id     bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	expires_at  timestamptz NOT NULL,
	consumed_at timestamptz
);
`

// historyBackfill links rows written before the meetings area existed. A seat
// whose identity is user-<id> gets that id as user_id, but only for an account
// that still exists, so an erased person is never relinked. Every recorded
// call gets one recordings row, once. Both statements are no-ops on a second
// boot.
const historyBackfill = `
UPDATE call_participants p SET user_id = substr(p.identity, 6)::bigint
 WHERE p.user_id IS NULL AND p.identity ~ '^user-[0-9]{1,18}$'
   AND EXISTS (SELECT 1 FROM users u WHERE u.id = substr(p.identity, 6)::bigint);

INSERT INTO recordings (id, call_id, egress_id, filename, shared, created_at)
SELECT gen_random_uuid(), c.id, c.egress_id, c.recording_path, false, c.updated_at FROM calls c
 WHERE c.recording_path <> ''
   AND NOT EXISTS (SELECT 1 FROM recordings r WHERE r.call_id = c.id)
ON CONFLICT DO NOTHING;
`
