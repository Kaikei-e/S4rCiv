package port

import (
	"context"
	"time"
)

// SourceConfig is the compliance-bearing configuration of one source/adapter
// (control.source). RateLimit encodes the DISCIPLINE §1 serial+interval rule.
type SourceConfig struct {
	Source    string
	BaseURL   string
	UserAgent string
	RateLimit time.Duration
	Enabled   bool
}

// Watch is one entry of what S4rCiv polls (control.watch).
type Watch struct {
	StreamID       string
	Source         string
	SourceLocalKey string
	CanonicalURL   string
}

// ControlStore is the mutable operational state (control plane).
type ControlStore interface {
	Source(ctx context.Context, source string) (SourceConfig, error)
	// DueWatches returns enabled watches whose next_due_at has passed (or is
	// unset), ordered oldest-first, capped at limit.
	DueWatches(ctx context.Context, source string, now time.Time, limit int) ([]Watch, error)
	UpsertWatch(ctx context.Context, w Watch) error
	// MarkPolled advances the poll cursor and backoff for a stream.
	MarkPolled(ctx context.Context, streamID string, polledAt, nextDue time.Time, ok bool) error
	// MarkPending records a poll that found the Resource present-but-without a
	// retrievable snapshot (FetchResult.ContentUnavailable). It advances the poll
	// cursor and schedules a soon re-poll at retryAt WITHOUT incrementing the
	// failure counter — this is not a fetch failure, the snapshot is simply not
	// published yet.
	MarkPending(ctx context.Context, streamID string, polledAt, retryAt time.Time) error
}
