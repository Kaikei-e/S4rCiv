// Package postgres implements the persistence ports over Postgres 18 via pgx.
// The observation-plane integrity (seq assignment, log-chain linkage) is enforced
// by DB triggers (ADR-000001); this driver supplies the app-computed log_hash and
// serializes appends by locking chain_head first.
package postgres

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Option tweaks the pool config built by Connect before the pool is created.
type Option func(*pgxpool.Config)

// WithStatementTimeout sets a server-side statement_timeout on every connection,
// so a single pathological query cannot hold a backend indefinitely (CWE-400).
// Apply this on the read-only query side ONLY — the collector's reproject /
// projection statements may legitimately run long and must stay unbounded.
func WithStatementTimeout(d time.Duration) Option {
	return func(cfg *pgxpool.Config) {
		cfg.ConnConfig.RuntimeParams["statement_timeout"] = strconv.FormatInt(d.Milliseconds(), 10)
	}
}

// Connect builds a pool from the standard env vars. The password is read from
// the mounted Docker secret file (DB_PASSWORD_FILE) and never placed in an env
// var or logged.
func Connect(ctx context.Context, opts ...Option) (*pgxpool.Pool, error) {
	pw, err := readSecret(os.Getenv("DB_PASSWORD_FILE"))
	if err != nil {
		return nil, err
	}
	// sslmode=prefer: use TLS when the server offers it, transparently falling back
	// to plaintext on the current no-TLS compose DB (no breakage). Set sslmode=require
	// once the DB is reachable beyond the private compose network (CWE-319).
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s/%s?sslmode=prefer",
		envOr("POSTGRES_USER", "s4rciv"),
		pw,
		net.JoinHostPort(envOr("POSTGRES_HOST", "db"), envOr("POSTGRES_PORT", "5432")),
		envOr("POSTGRES_DB", "s4rciv"),
	)
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 8
	cfg.MaxConnLifetime = time.Hour
	for _, opt := range opts {
		opt(cfg)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func readSecret(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("DB_PASSWORD_FILE is not set")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read db password secret: %w", err)
	}
	pw := strings.TrimSpace(string(b))
	if pw == "" {
		return "", fmt.Errorf("db password secret at %s is empty", path)
	}
	return pw, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
