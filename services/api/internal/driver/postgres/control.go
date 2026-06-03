package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"s4rciv.org/api/internal/port"
)

type ControlStore struct {
	pool *pgxpool.Pool
}

func NewControlStore(pool *pgxpool.Pool) *ControlStore { return &ControlStore{pool: pool} }

func (c *ControlStore) Source(ctx context.Context, source string) (port.SourceConfig, error) {
	var cfg port.SourceConfig
	var rateMs int
	err := c.pool.QueryRow(ctx, `
		SELECT source, base_url, rate_limit_ms, user_agent, enabled
		FROM control.source WHERE source = $1`, source,
	).Scan(&cfg.Source, &cfg.BaseURL, &rateMs, &cfg.UserAgent, &cfg.Enabled)
	if err != nil {
		return cfg, err
	}
	cfg.RateLimit = time.Duration(rateMs) * time.Millisecond
	return cfg, nil
}

func (c *ControlStore) DueWatches(ctx context.Context, source string, now time.Time, limit int) ([]port.Watch, error) {
	rows, err := c.pool.Query(ctx, `
		SELECT w.stream_id, w.source, w.source_local_key, w.canonical_url
		FROM control.watch w
		LEFT JOIN control.poll_state p ON p.stream_id = w.stream_id
		WHERE w.source = $1 AND w.enabled
		  AND (p.next_due_at IS NULL OR p.next_due_at <= $2)
		  AND (p.backoff_until IS NULL OR p.backoff_until <= $2)
		ORDER BY p.next_due_at ASC NULLS FIRST
		LIMIT $3`, source, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []port.Watch
	for rows.Next() {
		var w port.Watch
		if err := rows.Scan(&w.StreamID, &w.Source, &w.SourceLocalKey, &w.CanonicalURL); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (c *ControlStore) UpsertWatch(ctx context.Context, w port.Watch) error {
	_, err := c.pool.Exec(ctx, `
		INSERT INTO control.watch (stream_id, source, source_local_key, canonical_url)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (stream_id) DO UPDATE
		  SET source_local_key = EXCLUDED.source_local_key,
		      canonical_url = EXCLUDED.canonical_url`,
		w.StreamID, w.Source, w.SourceLocalKey, w.CanonicalURL)
	return err
}

func (c *ControlStore) MarkPolled(ctx context.Context, streamID string, polledAt, nextDue time.Time, ok bool) error {
	_, err := c.pool.Exec(ctx, `
		INSERT INTO control.poll_state
			(stream_id, last_polled_at, next_due_at, backoff_until, consecutive_failures)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (stream_id) DO UPDATE SET
			last_polled_at = EXCLUDED.last_polled_at,
			next_due_at = EXCLUDED.next_due_at,
			backoff_until = CASE WHEN $6 THEN NULL ELSE EXCLUDED.next_due_at END,
			consecutive_failures = CASE WHEN $6 THEN 0
				ELSE control.poll_state.consecutive_failures + 1 END`,
		streamID, polledAt, nextDue, backoffArg(ok, nextDue), failArg(ok), ok)
	return err
}

func backoffArg(ok bool, nextDue time.Time) any {
	if ok {
		return nil
	}
	return nextDue
}

func failArg(ok bool) int {
	if ok {
		return 0
	}
	return 1
}
