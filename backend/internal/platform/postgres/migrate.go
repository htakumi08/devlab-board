// Package postgres provides PostgreSQL-specific platform concerns shared by application features.
package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

const migrationLockID int64 = 724016849341

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrate applies immutable, numbered SQL migrations once per database.
func Migrate(ctx context.Context, db *sql.DB) error {
	connection, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("get migration connection: %w", err)
	}
	defer connection.Close()

	if _, err := connection.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, migrationLockID); err != nil {
		return fmt.Errorf("lock migrations: %w", err)
	}
	defer connection.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, migrationLockID)

	if _, err := connection.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			checksum TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	migrations, err := loadMigrations()
	if err != nil {
		return err
	}
	for _, migration := range migrations {
		if err := applyMigration(ctx, connection, migration); err != nil {
			return err
		}
	}
	return nil
}

type migration struct {
	version  int64
	name     string
	checksum string
	sql      string
}

func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}

	result := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		versionText, _, ok := strings.Cut(entry.Name(), "_")
		if !ok {
			return nil, fmt.Errorf("migration file %q must start with a numeric version", entry.Name())
		}
		version, err := strconv.ParseInt(versionText, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse migration version %q: %w", entry.Name(), err)
		}
		content, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", entry.Name(), err)
		}
		digest := sha256.Sum256(content)
		result = append(result, migration{
			version:  version,
			name:     entry.Name(),
			checksum: hex.EncodeToString(digest[:]),
			sql:      string(content),
		})
	}

	sort.Slice(result, func(i, j int) bool { return result[i].version < result[j].version })
	for index := 1; index < len(result); index++ {
		if result[index-1].version == result[index].version {
			return nil, fmt.Errorf("duplicate migration version %d", result[index].version)
		}
	}
	return result, nil
}

func applyMigration(ctx context.Context, connection *sql.Conn, migration migration) error {
	var appliedChecksum string
	err := connection.QueryRowContext(ctx, `
		SELECT checksum
		FROM schema_migrations
		WHERE version = $1
	`, migration.version).Scan(&appliedChecksum)
	if err == nil {
		if appliedChecksum != migration.checksum {
			return fmt.Errorf("migration %d checksum changed after application", migration.version)
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read migration %d: %w", migration.version, err)
	}

	transaction, err := connection.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", migration.version, err)
	}
	defer transaction.Rollback()

	if _, err := transaction.ExecContext(ctx, migration.sql); err != nil {
		return fmt.Errorf("apply migration %d: %w", migration.version, err)
	}
	if _, err := transaction.ExecContext(ctx, `
		INSERT INTO schema_migrations (version, name, checksum)
		VALUES ($1, $2, $3)
	`, migration.version, migration.name, migration.checksum); err != nil {
		return fmt.Errorf("record migration %d: %w", migration.version, err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit migration %d: %w", migration.version, err)
	}
	return nil
}
