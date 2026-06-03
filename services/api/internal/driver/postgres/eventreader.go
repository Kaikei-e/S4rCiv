package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"s4rciv.org/api/internal/blob"
	"s4rciv.org/api/internal/port"
)

// EventReader streams observation events (with decompressed snapshot content)
// in seq order for the projector.
type EventReader struct {
	pool *pgxpool.Pool
}

func NewEventReader(pool *pgxpool.Pool) *EventReader { return &EventReader{pool: pool} }

func (r *EventReader) EventsSince(ctx context.Context, afterSeq int64, limit int) ([]port.ObservedEvent, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT e.seq, e.stream_id, e.type::text, e.observed_at,
		       s.bytes, COALESCE(s.was_ocr, false)
		FROM observation.event e
		LEFT JOIN observation.snapshot s ON s.content_hash = e.content_hash
		WHERE e.seq > $1
		ORDER BY e.seq ASC
		LIMIT $2`, afterSeq, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []port.ObservedEvent
	for rows.Next() {
		var (
			seq        int64
			streamID   string
			typeText   string
			observedAt time.Time
			raw        []byte
			wasOCR     bool
		)
		if err := rows.Scan(&seq, &streamID, &typeText, &observedAt, &raw, &wasOCR); err != nil {
			return nil, err
		}
		ev := port.ObservedEvent{
			Seq:        seq,
			StreamID:   streamID,
			Type:       parseEventType(typeText),
			ObservedAt: observedAt,
			WasOCR:     wasOCR,
		}
		if raw != nil {
			content, err := blob.Decompress(raw)
			if err != nil {
				return nil, err
			}
			ev.SnapshotBytes = content
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

// PrevContentSnapshot returns the decompressed bytes of the most recent
// content-bearing snapshot in the stream strictly before beforeSeq (skips
// ResourceVanished, which carries no content_hash). found=false when none exists.
func (r *EventReader) PrevContentSnapshot(ctx context.Context, streamID string, beforeSeq int64) ([]byte, bool, error) {
	var raw []byte
	err := r.pool.QueryRow(ctx, `
		SELECT s.bytes
		FROM observation.event e
		JOIN observation.snapshot s ON s.content_hash = e.content_hash
		WHERE e.stream_id = $1 AND e.seq < $2 AND e.content_hash IS NOT NULL
		ORDER BY e.seq DESC LIMIT 1`, streamID, beforeSeq).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	content, err := blob.Decompress(raw)
	if err != nil {
		return nil, false, err
	}
	return content, true, nil
}
