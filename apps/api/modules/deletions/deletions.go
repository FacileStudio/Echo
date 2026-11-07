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
	selectRecordings = `SELECT x.filename FROM recordings x JOIN calls c ON c.id = x.call_id JOIN rooms r ON r.id = c.room_id
		WHERE r.owner_id = @id
		UNION SELECT c.recording_path FROM calls c JOIN rooms r ON r.id = c.room_id
		WHERE r.owner_id = @id AND c.recording_path <> ''`
	deleteOwnedRooms  = `DELETE FROM rooms WHERE owner_id = @id`
	eraseSpeakerLines = `UPDATE transcript_lines l SET speaker = 'erased-' || p.id
		FROM call_participants p WHERE p.call_id = l.call_id AND p.identity = l.speaker AND p.identity = @identity`
	eraseSpeakerText = `UPDATE transcripts t
		SET content = regexp_replace(t.content, '(^|\n)' || p.identity || ': ', '\1erased-' || p.id || ': ', 'g')
		FROM call_participants p WHERE p.call_id = t.call_id AND p.identity = @identity`
	deleteConsents = `DELETE FROM capture_consents WHERE identity = @identity`
	anonymiseSeats = `UPDATE call_participants SET identity = 'erased-' || id, name = '', user_id = NULL
		WHERE user_id = @id OR identity = @identity`
	deleteAccount = `DELETE FROM users WHERE id = @id`
)

// ErasePerson deletes the account behind a Registre identity and what it owns:
// its rooms with their calls, transcripts, transcript lines, summaries,
// recordings, capture answers and participant rows, and the recording files of those calls. A
// stored path is the egress container's absolute /output/<name>.mp4, so only
// its basename is resolved inside the recordings volume, through os.Root so a
// planted symlink cannot leave it. Calls the person joined in rooms other
// people own stay, but their seat loses the account link, the name and the
// user-<id> identity, and the same identity is replaced in the transcript text
// and lines, so nothing in the kept record names them. A subject that is
// already gone is a no-op, so a replayed notice changes nothing.
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
		for _, statement := range []string{deleteOwnedRooms, eraseSpeakerLines, eraseSpeakerText, deleteConsents, anonymiseSeats, deleteAccount} {
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
	root, err := os.OpenRoot(d.recordingsDir)
	if err != nil {
		d.logger.Warn("deletions: recordings volume unavailable", slog.Any("error", err))
		return
	}
	defer root.Close()
	for _, path := range stored {
		if err := root.Remove(filepath.Base(path)); err != nil && !errors.Is(err, os.ErrNotExist) {
			d.logger.Warn("deletions: could not remove an erased person's recording", slog.Any("error", err))
		}
	}
}
