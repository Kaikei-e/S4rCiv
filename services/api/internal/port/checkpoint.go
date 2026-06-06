package port

import (
	"context"

	obs "s4rciv.org/api/internal/domain/observation"
)

// CheckpointRecord is one signed checkpoint row to append (ADR-000019). SignedNote
// is the full C2SP signed-note bytes (stored in observation.checkpoint.signature);
// the scalar columns are a queryable index over it.
type CheckpointRecord struct {
	CheckpointID string
	ThroughSeq   int64
	TreeSize     int64
	RootHash     obs.Digest
	AlgVersion   string
	SignedNote   []byte
	SignerKeyID  string
}

// CheckpointStore is the command-side persistence for signed checkpoints. It reads
// the global log-chain head and appends checkpoints (append-only; never mutates).
type CheckpointStore interface {
	// ChainHead returns the current global log-chain head (seq, log_hash). seq 0 and
	// the zero digest mean the genesis head (nothing observed yet).
	ChainHead(ctx context.Context) (seq int64, head obs.Digest, err error)
	// LatestThroughSeq returns the through_seq of the most recent checkpoint, or 0.
	LatestThroughSeq(ctx context.Context) (int64, error)
	// AppendCheckpoint inserts one checkpoint row.
	AppendCheckpoint(ctx context.Context, rec CheckpointRecord) error
}

// CheckpointSigner signs a checkpoint's note, returning the C2SP signed-note bytes
// and the signing key's identifier. Implemented by a driver holding the mounted
// secret key; the usecase stays decoupled from the note/crypto library.
type CheckpointSigner interface {
	SignCheckpoint(c obs.Checkpoint) (signedNote []byte, signerKeyID string, err error)
}
