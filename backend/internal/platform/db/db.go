// Package db opens the SQLite database with one writer connection and a pool of readers.
package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB holds two pools on the same file. SQLite allows one writer at a time, so all writes go
// through Write (a single connection) and reads use Read.
type DB struct {
	Write *sql.DB
	Read  *sql.DB
}

var pragmas = []string{
	"journal_mode(WAL)",
	"synchronous(NORMAL)",
	"foreign_keys(ON)",
	"busy_timeout(5000)",
	"temp_store(MEMORY)",
}

func dsn(path string, readOnly bool) string {
	q := url.Values{}
	for _, p := range pragmas {
		q.Add("_pragma", p)
	}
	if readOnly {
		q.Add("_pragma", "query_only(1)")
	} else {
		// BEGIN IMMEDIATE takes the write lock up front, so a transaction never fails with
		// SQLITE_BUSY halfway through when it upgrades from read to write.
		q.Set("_txlock", "immediate")
	}
	return "file:" + path + "?" + q.Encode()
}

// Open opens (and creates if missing) the database at path. Use ":memory:"-like paths only in
// tests through OpenTemp, because the two pools must share one file.
func Open(ctx context.Context, path string) (*DB, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}
	w, err := sql.Open("sqlite", dsn(path, false))
	if err != nil {
		return nil, err
	}
	w.SetMaxOpenConns(1)
	w.SetMaxIdleConns(1)
	w.SetConnMaxLifetime(0)
	if err := w.PingContext(ctx); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("open writer: %w", err)
	}

	r, err := sql.Open("sqlite", dsn(path, true))
	if err != nil {
		_ = w.Close()
		return nil, err
	}
	r.SetMaxOpenConns(8)
	if err := r.PingContext(ctx); err != nil {
		_ = w.Close()
		_ = r.Close()
		return nil, fmt.Errorf("open reader: %w", err)
	}
	return &DB{Write: w, Read: r}, nil
}

func (d *DB) Close() error {
	return errors.Join(d.Read.Close(), d.Write.Close())
}

// Migrate applies all embedded migrations.
func (d *DB) Migrate(ctx context.Context) error {
	provider, err := goose.NewProvider(goose.DialectSQLite3, d.Write, mustSub(migrationsFS, "migrations"))
	if err != nil {
		return fmt.Errorf("goose provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// Tx runs fn inside a write transaction (BEGIN IMMEDIATE). Never call an exchange or any other
// network service inside fn: the write lock is held for the whole transaction.
func (d *DB) Tx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := d.Write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return errors.Join(err, tx.Rollback())
	}
	return tx.Commit()
}

// Ping checks both pools.
func (d *DB) Ping(ctx context.Context) error {
	return errors.Join(d.Write.PingContext(ctx), d.Read.PingContext(ctx))
}
