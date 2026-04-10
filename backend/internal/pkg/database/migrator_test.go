package database

import (
	"os"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupMigratorDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	return db
}

func setupMigrationsDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("failed to write migration file %s: %v", name, err)
		}
	}
	return dir
}

func TestNewMigrator(t *testing.T) {
	db := setupMigratorDB(t)
	m := NewMigrator(db, "/tmp/migrations")
	if m == nil {
		t.Fatal("expected non-nil migrator")
	}
	if m.db != db {
		t.Error("expected migrator db to match")
	}
	if m.migrationsDir != "/tmp/migrations" {
		t.Error("expected migrator migrationsDir to match")
	}
}

func TestExtractVersion(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"20260410120000_create_users_table.up.sql", "20260410120000_create_users_table"},
		{"20260410120000_create_users_table.down.sql", "20260410120000_create_users_table"},
		{"20260410120001_create_api_keys_table.up.sql", "20260410120001_create_api_keys_table"},
	}
	for _, tt := range tests {
		got := extractVersion(tt.filename)
		if got != tt.want {
			t.Errorf("extractVersion(%q) = %q, want %q", tt.filename, got, tt.want)
		}
	}
}

func TestMigrateUp(t *testing.T) {
	db := setupMigratorDB(t)
	dir := setupMigrationsDir(t, map[string]string{
		"001_create_foo.up.sql":   "CREATE TABLE foo (id INTEGER PRIMARY KEY, name TEXT);",
		"001_create_foo.down.sql": "DROP TABLE IF EXISTS foo;",
		"002_create_bar.up.sql":   "CREATE TABLE bar (id INTEGER PRIMARY KEY, value TEXT);",
		"002_create_bar.down.sql": "DROP TABLE IF EXISTS bar;",
	})

	m := NewMigrator(db, dir)

	if err := m.MigrateUp(); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

	sqlDB, _ := db.DB()
	for _, table := range []string{"foo", "bar"} {
		var count int
		err := sqlDB.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count)
		if err != nil {
			t.Fatalf("failed to check table %s: %v", table, err)
		}
		if count != 1 {
			t.Errorf("expected table %s to exist", table)
		}
	}

	var migrations []SchemaMigration
	db.Find(&migrations)
	if len(migrations) != 2 {
		t.Fatalf("expected 2 migration records, got %d", len(migrations))
	}
	if migrations[0].Version != "001_create_foo" {
		t.Errorf("expected first version 001_create_foo, got %s", migrations[0].Version)
	}
	if migrations[1].Version != "002_create_bar" {
		t.Errorf("expected second version 002_create_bar, got %s", migrations[1].Version)
	}
}

func TestMigrateUpIdempotent(t *testing.T) {
	db := setupMigratorDB(t)
	dir := setupMigrationsDir(t, map[string]string{
		"001_create_foo.up.sql":   "CREATE TABLE foo (id INTEGER PRIMARY KEY);",
		"001_create_foo.down.sql": "DROP TABLE IF EXISTS foo;",
	})

	m := NewMigrator(db, dir)

	if err := m.MigrateUp(); err != nil {
		t.Fatalf("first MigrateUp failed: %v", err)
	}
	if err := m.MigrateUp(); err != nil {
		t.Fatalf("second MigrateUp failed: %v", err)
	}

	var count int64
	db.Model(&SchemaMigration{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 migration record after idempotent run, got %d", count)
	}
}

func TestMigrateDown(t *testing.T) {
	db := setupMigratorDB(t)
	dir := setupMigrationsDir(t, map[string]string{
		"001_create_foo.up.sql":   "CREATE TABLE foo (id INTEGER PRIMARY KEY);",
		"001_create_foo.down.sql": "DROP TABLE IF EXISTS foo;",
		"002_create_bar.up.sql":   "CREATE TABLE bar (id INTEGER PRIMARY KEY);",
		"002_create_bar.down.sql": "DROP TABLE IF EXISTS bar;",
	})

	m := NewMigrator(db, dir)

	if err := m.MigrateUp(); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

	if err := m.MigrateDown(1); err != nil {
		t.Fatalf("MigrateDown(1) failed: %v", err)
	}

	sqlDB, _ := db.DB()
	var barCount int
	sqlDB.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='bar'").Scan(&barCount)
	if barCount != 0 {
		t.Error("expected table bar to be dropped")
	}

	var fooCount int
	sqlDB.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='foo'").Scan(&fooCount)
	if fooCount != 1 {
		t.Error("expected table foo to still exist")
	}

	var migCount int64
	db.Model(&SchemaMigration{}).Count(&migCount)
	if migCount != 1 {
		t.Errorf("expected 1 migration record, got %d", migCount)
	}
}

func TestMigrateDownAll(t *testing.T) {
	db := setupMigratorDB(t)
	dir := setupMigrationsDir(t, map[string]string{
		"001_create_foo.up.sql":   "CREATE TABLE foo (id INTEGER PRIMARY KEY);",
		"001_create_foo.down.sql": "DROP TABLE IF EXISTS foo;",
		"002_create_bar.up.sql":   "CREATE TABLE bar (id INTEGER PRIMARY KEY);",
		"002_create_bar.down.sql": "DROP TABLE IF EXISTS bar;",
	})

	m := NewMigrator(db, dir)

	if err := m.MigrateUp(); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

	if err := m.MigrateDown(2); err != nil {
		t.Fatalf("MigrateDown(2) failed: %v", err)
	}

	var migCount int64
	db.Model(&SchemaMigration{}).Count(&migCount)
	if migCount != 0 {
		t.Errorf("expected 0 migration records, got %d", migCount)
	}
}

func TestMigrateDownNoMigrations(t *testing.T) {
	db := setupMigratorDB(t)
	dir := setupMigrationsDir(t, map[string]string{})

	m := NewMigrator(db, dir)

	if err := m.MigrateDown(1); err != nil {
		t.Fatalf("MigrateDown on empty should not error: %v", err)
	}
}

func TestStatus(t *testing.T) {
	db := setupMigratorDB(t)
	dir := setupMigrationsDir(t, map[string]string{
		"001_create_foo.up.sql":   "CREATE TABLE foo (id INTEGER PRIMARY KEY);",
		"001_create_foo.down.sql": "DROP TABLE IF EXISTS foo;",
	})

	m := NewMigrator(db, dir)

	status, err := m.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if len(status) != 0 {
		t.Errorf("expected 0 migrations, got %d", len(status))
	}

	m.MigrateUp()
	status, err = m.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if len(status) != 1 {
		t.Errorf("expected 1 migration, got %d", len(status))
	}
}

func TestMigrateUpNoFiles(t *testing.T) {
	db := setupMigratorDB(t)
	dir := setupMigrationsDir(t, map[string]string{})

	m := NewMigrator(db, dir)

	if err := m.MigrateUp(); err != nil {
		t.Fatalf("MigrateUp with no files should not error: %v", err)
	}
}

func TestMigrateUpBadSQL(t *testing.T) {
	db := setupMigratorDB(t)
	dir := setupMigrationsDir(t, map[string]string{
		"001_bad.up.sql":   "THIS IS NOT VALID SQL;",
		"001_bad.down.sql": "SELECT 1;",
	})

	m := NewMigrator(db, dir)

	err := m.MigrateUp()
	if err == nil {
		t.Fatal("expected error for bad SQL migration")
	}
}
