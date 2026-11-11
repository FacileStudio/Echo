package deletions

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/FacileStudio/porte/directory"
	"github.com/FacileStudio/porte/kit"
)

// Deletions erases the people Registre erased. It polls Registre's deletion
// list with the same client credentials as the snapshot sync and accepts
// Registre's signed push on directory.DeletionsPath.
type Deletions struct {
	users         *kit.Store
	recordingsDir string
	logger        *slog.Logger
	receiver      *directory.Deletions
}

// New builds the receiver over Echo's tables. users is the kit store built
// with EraseData; erase removes porte's own rows, pg.Store.EraseIdentity.
func New(
	users *kit.Store, client directory.Client, erase func(context.Context, string, string) error,
	recordingsDir string, logger *slog.Logger,
) *Deletions {
	d := &Deletions{users: users, recordingsDir: recordingsDir, logger: logger}
	d.receiver = &directory.Deletions{
		Client:          client,
		OnPersonDeleted: d.ErasePerson,
		Erase:           erase,
		State:           state{db: users.DB},
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

// ErasePerson deletes the account behind a Registre identity through the kit
// store, which runs EraseData and deletes the users row in one transaction,
// then removes the recording files of the deleted calls. A stored path is
// the egress container's absolute /output/<name>.mp4, so only its basename
// is resolved inside the recordings volume, through os.Root so a planted
// symlink cannot leave it. A subject that is already gone is a no-op, so a
// replayed notice changes nothing.
func (d *Deletions) ErasePerson(ctx context.Context, provider, subject string) error {
	var files []string
	if err := d.users.EraseAccount(context.WithValue(ctx, filesKey{}, &files), provider, subject); err != nil {
		return err
	}
	d.removeRecordings(files)
	return nil
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
