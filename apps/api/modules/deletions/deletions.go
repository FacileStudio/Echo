package deletions

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/FacileStudio/porte/directory"

	"gorm.io/gorm"
)

// Deletions erases the people Registre erased. It polls Registre's deletion
// list with the same client credentials as the snapshot sync and accepts
// Registre's signed push on directory.DeletionsPath.
type Deletions struct {
	orm           *gorm.DB
	recordingsDir string
	logger        *slog.Logger
	receiver      *directory.Deletions
}

// New builds the receiver over Echo's tables. erase removes porte's own rows,
// pg.Store.EraseIdentity.
func New(
	orm *gorm.DB, client directory.Client, erase func(context.Context, string, string) error,
	recordingsDir string, logger *slog.Logger,
) *Deletions {
	d := &Deletions{orm: orm, recordingsDir: recordingsDir, logger: logger}
	d.receiver = &directory.Deletions{
		Client:          client,
		OnPersonDeleted: d.ErasePerson,
		Erase:           erase,
		State:           state{orm: orm},
		Logger:          logger,
	}
	return d
}

// Run polls until ctx ends. Start it in exactly one replica.
func (d *Deletions) Run(ctx context.Context) {
	d.receiver.Run(ctx)
}

// Handler verifies and applies Registre's signed user.deleted push.
func (d *Deletions) Handler() http.Handler {
	return d.receiver.Handler()
}

const (
	selectRecordings = `SELECT c.recording_path FROM calls c JOIN rooms r ON r.id = c.room_id
		WHERE r.owner_id = @id AND c.recording_path <> ''`
	deleteOwnedRooms = `DELETE FROM rooms WHERE owner_id = @id`
	anonymiseSeats   = `UPDATE call_participants SET name = '' WHERE identity = @identity`
	deleteAccount    = `DELETE FROM users WHERE id = @id`
)

// ErasePerson deletes the account behind a Registre identity and what it owns:
// its rooms with their calls, transcripts, summaries and participant rows, and
// the recordings of those calls when the volume lets Echo remove them. Calls
// the person joined in rooms other people own stay, with the name blanked. A
// subject that is already gone is a no-op, so a replayed notice changes
// nothing.
func (d *Deletions) ErasePerson(ctx context.Context, provider, subject string) error {
	var userID int64
	err := d.orm.WithContext(ctx).Raw(
		`SELECT user_id FROM porte_identities WHERE provider = ? AND subject = ?`, provider, subject,
	).Scan(&userID).Error
	if err != nil || userID == 0 {
		return err
	}
	var recordings []string
	err = d.orm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		args := map[string]any{"id": userID, "identity": "user-" + itoa(userID)}
		if err := tx.Raw(selectRecordings, args).Scan(&recordings).Error; err != nil {
			return err
		}
		for _, statement := range []string{anonymiseSeats, deleteOwnedRooms, deleteAccount} {
			if err := tx.Exec(statement, args).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err == nil {
		d.removeRecordings(recordings)
	}
	return err
}

func (d *Deletions) removeRecordings(stored []string) {
	for _, relative := range stored {
		path := filepath.Join(d.recordingsDir, filepath.Clean("/"+relative))
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			d.logger.Warn("deletions: could not remove an erased person's recording", slog.Any("error", err))
		}
	}
}
