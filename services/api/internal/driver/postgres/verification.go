package postgres

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	obs "s4rciv.org/api/internal/domain/observation"
	"s4rciv.org/api/internal/port"
)

// StreamVerification returns one Stream's full verifiable export for the
// in-browser verifier (ADR-000014): every event, in stream_seq order, rebuilt
// into the domain EventFacts so the handler can re-derive the EXACT canonical
// HashableEvent the collector hashed (never re-deriving the canonical form in
// SQL), plus the stored log_hash and the covering checkpoint when one exists.
//
// This is an observation-plane read keyed by stream_id; it touches no
// interpretation read model, so any Stream (meeting / law / future source)
// verifies through the same path.
func (q *QueryReader) StreamVerification(ctx context.Context, streamID string) (port.StreamVerificationView, bool, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT e.seq, e.event_id::text, e.stream_seq, e.type::text, e.source,
		       e.fetcher_version, e.observed_at, e.source_published_at,
		       e.content_hash, e.prev_content_hash, e.log_prev_hash, e.log_hash
		FROM observation.event e
		WHERE e.stream_id = $1
		ORDER BY e.stream_seq ASC`, streamID)
	if err != nil {
		return port.StreamVerificationView{}, false, fmt.Errorf("query stream %s events: %w", streamID, err)
	}
	defer rows.Close()

	var (
		events []port.VerifiableEventView
		maxSeq int64
	)
	for rows.Next() {
		var (
			seq                                              int64
			eventID, typeText, source, fetcherVersion        string
			streamSeq                                        int64
			observedAt                                       time.Time
			sourcePublishedAt                                *time.Time
			contentHash, prevContentHash, logPrev, logHashed []byte
		)
		if err := rows.Scan(&seq, &eventID, &streamSeq, &typeText, &source,
			&fetcherVersion, &observedAt, &sourcePublishedAt,
			&contentHash, &prevContentHash, &logPrev, &logHashed); err != nil {
			return port.StreamVerificationView{}, false, fmt.Errorf("scan stream %s event: %w", streamID, err)
		}

		logPrevHash, ok := obs.DigestFromBytes(logPrev)
		if !ok {
			return port.StreamVerificationView{}, false,
				fmt.Errorf("stream %s seq %d: log_prev_hash is not 32 bytes", streamID, seq)
		}
		facts := obs.EventFacts{
			EventID:           eventID,
			StreamID:          streamID,
			StreamSeq:         streamSeq,
			Type:              parseEventType(typeText),
			Source:            source,
			FetcherVersion:    fetcherVersion,
			ObservedAt:        observedAt,
			SourcePublishedAt: sourcePublishedAt,
			LogPrevHash:       logPrevHash,
		}
		if d, ok := obs.DigestFromBytes(contentHash); ok {
			facts.ContentHash = &d
		}
		if d, ok := obs.DigestFromBytes(prevContentHash); ok {
			facts.PrevContentHash = &d
		}

		events = append(events, port.VerifiableEventView{
			Seq:     seq,
			Facts:   facts,
			LogHash: hex.EncodeToString(logHashed),
		})
		if seq > maxSeq {
			maxSeq = seq
		}
	}
	if err := rows.Err(); err != nil {
		return port.StreamVerificationView{}, false, fmt.Errorf("iterate stream %s events: %w", streamID, err)
	}
	if len(events) == 0 {
		return port.StreamVerificationView{}, false, nil
	}

	view := port.StreamVerificationView{
		StreamID:   streamID,
		Source:     sourceOf(streamID),
		AlgVersion: obs.AlgVersion,
		Events:     events,
	}
	cp, err := q.coveringCheckpoint(ctx, maxSeq)
	if err != nil {
		return port.StreamVerificationView{}, false, fmt.Errorf("covering checkpoint for stream %s: %w", streamID, err)
	}
	view.Checkpoint = cp
	return view, true, nil
}

// coveringCheckpoint returns the earliest checkpoint that commits to a seq >=
// throughSeq (i.e. the nearest checkpoint that bounds this stream's events), or
// nil when none has been recorded yet. In v0 no signing/anchoring job runs, so
// any row found has signature NULL and Signed=false (ADR-000014 §4 deferred).
func (q *QueryReader) coveringCheckpoint(ctx context.Context, throughSeq int64) (*port.CheckpointView, error) {
	var (
		cp       port.CheckpointView
		rootHash []byte
	)
	err := q.pool.QueryRow(ctx, `
		SELECT through_seq, tree_size, root_hash, alg_version,
		       signature IS NOT NULL, COALESCE(signer_key_id,''), recorded_at
		FROM observation.checkpoint
		WHERE through_seq >= $1
		ORDER BY through_seq ASC
		LIMIT 1`, throughSeq,
	).Scan(&cp.ThroughSeq, &cp.TreeSize, &rootHash, &cp.AlgVersion,
		&cp.Signed, &cp.SignerKeyID, &cp.RecordedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	cp.RootHash = hex.EncodeToString(rootHash)
	return &cp, nil
}
