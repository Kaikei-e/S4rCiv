//go:build integration

package postgres

import (
	"context"
	"testing"

	obs "s4rciv.org/api/internal/domain/observation"
)

// The interpretation plane's Tier-1 event table (review verdicts, corrections,
// overrides) carries its OWN append-only hash chain (ADR-000002): durable facts
// that survive every reproject and prove S4rCiv has not silently rewritten its
// verdicts. These assert the same trigger guarantees as the observation plane.

const insertInterpEventSQL = `
INSERT INTO interpretation.event
  (event_id, type, observation_seq, actor, decided_at, payload, log_prev_hash, log_hash)
VALUES ($1, $2, NULL, $3, $4, $5, $6, $7)
RETURNING seq`

func TestInterpretationEvent_AppendAssignsSeqAndIsAppendOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := newTestDB(t)

	var genesis obs.Digest // chain_head starts at zero
	logHash := obs.SumBytes([]byte("verdict-1"))
	var seq int64
	if err := pool.QueryRow(ctx, insertInterpEventSQL,
		uuidN(1), "ReviewVerdict", "reviewer:test", baseObserved,
		[]byte(`{"verdict":"substantive"}`), genesis.Bytes(), logHash.Bytes(),
	).Scan(&seq); err != nil {
		t.Fatalf("insert interpretation event: %v", err)
	}
	if seq != 1 {
		t.Fatalf("first interpretation seq = %d, want 1 (trigger-assigned)", seq)
	}

	_, err := pool.Exec(ctx, `UPDATE interpretation.event SET actor = 'tampered' WHERE seq = 1`)
	wantPgMessage(t, err, "append-only")
	_, err = pool.Exec(ctx, `DELETE FROM interpretation.event WHERE seq = 1`)
	wantPgMessage(t, err, "append-only")
}

func TestInterpretationEvent_RejectsBrokenLogChain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := newTestDB(t)

	wrong := obs.SumBytes([]byte("not the interpretation head"))
	logHash := obs.SumBytes([]byte("verdict"))
	var seq int64
	err := pool.QueryRow(ctx, insertInterpEventSQL,
		uuidN(1), "ReviewVerdict", "reviewer:test", baseObserved,
		[]byte(`{}`), wrong.Bytes(), logHash.Bytes(),
	).Scan(&seq)
	wantPgMessage(t, err, "broken interpretation log chain")
}
