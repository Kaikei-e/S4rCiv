// Package checkpoint is the command-side usecase that attests the observation log
// chain head with a signed checkpoint (ADR-000019). It depends only on ports, so it
// is unit-testable with fakes (no DB, no key file). Generation is idempotent: if a
// checkpoint already covers the current head, it does nothing.
package checkpoint

import (
	"context"

	obs "s4rciv.org/api/internal/domain/observation"
	"s4rciv.org/api/internal/port"
)

type Usecase struct {
	store  port.CheckpointStore
	signer port.CheckpointSigner
	ids    port.IDGenerator
}

func New(store port.CheckpointStore, signer port.CheckpointSigner, ids port.IDGenerator) *Usecase {
	return &Usecase{store: store, signer: signer, ids: ids}
}

// Run appends a linked-v1 signed checkpoint over the current chain head, unless one
// already covers it. Returns whether a new checkpoint was created and the seq it
// commits to.
func (u *Usecase) Run(ctx context.Context) (created bool, throughSeq int64, err error) {
	seq, head, err := u.store.ChainHead(ctx)
	if err != nil {
		return false, 0, err
	}
	if seq == 0 {
		// Genesis: nothing has been observed yet, so there is nothing to attest.
		return false, 0, nil
	}
	last, err := u.store.LatestThroughSeq(ctx)
	if err != nil {
		return false, 0, err
	}
	if seq <= last {
		// A checkpoint already covers this head — idempotent, emit nothing.
		return false, seq, nil
	}

	cp := obs.NewLinkedCheckpoint(seq, head)
	signed, keyID, err := u.signer.SignCheckpoint(cp)
	if err != nil {
		return false, 0, err
	}
	rec := port.CheckpointRecord{
		CheckpointID: u.ids.NewID(),
		ThroughSeq:   cp.ThroughSeq,
		TreeSize:     cp.TreeSize,
		RootHash:     cp.RootHash,
		AlgVersion:   cp.AlgVersion,
		SignedNote:   signed,
		SignerKeyID:  keyID,
	}
	if err := u.store.AppendCheckpoint(ctx, rec); err != nil {
		return false, 0, err
	}
	return true, seq, nil
}
