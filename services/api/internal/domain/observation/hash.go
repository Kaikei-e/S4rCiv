package observation

import (
	"crypto/sha256"
	"time"

	"google.golang.org/protobuf/proto"

	observationv1 "s4rciv.org/api/gen/s4rciv/observation/v1"
)

// AlgVersion pins the canonical hashing scheme recorded in
// observation.checkpoint.alg_version (ADR-000003). Bump it (and never reuse a
// retired value) when the HashableEvent field set or encoding changes.
const AlgVersion = "proto-linked-v1"

// EventFacts are the business facts of one observation event — the inputs to
// the canonical log hash. They are assembled by the collect usecase; seq and
// recorded_at (DB-assigned, ops-only) are deliberately absent.
type EventFacts struct {
	EventID           string // uuidv7, lowercase hyphenated
	StreamID          string
	StreamSeq         int64
	Type              EventType
	Source            string
	FetcherVersion    string
	ObservedAt        time.Time
	SourcePublishedAt *time.Time // nil when the source asserts none
	ContentHash       *Digest    // nil on ResourceVanished
	PrevContentHash   *Digest    // nil at the first snapshot of a stream
	LogPrevHash       Digest     // previous event's log_hash (genesis = zero)
}

// Hashable projects the facts onto the pinned, scalar-only canonical message.
// Absent values use the documented empty encoding ("" / zero), never an unset
// field, so Deterministic marshaling is stable across proto implementations.
//
// The verification read surface (ADR-000014) reuses this exact projection to
// rebuild a stored event for client-side re-hashing, so the bytes a third party
// re-marshals are identical to the bytes the collector hashed — the canonical
// form lives in one place, never re-derived in SQL.
func (f EventFacts) Hashable() *observationv1.HashableEvent {
	return &observationv1.HashableEvent{
		EventId:           f.EventID,
		StreamId:          f.StreamID,
		StreamSeq:         f.StreamSeq,
		Type:              f.Type.proto(),
		Source:            f.Source,
		FetcherVersion:    f.FetcherVersion,
		ObservedAt:        canonicalTime(&f.ObservedAt),
		SourcePublishedAt: canonicalTime(f.SourcePublishedAt),
		ContentHash:       contentRef(f.ContentHash),
		PrevContentHash:   contentRef(f.PrevContentHash),
		LogPrevHash:       f.LogPrevHash.Hex(),
	}
}

// LogHash computes log_hash = sha256(Deterministic-marshal(HashableEvent)).
// Re-running it with identical facts yields identical bytes; this is what a
// third-party verifier reimplements to check the chain (CORE_CONCEPT §13).
func (f EventFacts) LogHash() (Digest, error) {
	b, err := proto.MarshalOptions{Deterministic: true}.Marshal(f.Hashable())
	if err != nil {
		return Digest{}, err
	}
	return sha256.Sum256(b), nil
}

// canonicalTime normalizes to RFC3339 in UTC at second precision; nil -> "".
func canonicalTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Truncate(time.Second).Format(time.RFC3339)
}

func contentRef(d *Digest) string {
	if d == nil {
		return ""
	}
	return d.ContentRef()
}
