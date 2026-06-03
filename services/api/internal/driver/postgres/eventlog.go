package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	obs "s4rciv.org/api/internal/domain/observation"
	"s4rciv.org/api/internal/port"
)

type EventLog struct {
	pool *pgxpool.Pool
}

func NewEventLog(pool *pgxpool.Pool) *EventLog { return &EventLog{pool: pool} }

func (e *EventLog) EnsureStream(ctx context.Context, s port.Stream) error {
	_, err := e.pool.Exec(ctx, `
		INSERT INTO observation.stream (stream_id, source, source_local_key, canonical_url)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (stream_id) DO NOTHING`,
		s.StreamID, s.Source, s.SourceLocalKey, nullStr(s.CanonicalURL))
	return err
}

func (e *EventLog) StreamState(ctx context.Context, streamID string) (obs.StreamState, error) {
	var st obs.StreamState
	var typeText string
	err := e.pool.QueryRow(ctx, `
		SELECT type::text FROM observation.event
		WHERE stream_id = $1 ORDER BY stream_seq DESC LIMIT 1`, streamID).Scan(&typeText)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return st, nil // stream has no events yet
	case err != nil:
		return st, err
	}
	st.Exists = true
	st.LastType = parseEventType(typeText)

	// Last actual snapshot (skips ResourceVanished, which has no content_hash).
	var raw []byte
	err = e.pool.QueryRow(ctx, `
		SELECT content_hash FROM observation.event
		WHERE stream_id = $1 AND content_hash IS NOT NULL
		ORDER BY stream_seq DESC LIMIT 1`, streamID).Scan(&raw)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// none yet (e.g. only a Vanished); leave LastContentHash nil
	case err != nil:
		return st, err
	default:
		if d, ok := obs.DigestFromBytes(raw); ok {
			st.LastContentHash = &d
		}
	}
	return st, nil
}

func (e *EventLog) Append(ctx context.Context, cmd port.AppendCmd) (int64, error) {
	tx, err := e.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after a successful Commit

	// Lock the chain head first: this serializes all appenders, so log_prev_hash
	// read here still matches when the BEFORE INSERT trigger re-validates it.
	var headSeq int64
	var headHash []byte
	if err := tx.QueryRow(ctx,
		`SELECT seq, log_hash FROM observation.chain_head WHERE id = 1 FOR UPDATE`,
	).Scan(&headSeq, &headHash); err != nil {
		return 0, fmt.Errorf("lock chain head: %w", err)
	}
	logPrev, ok := obs.DigestFromBytes(headHash)
	if !ok {
		return 0, fmt.Errorf("chain head log_hash is not 32 bytes")
	}

	var streamSeq int64
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(stream_seq), 0) + 1 FROM observation.event WHERE stream_id = $1`,
		cmd.Stream.StreamID,
	).Scan(&streamSeq); err != nil {
		return 0, fmt.Errorf("next stream_seq: %w", err)
	}

	var contentHash *obs.Digest
	if cmd.Snapshot != nil {
		contentHash = &cmd.Snapshot.ContentHash
	}

	facts := obs.EventFacts{
		EventID:           cmd.EventID,
		StreamID:          cmd.Stream.StreamID,
		StreamSeq:         streamSeq,
		Type:              cmd.Type,
		Source:            cmd.Source,
		FetcherVersion:    cmd.FetcherVersion,
		ObservedAt:        cmd.ObservedAt,
		SourcePublishedAt: cmd.SourcePublishedAt,
		ContentHash:       contentHash,
		PrevContentHash:   cmd.PrevContentHash,
		LogPrevHash:       logPrev,
	}
	logHash, err := facts.LogHash()
	if err != nil {
		return 0, fmt.Errorf("compute log_hash: %w", err)
	}

	if cmd.Snapshot != nil {
		if _, err := tx.Exec(ctx, `
			INSERT INTO observation.snapshot
				(content_hash, bytes, external_ref, byte_size, media_type, was_ocr)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (content_hash) DO NOTHING`,
			cmd.Snapshot.ContentHash.Bytes(), cmd.Snapshot.Bytes, nullStr(cmd.Snapshot.ExternalRef),
			cmd.Snapshot.ByteSize, nullStr(cmd.Snapshot.MediaType), cmd.Snapshot.WasOCR,
		); err != nil {
			return 0, fmt.Errorf("insert snapshot: %w", err)
		}
	}

	var seq int64
	// seq is omitted: the BEFORE INSERT trigger assigns it and advances the head.
	if err := tx.QueryRow(ctx, `
		INSERT INTO observation.event
			(event_id, stream_id, stream_seq, type, source, fetcher_version,
			 observed_at, source_published_at, content_hash, prev_content_hash,
			 log_prev_hash, log_hash)
		VALUES ($1, $2, $3, $4::observation.event_type, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING seq`,
		cmd.EventID, cmd.Stream.StreamID, streamSeq, cmd.Type.DBValue(), cmd.Source,
		cmd.FetcherVersion, cmd.ObservedAt, cmd.SourcePublishedAt,
		digestBytes(contentHash), digestBytes(cmd.PrevContentHash),
		logPrev.Bytes(), logHash.Bytes(),
	).Scan(&seq); err != nil {
		return 0, fmt.Errorf("insert event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return seq, nil
}

func digestBytes(d *obs.Digest) []byte {
	if d == nil {
		return nil
	}
	return d.Bytes()
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func parseEventType(s string) obs.EventType {
	switch s {
	case "ResourceObserved":
		return obs.ResourceObserved
	case "ResourceChanged":
		return obs.ResourceChanged
	case "ResourceVanished":
		return obs.ResourceVanished
	case "ResourceRestored":
		return obs.ResourceRestored
	default:
		return obs.Unknown
	}
}
