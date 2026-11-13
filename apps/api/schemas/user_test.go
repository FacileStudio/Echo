package schemas_test

import (
	"testing"

	"github.com/FacileStudio/Echo/apps/api/internal/testdb"
)

// porte/kit inserts an account naming only the columns it knows, so the
// legacy password_hash column needs a database default. GORM's AutoMigrate
// never added one to an existing column; the baseline carries it.
func TestAnInsertWithoutPasswordHashGetsTheEmptyDefault(t *testing.T) {
	db := testdb.Migrated(t)
	if err := db.Exec(`INSERT INTO users (email, name) VALUES ('kit@example.com', 'Kit')`).Error; err != nil {
		t.Fatalf("an insert without password_hash failed: %v", err)
	}
	var row struct {
		PasswordHash string
		Missing      bool
	}
	err := db.Raw(`SELECT password_hash, created_at IS NULL AS missing FROM users WHERE email = 'kit@example.com'`).Scan(&row).Error
	if err != nil {
		t.Fatal(err)
	}
	if row.PasswordHash != "" || row.Missing {
		t.Fatalf("row = %+v, want an empty password_hash and a created_at", row)
	}
}
