// Package testdb opens a migrated, isolated database for a test.
//
// Postgres is the only database in this suite, tests included: a test on
// SQLite builds a different schema from the same struct tags and then passes,
// proving nothing about the DDL that ships.
package testdb

import (
	"os"
	"strings"
	"testing"

	"github.com/FacileStudio/Echo/apps/api/schemas"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openRaw(t *testing.T, url string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.Open(url), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return db
}

func createSchema(t *testing.T, admin *gorm.DB, name string) {
	t.Helper()
	if err := admin.Exec(`DROP SCHEMA IF EXISTS ` + name + ` CASCADE`).Error; err != nil {
		t.Fatalf("drop the test schema: %v", err)
	}
	if err := admin.Exec(`CREATE SCHEMA ` + name).Error; err != nil {
		t.Fatalf("create the test schema: %v", err)
	}
}

// Open returns a database scoped to a schema of this test's own, or skips
// when ECHO_TEST_DATABASE_URL is unset.
func Open(t *testing.T) *gorm.DB {
	t.Helper()
	url := os.Getenv("ECHO_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("ECHO_TEST_DATABASE_URL is unset")
	}

	name := schemaName(t.Name())
	admin := openRaw(t, url)
	createSchema(t, admin, name)

	db, err := gorm.Open(postgres.Open(withSearchPath(url, name)), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open with the search path: %v", err)
	}
	t.Cleanup(func() {
		if handle, err := db.DB(); err == nil {
			if cerr := handle.Close(); cerr != nil {
				t.Logf("close test db: %v", cerr)
			}
		}
		if err := admin.Exec(`DROP SCHEMA IF EXISTS ` + name + ` CASCADE`).Error; err != nil {
			t.Logf("drop test schema: %v", err)
		}
		if handle, err := admin.DB(); err == nil {
			if cerr := handle.Close(); cerr != nil {
				t.Logf("close admin db: %v", cerr)
			}
		}
	})
	return db
}

// Migrated is Open plus the app's migrations, which is what most tests want.
func Migrated(t *testing.T) *gorm.DB {
	t.Helper()
	db := Open(t)
	if err := schemas.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func withSearchPath(url, schema string) string {
	separator := "?"
	if strings.Contains(url, "?") {
		separator = "&"
	}
	return url + separator + "search_path=" + schema
}

func schemaName(test string) string {
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		default:
			return '_'
		}
	}, test)
	if len(safe) > 40 {
		safe = safe[:40]
	}
	return "test_" + safe
}
