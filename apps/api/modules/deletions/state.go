package deletions

import (
	"context"
	"strconv"

	"gorm.io/gorm"
)

func itoa(id int64) string {
	return strconv.FormatInt(id, 10)
}

// state keeps the receiver's cursor and handled subjects in one row.
type state struct {
	orm *gorm.DB
}

// Load returns the saved state, or nil before the first save.
func (s state) Load(ctx context.Context) ([]byte, error) {
	var rows []struct{ State []byte }
	err := s.orm.WithContext(ctx).Raw(`SELECT state FROM directory_deletion_state WHERE id = 1`).Scan(&rows).Error
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0].State, nil
}

// Save replaces the saved state.
func (s state) Save(ctx context.Context, state []byte) error {
	return s.orm.WithContext(ctx).Exec(
		`INSERT INTO directory_deletion_state (id, state, updated_at) VALUES (1, ?, now())
		 ON CONFLICT (id) DO UPDATE SET state = EXCLUDED.state, updated_at = now()`, state).Error
}
