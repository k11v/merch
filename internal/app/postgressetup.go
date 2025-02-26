package app

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func SetupPostgres(url string) error {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return fmt.Errorf("app.SetupPostgres: %w", err)
	}
	defer db.Close()

	return migratePostgresDB(db)
}

func migratePostgresDB(db *sql.DB) error {
	sourceDriver, err := iofs.New(migrationFS(), ".")
	if err != nil {
		return err
	}
	databaseDriver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return err
	}
	m, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", databaseDriver)
	if err != nil {
		return err
	}
	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

//go:embed migrationdata/*.sql
var migrationEmbedFS embed.FS

func migrationFS() fs.FS {
	sub, err := fs.Sub(migrationEmbedFS, "migrationdata")
	if err != nil {
		panic(err)
	}
	return sub
}
