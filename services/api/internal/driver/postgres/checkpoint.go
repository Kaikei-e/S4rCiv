package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	obs "s4rciv.org/api/internal/domain/observation"
	"s4rciv.org/api/internal/port"
)

// CheckpointStore appends signed checkpoints over the observation log chain
// (ADR-000019). It reads the global chain_head and inserts into the append-only
// observation.checkpoint table; mutation is rejected by a DB trigger.
type CheckpointStore struct{ pool *pgxpool.Pool }

func NewCheckpointStore(pool *pgxpool.Pool) *CheckpointStore { return &CheckpointStore{pool: pool} }

// ChainHead reads the current global log-chain head. The genesis head is (0, zero).
func (c *CheckpointStore) ChainHead(ctx context.Context) (int64, obs.Digest, error) {
	var (
		seq int64
		lh  []byte
	)
	err := c.pool.QueryRow(ctx,
		`SELECT seq, log_hash FROM observation.chain_head WHERE id = 1`,
	).Scan(&seq, &lh)
	if err != nil {
		return 0, obs.Digest{}, err
	}
	if len(lh) != len(obs.Digest{}) {
		return 0, obs.Digest{}, fmt.Errorf("chain head log_hash is not %d bytes", len(obs.Digest{}))
	}
	var head obs.Digest
	copy(head[:], lh)
	return seq, head, nil
}

// LatestThroughSeq returns the through_seq of the most recent checkpoint, or 0 when
// none has been recorded — used to make checkpoint generation idempotent.
func (c *CheckpointStore) LatestThroughSeq(ctx context.Context) (int64, error) {
	var s int64
	err := c.pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(through_seq), 0) FROM observation.checkpoint`,
	).Scan(&s)
	return s, err
}

// AppendCheckpoint inserts one signed checkpoint row.
func (c *CheckpointStore) AppendCheckpoint(ctx context.Context, rec port.CheckpointRecord) error {
	_, err := c.pool.Exec(ctx, `
		INSERT INTO observation.checkpoint
		  (checkpoint_id, through_seq, tree_size, root_hash, alg_version, signature, signer_key_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		rec.CheckpointID, rec.ThroughSeq, rec.TreeSize, rec.RootHash.Bytes(),
		rec.AlgVersion, rec.SignedNote, rec.SignerKeyID,
	)
	return err
}
