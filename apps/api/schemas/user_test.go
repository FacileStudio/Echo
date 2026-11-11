package schemas_test

import (
	"testing"

	"github.com/FacileStudio/Echo/apps/api/internal/testdb"
	"github.com/FacileStudio/Echo/apps/api/schemas"
)

// porte/kit inserts an account naming only the columns it knows, so the
// legacy password_hash column needs a database default, on a fresh database
// and on one created before the default existed.
func TestPasswordHashGetsADefaultOnAnExistingTable(t *testing.T) {
	db := testdb.Migrated(t)
	if err := db.Exec(`ALTER TABLE users ALTER COLUMN password_hash DROP DEFAULT`).Error; err != nil {
		t.Fatal(err)
	}
	if err := schemas.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Exec(`INSERT INTO users (email) VALUES ('kit@example.com')`).Error; err != nil {
		t.Fatalf("an insert without password_hash failed: %v", err)
	}
}
