package postgres

import (
	"context"
	"encoding/hex"
	"errors"

	"github.com/jackc/pgx/v5"

	"s4rciv.org/api/internal/port"
)

// MastheadStatus returns the watch coverage (enabled Resources) and the latest
// checkpoint, if any, for the global provenance masthead (ADR-000018/000019). The
// count is scope, never a completeness claim; the checkpoint is a commitment, never
// a self-graded "verified" flag (ADR-000014).
func (q *QueryReader) MastheadStatus(ctx context.Context) (int64, port.CheckpointView, bool, error) {
	var watch int64
	if err := q.pool.QueryRow(ctx,
		`SELECT count(*) FROM control.watch WHERE enabled`,
	).Scan(&watch); err != nil {
		return 0, port.CheckpointView{}, false, err
	}

	var (
		cp       port.CheckpointView
		rootHash []byte
	)
	err := q.pool.QueryRow(ctx, `
		SELECT through_seq, tree_size, root_hash, alg_version,
		       signature IS NOT NULL, COALESCE(signer_key_id, ''), recorded_at
		FROM observation.checkpoint
		ORDER BY through_seq DESC
		LIMIT 1`,
	).Scan(&cp.ThroughSeq, &cp.TreeSize, &rootHash, &cp.AlgVersion,
		&cp.Signed, &cp.SignerKeyID, &cp.RecordedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return watch, port.CheckpointView{}, false, nil
	}
	if err != nil {
		return 0, port.CheckpointView{}, false, err
	}
	cp.RootHash = hex.EncodeToString(rootHash)
	return watch, cp, true, nil
}
