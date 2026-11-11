// Package db opens MariaDB connections and runs chassis + service migrations.
package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"
)

//go:embed migrations/*.sql
var chassisFS embed.FS

// Open connects with utf8mb4, parseTime and UTC enforced.
func Open(dsn string) (*sql.DB, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("db: parse dsn: %w", err)
	}
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	cfg.Collation = "utf8mb4_unicode_ci"
	delete(cfg.Params, "charset")
	d, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(20)
	d.SetMaxIdleConns(10)
	d.SetConnMaxLifetime(5 * time.Minute)
	return d, nil
}

// Migrate runs chassis migrations (goose_chassis) then the service's (goose_service).
func Migrate(ctx context.Context, d *sql.DB, serviceFS fs.FS) error {
	sub, err := fs.Sub(chassisFS, "migrations")
	if err != nil {
		return err
	}
	if err := up(ctx, d, sub, "goose_chassis"); err != nil {
		return fmt.Errorf("db: chassis migrations: %w", err)
	}
	if serviceFS == nil {
		return nil
	}
	if err := up(ctx, d, serviceFS, "goose_service"); err != nil {
		return fmt.Errorf("db: service migrations: %w", err)
	}
	return nil
}

func up(ctx context.Context, d *sql.DB, fsys fs.FS, table string) error {
	store, err := database.NewStore(database.DialectMySQL, table)
	if err != nil {
		return err
	}
	p, err := goose.NewProvider("", d, fsys, goose.WithStore(store))
	if err != nil {
		if errors.Is(err, goose.ErrNoMigrations) {
			return nil
		}
		return err
	}
	_, err = p.Up(ctx)
	return err
}
