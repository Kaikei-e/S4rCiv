package postgres

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// WithStatementTimeout must surface as a statement_timeout runtime param (in
// milliseconds) on the connection config, so every session opened by the query
// side carries the server-enforced bound. No DB needed: the option only mutates
// the parsed config.
func TestWithStatementTimeout(t *testing.T) {
	cfg, err := pgxpool.ParseConfig("postgres://u:secret@localhost:5432/s4rciv")
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	WithStatementTimeout(10 * time.Second)(cfg)
	if got := cfg.ConnConfig.RuntimeParams["statement_timeout"]; got != "10000" {
		t.Errorf("statement_timeout = %q, want %q", got, "10000")
	}
}
