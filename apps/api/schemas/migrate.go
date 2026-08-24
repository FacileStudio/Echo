package schemas

import "gorm.io/gorm"

// Migrate brings the schema up to date on every boot. Echo is a fresh porte
// adoption: there are no legacy sessions to carry over and no pre-porte
// password hashes on the user row to re-adopt, so the only hand-written
// statements are porte/pg's Schema with its foreign keys repointed at Echo's
// own users table — kept verbatim otherwise, statement for statement,
// including the v0.3.0 re-key of a local identity onto the account id.
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&User{}, &Room{}, &Call{}, &Transcript{}, &Summary{}, &CallParticipant{}); err != nil {
		return err
	}

	statements := []string{
		porteSchema,
		callIndexes,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

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
