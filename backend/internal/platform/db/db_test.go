package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"bigwaves/internal/platform/db"
)

func TestOpenMigrateAndPragmas(t *testing.T) {
	ctx := context.Background()
	d, err := db.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	require.NoError(t, d.Migrate(ctx))
	require.NoError(t, d.Migrate(ctx), "migrate is idempotent")

	var mode string
	require.NoError(t, d.Write.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode))
	require.Equal(t, "wal", mode)

	var fk int
	require.NoError(t, d.Read.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk))
	require.Equal(t, 1, fk)

	_, err = d.Read.ExecContext(ctx, "INSERT INTO users(email) VALUES ('x@example.com')")
	require.Error(t, err, "reader pool must be read-only")

	_, err = d.Write.ExecContext(ctx, "INSERT INTO users(email) VALUES ('x@example.com')")
	require.NoError(t, err)
}
