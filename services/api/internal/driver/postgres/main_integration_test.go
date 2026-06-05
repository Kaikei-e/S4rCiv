//go:build integration

// Integration harness for the postgres drivers. Gated behind the `integration`
// build tag so `go test ./...` (unit, fakes only) stays hermetic and DB-free.
//
// Isolation = database-per-test via CREATE DATABASE ... TEMPLATE (ADR-000016):
// a single migrated template (s4rciv_tmpl) is built once in TestMain by applying
// the real migration SQL, then each test clones its own database from it. This is
// the only reset strategy that preserves the append-only log's true global `seq`
// and real COMMIT visibility while remaining safe under t.Parallel() — TRUNCATE
// takes ACCESS EXCLUSIVE (not parallel-safe) and a wrapping tx/rollback would turn
// the driver's own COMMIT into a savepoint release (PostgreSQL docs / go-txdb).
package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const templateDB = "s4rciv_tmpl"

var (
	dbUser, dbPass, dbHost, dbPort string
	cloneCounter                   atomic.Int64
)

func TestMain(m *testing.M) {
	code, err := setupAndRun(m)
	if err != nil {
		fmt.Fprintln(os.Stderr, "integration harness setup failed:", err)
		fmt.Fprintln(os.Stderr, "(this suite needs the compose Postgres; run via `make integration`)")
		os.Exit(1)
	}
	os.Exit(code)
}

func setupAndRun(m *testing.M) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pw, err := readSecret(os.Getenv("DB_PASSWORD_FILE"))
	if err != nil {
		return 1, err
	}
	dbUser = envOr("POSTGRES_USER", "s4rciv")
	dbPass = pw
	dbHost = envOr("POSTGRES_HOST", "db")
	dbPort = envOr("POSTGRES_PORT", "5432")

	if err := provisionTemplate(ctx); err != nil {
		return 1, fmt.Errorf("provision template db: %w", err)
	}
	return m.Run(), nil
}

// dsn builds a connection string for a named database on the compose Postgres.
func dsn(database string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPass, dbHost, dbPort, database)
}

// provisionTemplate (re)creates s4rciv_tmpl from scratch and applies the real
// migration SQL to it, then disconnects — a template must have zero connections
// for later CREATE DATABASE ... TEMPLATE to succeed.
func provisionTemplate(ctx context.Context) error {
	admin, err := pgx.Connect(ctx, dsn("postgres"))
	if err != nil {
		return fmt.Errorf("connect maintenance db: %w", err)
	}
	defer admin.Close(ctx)

	if err := terminateAndDrop(ctx, admin, templateDB); err != nil {
		return err
	}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+ident(templateDB)); err != nil {
		return fmt.Errorf("create template: %w", err)
	}

	tmpl, err := pgx.Connect(ctx, dsn(templateDB))
	if err != nil {
		return fmt.Errorf("connect template: %w", err)
	}
	defer tmpl.Close(ctx)
	return applyMigrations(ctx, tmpl)
}

// applyMigrations runs every db/migrations/*.sql (lexical order = chronological,
// per the timestamp-prefixed naming) against conn. The files are vanilla SQL with
// dollar-quoted function bodies, so each whole file is sent as one Exec over the
// simple query protocol (pgx runs multi-statement strings when there are no args).
func applyMigrations(ctx context.Context, conn *pgx.Conn) error {
	dir := envOr("MIGRATIONS_DIR", "/migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations dir %s: %w", dir, err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	if len(files) == 0 {
		return fmt.Errorf("no .sql migrations under %s", dir)
	}
	sort.Strings(files)
	for _, name := range files {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		if _, err := conn.Exec(ctx, string(b)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
	}
	return nil
}

// newTestDB clones a fresh, fully-migrated database from the template, returns a
// pool to it, and registers teardown. Safe under t.Parallel(): each test gets its
// own database, so global `seq`/chain_head and any sequences start clean.
func newTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	name := fmt.Sprintf("s4rciv_test_%d_%d", os.Getpid(), cloneCounter.Add(1))

	admin, err := pgx.Connect(ctx, dsn("postgres"))
	if err != nil {
		t.Fatalf("admin connect: %v", err)
	}
	defer admin.Close(ctx)
	if _, err := admin.Exec(ctx,
		fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", ident(name), ident(templateDB))); err != nil {
		t.Fatalf("clone test db %s: %v", name, err)
	}

	pool, err := pgxpool.New(ctx, dsn(name))
	if err != nil {
		t.Fatalf("open pool to %s: %v", name, err)
	}
	t.Cleanup(func() {
		pool.Close()
		a, err := pgx.Connect(ctx, dsn("postgres"))
		if err != nil {
			return
		}
		defer a.Close(ctx)
		_ = terminateAndDrop(ctx, a, name)
	})
	return pool
}

// terminateAndDrop force-drops a database after kicking any stragglers, so reruns
// and cleanup never wedge on a lingering connection.
func terminateAndDrop(ctx context.Context, conn *pgx.Conn, name string) error {
	_, _ = conn.Exec(ctx,
		`SELECT pg_terminate_backend(pid) FROM pg_stat_activity
		 WHERE datname = $1 AND pid <> pg_backend_pid()`, name)
	if _, err := conn.Exec(ctx, "DROP DATABASE IF EXISTS "+ident(name)+" WITH (FORCE)"); err != nil {
		return fmt.Errorf("drop %s: %w", name, err)
	}
	return nil
}

func ident(s string) string { return pgx.Identifier{s}.Sanitize() }
