package bus

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/FacileStudio/antenne-client/go/envelope"
	"github.com/google/uuid"
)

// load reads the call as the contract carries it, or nil for a call in an
// integration room.
func (o *Outbox) load(ctx context.Context, tx *sql.Tx, callID uuid.UUID) (*envelope.Call, error) {
	var (
		call        envelope.Call
		integration string
		started     time.Time
		ended       sql.NullTime
	)
	err := tx.QueryRowContext(ctx, `SELECT r.slug, r.name, r.integration_app, c.started_at, c.ended_at
		FROM calls c JOIN rooms r ON r.id = c.room_id WHERE c.id = $1`, callID,
	).Scan(&call.RoomSlug, &call.RoomName, &integration, &started, &ended)
	if errors.Is(err, sql.ErrNoRows) || integration != "" {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("bus: read the call: %w", err)
	}
	call.FacileID = callID.String()
	call.StartedAt = started.UTC().Format(time.RFC3339)
	if ended.Valid {
		at := ended.Time.UTC().Format(time.RFC3339)
		call.EndedAt = &at
	}
	if call.ParticipantEmails, err = participantEmails(ctx, tx, callID); err != nil {
		return nil, err
	}
	if o.publicURL != "" {
		call.URL = o.publicURL + "/calls/" + call.FacileID
	}
	return &call, nil
}

// participantEmails lists the signed-in people who sat in the call, lowercase
// and sorted. Guests have no account and so no row to join.
func participantEmails(ctx context.Context, tx *sql.Tx, callID uuid.UUID) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT lower(u.email) FROM call_participants p
		JOIN users u ON u.id = p.user_id WHERE p.call_id = $1 AND u.email <> '' ORDER BY 1`, callID)
	if err != nil {
		return nil, fmt.Errorf("bus: read the participants: %w", err)
	}
	defer rows.Close()
	emails := []string{}
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, fmt.Errorf("bus: read the participants: %w", err)
		}
		emails = append(emails, email)
	}
	return emails, rows.Err()
}
