package observation

import (
	"encoding/base64"
	"fmt"

	"golang.org/x/mod/sumdb/note"
)

// CheckpointOrigin identifies this log and its commitment scheme inside the signed
// note. linked-v1 commits to the linked-list chain head (chain_head.log_hash), which
// is NOT an RFC 6962 Merkle root, so the origin carries the alg version to make clear
// this is the linked variant and must not be mistaken for a conformant C2SP tlog
// checkpoint (ADR-000019). merkle-v1 will commit to a Merkle root under its own origin
// and only then join the public witness network.
const CheckpointOrigin = "s4rciv.org/observation/" + AlgVersion

// Checkpoint is a commitment to the observation log chain through ThroughSeq. For
// linked-v1: RootHash is chain_head.log_hash at that seq and TreeSize == ThroughSeq.
// It carries no signature itself; signing is applied to its note encoding.
type Checkpoint struct {
	Origin     string
	ThroughSeq int64
	TreeSize   int64
	RootHash   Digest
	AlgVersion string
}

// NewLinkedCheckpoint builds a linked-v1 checkpoint over a chain head (seq, log_hash).
func NewLinkedCheckpoint(seq int64, head Digest) Checkpoint {
	return Checkpoint{
		Origin:     CheckpointOrigin,
		ThroughSeq: seq,
		TreeSize:   seq,
		RootHash:   head,
		AlgVersion: AlgVersion,
	}
}

// NoteText is the signed-note body: three newline-terminated lines (origin, decimal
// tree size, base64 root hash) — the C2SP tlog-checkpoint body shape. For linked-v1
// the "root" is the linked chain head (see CheckpointOrigin), not a Merkle root.
func (c Checkpoint) NoteText() string {
	return fmt.Sprintf("%s\n%d\n%s\n",
		c.Origin, c.TreeSize, base64.StdEncoding.EncodeToString(c.RootHash.Bytes()))
}

// Sign returns the signed note (body + Ed25519 signature line) in the C2SP signed-note
// encoding, using the canonical sumdb/note implementation so the bytes are byte-exact
// with the transparency-log ecosystem (a third party verifies with the published key).
func (c Checkpoint) Sign(signer note.Signer) ([]byte, error) {
	return note.Sign(&note.Note{Text: c.NoteText()}, signer)
}
