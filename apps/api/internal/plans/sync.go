package plans

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/FacileStudio/porte/directory"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// registreTimeout bounds every snapshot pull.
const registreTimeout = 10 * time.Second

// Credentials are the app's OIDC client credentials, which Registre's
// directory endpoints authenticate.
type Credentials struct {
	Issuer       string
	ClientID     string
	ClientSecret string
}

// NewSyncer builds the directory syncer that feeds Plans. Echo has no spaces,
// so only the snapshot's people and its entitlements revision matter.
func (p *Plans) NewSyncer(creds Credentials) *directory.Syncer {
	return &directory.Syncer{
		Client: directory.Client{
			Issuer:       creds.Issuer,
			ClientID:     creds.ClientID,
			ClientSecret: creds.ClientSecret,
			HTTP:         &http.Client{Timeout: registreTimeout},
		},
		Apply:  p.Apply,
		Logger: p.logger,
	}
}

// Apply hands a snapshot to the store and persists what it now holds, so a
// restart restores it. A store that ignored the snapshot exports the state it
// already had, and rewriting that row is harmless.
func (p *Plans) Apply(ctx context.Context, snapshot directory.Snapshot) error {
	p.store.Apply(snapshot)
	document, err := p.store.Export()
	if err != nil {
		return err
	}
	row := schemas.ForfaitState{ID: schemas.ForfaitStateID, Document: string(document)}
	return p.orm.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"document", "updated_at"}),
	}).Create(&row).Error
}

// errEmptyState is a forfait_state row that exists and holds nothing.
var errEmptyState = errors.New("forfait_state row is empty")

// failClosed is an export with an entitlements revision and no scope, which
// makes every feature read its free default. The next real snapshot, whose
// revision is at least 1, replaces it.
const failClosed = `{"version":1,"snapshot_version":0,"entitlements_revision":1,"spaces":{},"people":{}}`

// Load restores the persisted entitlements at boot, before the first pull.
// No row means Echo never received entitlements and keeps failing open. A row
// that is empty, or that forfait refuses, means entitlements were known once
// and are now unreadable, so the store fails closed on the free defaults
// until a fresh snapshot, rather than handing every owner unlimited calls or
// refusing to boot. forfait itself imports an empty document as a no-op that
// keeps failing open, which is why Echo checks for one before handing it on.
// To go back to failing open on purpose, delete the row; never blank it.
func (p *Plans) Load(ctx context.Context) error {
	var row schemas.ForfaitState
	err := p.orm.WithContext(ctx).Where("id = ?", schemas.ForfaitStateID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	err = errEmptyState
	if strings.TrimSpace(row.Document) != "" {
		err = p.store.Import([]byte(row.Document))
	}
	if err != nil {
		p.logger.Warn("forfait: persisted entitlements refused, enforcing the free defaults until the next snapshot",
			slog.Any("error", err))
		return p.store.Import([]byte(failClosed))
	}
	return nil
}
