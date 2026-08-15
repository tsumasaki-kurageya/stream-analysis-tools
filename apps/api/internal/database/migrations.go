package database

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const migrationLockID int64 = 7_627_333_579_928_516_940

type migration struct {
	name string
	sql  string
}

// ApplyMigrations applies each ordered *.up.sql migration once.
func ApplyMigrations(ctx context.Context, pool *pgxpool.Pool, migrationFS fs.FS) error {
	migrations, err := loadMigrations(migrationFS)
	if err != nil {
		return err
	}
	if len(migrations) == 0 {
		return errors.New("no up migrations found")
	}

	connection, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer connection.Release()

	if _, err := connection.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockID); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		_, _ = connection.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLockID)
	}()

	if _, err := connection.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS public.schema_migrations (
			name TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("create migration ledger: %w", err)
	}

	for _, item := range migrations {
		if err := applyMigration(ctx, connection, item); err != nil {
			return err
		}
	}

	return nil
}

func loadMigrations(migrationFS fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(migrationFS, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}

	var migrations []migration
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}

		contents, err := fs.ReadFile(migrationFS, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		migrations = append(migrations, migration{name: entry.Name(), sql: string(contents)})
	}

	sort.Slice(migrations, func(left int, right int) bool {
		return migrations[left].name < migrations[right].name
	})
	return migrations, nil
}

type migrationConnection interface {
	Begin(context.Context) (pgx.Tx, error)
}

func applyMigration(ctx context.Context, connection migrationConnection, item migration) error {
	transaction, err := connection.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", item.name, err)
	}
	defer func() { _ = transaction.Rollback(context.Background()) }()

	var applied bool
	if err := transaction.QueryRow(
		ctx,
		"SELECT EXISTS (SELECT 1 FROM public.schema_migrations WHERE name = $1)",
		item.name,
	).Scan(&applied); err != nil {
		return fmt.Errorf("check migration %s: %w", item.name, err)
	}
	if applied {
		return transaction.Commit(ctx)
	}

	if _, err := transaction.Exec(ctx, item.sql); err != nil {
		return fmt.Errorf("execute migration %s: %w", item.name, err)
	}
	if _, err := transaction.Exec(
		ctx,
		"INSERT INTO public.schema_migrations (name) VALUES ($1)",
		item.name,
	); err != nil {
		return fmt.Errorf("record migration %s: %w", item.name, err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %s: %w", item.name, err)
	}
	return nil
}
