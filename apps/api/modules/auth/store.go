package auth

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/FacileStudio/porte/kit"
)

// Columns names Echo's users table for porte/kit: the picture lives in
// picture and the first account gets is_admin. created_at has no database
// default, so the kit adopts and fills it.
func Columns() kit.Columns {
	return kit.Columns{Picture: "picture", Admin: "is_admin"}
}

// NewStore is porte/kit's account store over Echo's users table. It resolves
// a Registre sign-in to an account, hydrates the request identity and, with
// eraseData, erases an account's data before the kit deletes the account.
func NewStore(
	ctx context.Context, sqlDB *sql.DB, log *slog.Logger, eraseData func(context.Context, *sql.Tx, int64) error,
) (*kit.Store, error) {
	return kit.NewStore(ctx, kit.Store{DB: sqlDB, Columns: Columns(), Logger: log, EraseData: eraseData})
}
