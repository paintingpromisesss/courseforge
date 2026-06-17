package repo

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func RunMigrations(path string) error {
	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create source driver: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		sourceDriver.Close()
		return fmt.Errorf("failed to open migration database: %w", err)
	}

	databaseDriver, err := migratesqlite.WithInstance(db, &migratesqlite.Config{})
	if err != nil {
		sourceDriver.Close()
		db.Close()
		return fmt.Errorf("failed to create migration database driver: %w", err)
	}

	migrateInstance, err := migrate.NewWithInstance("iofs", sourceDriver, "sqlite", databaseDriver)
	if err != nil {
		sourceDriver.Close()
		databaseDriver.Close()
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	defer migrateInstance.Close()

	if err := migrateInstance.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration failed: %w", err)
	}

	return nil
}
