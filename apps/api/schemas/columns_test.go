package schemas_test

import (
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/testdb"
	"github.com/FacileStudio/Echo/apps/api/schemas"
	portepg "github.com/FacileStudio/porte/pg"
	mcpkit "github.com/FacileStudio/tronc/mcp"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var queryModels = []any{
	&schemas.User{}, &schemas.Room{}, &schemas.Call{}, &schemas.Transcript{}, &schemas.Summary{},
	&schemas.CallParticipant{}, &schemas.ForfaitState{}, &schemas.Recording{}, &schemas.TranscriptLine{},
	&schemas.CaptureConsent{},
}

var (
	timeType = reflect.TypeFor[time.Time]()
	uuidType = reflect.TypeFor[uuid.UUID]()
)

func compatibleTypes(field reflect.Type) []string {
	for field.Kind() == reflect.Pointer {
		field = field.Elem()
	}
	switch field {
	case timeType:
		return []string{"timestamp with time zone"}
	case uuidType:
		return []string{"uuid"}
	}
	switch field.Kind() {
	case reflect.String:
		return []string{"text", "character varying"}
	case reflect.Bool:
		return []string{"boolean"}
	case reflect.Int, reflect.Int64, reflect.Uint, reflect.Uint64:
		return []string{"bigint"}
	case reflect.Int32, reflect.Uint32:
		return []string{"integer", "bigint"}
	default:
		return nil
	}
}

func migratedColumnTypes(t *testing.T, db *gorm.DB, table string) map[string]string {
	t.Helper()
	var rows []struct {
		ColumnName string
		DataType   string
	}
	err := db.Raw(`SELECT column_name, data_type FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = ?`, table).Scan(&rows).Error
	if err != nil {
		t.Fatalf("read the columns of %s: %v", table, err)
	}
	columns := map[string]string{}
	for _, row := range rows {
		columns[row.ColumnName] = row.DataType
	}
	return columns
}

// Every column a GORM model reads or writes exists in the schema the
// migrations build, with a type the field can hold. AutoMigrate no longer adds
// a missing column, so a field added to a struct without a migration fails
// here instead of at query time.
func TestGORMModelsMatchTheMigratedSchema(t *testing.T) {
	db := testdb.Migrated(t)
	for _, model := range queryModels {
		parsed, err := schema.Parse(model, &sync.Map{}, db.NamingStrategy)
		if err != nil {
			t.Fatalf("parse %T: %v", model, err)
		}
		columns := migratedColumnTypes(t, db, parsed.Table)
		if len(columns) == 0 {
			t.Errorf("%T: table %s does not exist", model, parsed.Table)
			continue
		}
		for _, field := range parsed.Fields {
			if field.DBName == "" {
				continue
			}
			got, ok := columns[field.DBName]
			if !ok {
				t.Errorf("%s.%s: no such column", parsed.Table, field.DBName)
				continue
			}
			if want := compatibleTypes(field.FieldType); !slices.Contains(want, got) {
				t.Errorf("%s.%s: column is %s, field %s needs one of %v", parsed.Table, field.DBName, got, field.FieldType, want)
			}
		}
	}
}

func tableShape(t *testing.T, db *gorm.DB, table string) []string {
	t.Helper()
	var shape []string
	err := db.Raw(`SELECT column_name || ' ' || data_type || ' ' || is_nullable || ' '
			|| replace(coalesce(column_default, ''), table_schema || '.', '')
		FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = ?
		UNION ALL SELECT replace(indexdef, schemaname || '.', '')
		FROM pg_indexes WHERE schemaname = current_schema() AND tablename = ?
		ORDER BY 1`, table, table).Scan(&shape).Error
	if err != nil {
		t.Fatalf("read the shape of %s: %v", table, err)
	}
	if len(shape) == 0 {
		t.Fatalf("table %s does not exist", table)
	}
	return shape
}

// The migrations hold tronc/mcp's PendingSchema, which Echo used to execute
// at every boot. A tronc bump that changes it makes this fail, and the change
// then needs a numbered migration of its own.
func TestMCPPendingSchemaIsInTheMigrations(t *testing.T) {
	db := testdb.Migrated(t)
	before := tableShape(t, db, "mcp_actions")
	if err := db.Exec(mcpkit.PendingSchema).Error; err != nil {
		t.Fatalf("run PendingSchema: %v", err)
	}
	if after := tableShape(t, db, "mcp_actions"); !slices.Equal(before, after) {
		t.Fatalf("PendingSchema changed mcp_actions:\nbefore %v\nafter  %v", before, after)
	}
}

// Echo keeps porte's three tables in the baseline, with their foreign keys
// on its own users table instead of porte_users. Every column, default and
// index still matches what porte/pg's Schema builds, so a porte bump that
// changes one fails here and needs a numbered migration of its own.
func TestPorteTablesMatchPorteSchema(t *testing.T) {
	migrated := testdb.Migrated(t)
	t.Run("pg", func(t *testing.T) {
		library := testdb.Open(t)
		if err := library.Exec(portepg.Schema).Error; err != nil {
			t.Fatalf("run porte's Schema: %v", err)
		}
		for _, table := range []string{"porte_identities", "porte_sessions", "porte_login_codes"} {
			want, got := tableShape(t, library, table), tableShape(t, migrated, table)
			if !slices.Equal(want, got) {
				t.Errorf("%s differs from porte's Schema:\nporte %v\necho  %v", table, want, got)
			}
		}
	})
}
