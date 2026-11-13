-- +goose Up
-- GORM fills these timestamps itself. A row written outside GORM, porte/kit's sign-up insert first among them, now gets them from the database instead of NULL.

ALTER TABLE calls
    ALTER COLUMN created_at SET DEFAULT now(),
    ALTER COLUMN updated_at SET DEFAULT now();
ALTER TABLE capture_consents ALTER COLUMN updated_at SET DEFAULT now();
ALTER TABLE forfait_state ALTER COLUMN updated_at SET DEFAULT now();
ALTER TABLE recordings
    ALTER COLUMN created_at SET DEFAULT now(),
    ALTER COLUMN updated_at SET DEFAULT now();
ALTER TABLE rooms
    ALTER COLUMN created_at SET DEFAULT now(),
    ALTER COLUMN updated_at SET DEFAULT now();
ALTER TABLE summaries
    ALTER COLUMN created_at SET DEFAULT now(),
    ALTER COLUMN updated_at SET DEFAULT now();
ALTER TABLE transcripts
    ALTER COLUMN created_at SET DEFAULT now(),
    ALTER COLUMN updated_at SET DEFAULT now();
ALTER TABLE users ALTER COLUMN created_at SET DEFAULT now();

-- +goose Down
ALTER TABLE users ALTER COLUMN created_at DROP DEFAULT;
ALTER TABLE transcripts
    ALTER COLUMN updated_at DROP DEFAULT,
    ALTER COLUMN created_at DROP DEFAULT;
ALTER TABLE summaries
    ALTER COLUMN updated_at DROP DEFAULT,
    ALTER COLUMN created_at DROP DEFAULT;
ALTER TABLE rooms
    ALTER COLUMN updated_at DROP DEFAULT,
    ALTER COLUMN created_at DROP DEFAULT;
ALTER TABLE recordings
    ALTER COLUMN updated_at DROP DEFAULT,
    ALTER COLUMN created_at DROP DEFAULT;
ALTER TABLE forfait_state ALTER COLUMN updated_at DROP DEFAULT;
ALTER TABLE capture_consents ALTER COLUMN updated_at DROP DEFAULT;
ALTER TABLE calls
    ALTER COLUMN updated_at DROP DEFAULT,
    ALTER COLUMN created_at DROP DEFAULT;
