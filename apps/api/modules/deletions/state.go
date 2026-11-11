package deletions

import (
	"context"
	"database/sql"
	"errors"
)

// state keeps the receiver's cursor and handled subjects in one row.
type state struct {
	db *sql.DB
}

// Load returns the saved state, or nil before the first save.
func (s state) Load(ctx context.Context) ([]byte, error) {
	var saved []byte
	err := s.db.QueryRowContext(ctx, `SELECT state FROM directory_deletion_state WHERE id = 1`).Scan(&saved)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return saved, err
}

// Save replaces the saved state.
func (s state) Save(ctx context.Context, state []byte) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO directory_deletion_state (id, state, updated_at) VALUES (1, $1, now())
		 ON CONFLICT (id) DO UPDATE SET state = EXCLUDED.state, updated_at = now()`, state)
	return err
}
