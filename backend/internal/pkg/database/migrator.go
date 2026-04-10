package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

// SchemaMigration represents a row in the schema_migrations tracking table.
type SchemaMigration struct {
	ID        int       `gorm:"primaryKey;autoIncrement"`
	Version   string    `gorm:"type:varchar(255);uniqueIndex;not null"`
	AppliedAt time.Time `gorm:"not null"`
}

// TableName overrides the default GORM table name.
func (SchemaMigration) TableName() string {
	return "schema_migrations"
}

// Migrator handles running SQL migration files in order and tracking history.
type Migrator struct {
	db            *gorm.DB
	migrationsDir string
}

// NewMigrator creates a new Migrator instance.
func NewMigrator(db *gorm.DB, migrationsDir string) *Migrator {
	return &Migrator{
		db:            db,
		migrationsDir: migrationsDir,
	}
}

// ensureTable creates the schema_migrations table if it does not exist.
func (m *Migrator) ensureTable() error {
	return m.db.AutoMigrate(&SchemaMigration{})
}

// appliedVersions returns a set of already-applied migration versions.
func (m *Migrator) appliedVersions() (map[string]bool, error) {
	var migrations []SchemaMigration
	if err := m.db.Order("version ASC").Find(&migrations).Error; err != nil {
		return nil, fmt.Errorf("failed to query schema_migrations: %w", err)
	}

	applied := make(map[string]bool, len(migrations))
	for _, mg := range migrations {
		applied[mg.Version] = true
	}
	return applied, nil
}

// MigrateUp reads all .up.sql files sorted by timestamp, executes ones not yet
// applied, and records each in the schema_migrations table. Each migration runs
// in its own database transaction.
func (m *Migrator) MigrateUp() error {
	if err := m.ensureTable(); err != nil {
		return fmt.Errorf("failed to ensure schema_migrations table: %w", err)
	}

	applied, err := m.appliedVersions()
	if err != nil {
		return err
	}

	files, err := m.listFiles(".up.sql")
	if err != nil {
		return err
	}

	if len(files) == 0 {
		slog.Info("no migration files found", "dir", m.migrationsDir)
		return nil
	}

	sqlDB, err := m.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	count := 0
	for _, f := range files {
		version := extractVersion(f)
		if applied[version] {
			continue
		}

		slog.Info("applying migration", "version", version, "file", f)

		content, err := os.ReadFile(filepath.Join(m.migrationsDir, f))
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", f, err)
		}

		if err := m.execInTransaction(sqlDB, string(content)); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", version, err)
		}

		// Record the migration in schema_migrations.
		if err := m.db.Create(&SchemaMigration{
			Version:   version,
			AppliedAt: time.Now(),
		}).Error; err != nil {
			return fmt.Errorf("failed to record migration %s: %w", version, err)
		}

		slog.Info("migration applied", "version", version)
		count++
	}

	if count == 0 {
		slog.Info("database is up to date")
	} else {
		slog.Info("migrations complete", "applied", count)
	}

	return nil
}

// MigrateDown rolls back the last N applied migrations by executing their
// corresponding .down.sql files in reverse chronological order.
func (m *Migrator) MigrateDown(steps int) error {
	if err := m.ensureTable(); err != nil {
		return fmt.Errorf("failed to ensure schema_migrations table: %w", err)
	}

	// Fetch the last N applied migrations in reverse order.
	var migrations []SchemaMigration
	if err := m.db.Order("version DESC").Limit(steps).Find(&migrations).Error; err != nil {
		return fmt.Errorf("failed to query schema_migrations: %w", err)
	}

	if len(migrations) == 0 {
		slog.Info("no migrations to roll back")
		return nil
	}

	sqlDB, err := m.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	for _, mg := range migrations {
		downFile, err := m.findDownFile(mg.Version)
		if err != nil {
			return err
		}

		slog.Info("rolling back migration", "version", mg.Version, "file", downFile)

		content, err := os.ReadFile(filepath.Join(m.migrationsDir, downFile))
		if err != nil {
			return fmt.Errorf("failed to read down migration file %s: %w", downFile, err)
		}

		if err := m.execInTransaction(sqlDB, string(content)); err != nil {
			return fmt.Errorf("failed to roll back migration %s: %w", mg.Version, err)
		}

		// Remove the migration record.
		if err := m.db.Where("version = ?", mg.Version).Delete(&SchemaMigration{}).Error; err != nil {
			return fmt.Errorf("failed to remove migration record %s: %w", mg.Version, err)
		}

		slog.Info("migration rolled back", "version", mg.Version)
	}

	slog.Info("rollback complete", "rolled_back", len(migrations))
	return nil
}

// Status returns the list of applied migrations.
func (m *Migrator) Status() ([]SchemaMigration, error) {
	if err := m.ensureTable(); err != nil {
		return nil, fmt.Errorf("failed to ensure schema_migrations table: %w", err)
	}

	var migrations []SchemaMigration
	if err := m.db.Order("version ASC").Find(&migrations).Error; err != nil {
		return nil, fmt.Errorf("failed to query schema_migrations: %w", err)
	}
	return migrations, nil
}

// execInTransaction runs raw SQL within a database transaction.
func (m *Migrator) execInTransaction(sqlDB *sql.DB, query string) error {
	tx, err := sqlDB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			panic(r)
		}
	}()

	if _, err := tx.Exec(query); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to execute SQL: %w", err)
	}

	return tx.Commit()
}

// listFiles returns migration files matching the given suffix, sorted by name.
func (m *Migrator) listFiles(suffix string) ([]string, error) {
	entries, err := os.ReadDir(m.migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory %s: %w", m.migrationsDir, err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), suffix) {
			files = append(files, e.Name())
		}
	}

	sort.Strings(files)
	return files, nil
}

// findDownFile locates the .down.sql file for a given migration version.
func (m *Migrator) findDownFile(version string) (string, error) {
	files, err := m.listFiles(".down.sql")
	if err != nil {
		return "", err
	}

	for _, f := range files {
		if extractVersion(f) == version {
			return f, nil
		}
	}

	return "", fmt.Errorf("down migration file not found for version %s", version)
}

// extractVersion extracts the version (timestamp_description) from a migration
// filename. For example, "20260410120000_create_users_table.up.sql" returns
// "20260410120000_create_users_table".
func extractVersion(filename string) string {
	name := filename
	name = strings.TrimSuffix(name, ".up.sql")
	name = strings.TrimSuffix(name, ".down.sql")
	return name
}
