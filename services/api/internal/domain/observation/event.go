// Package observation holds the pure domain logic of the observation plane:
// the hash-chained, append-only ground truth. It has no external dependencies
// beyond the canonical hashing schema (the generated HashableEvent proto, used
// only as the pinned serialization for log_hash — see ADR-000003).
package observation

import (
	"crypto/sha256"
	"encoding/hex"

	observationv1 "s4rciv.org/api/gen/s4rciv/observation/v1"
)

// EventType is the observation event kind (CORE_CONCEPT §9.1).
type EventType int

const (
	Unknown EventType = iota
	ResourceObserved
	ResourceChanged
	ResourceVanished
	ResourceRestored
)

// DBValue is the observation.event_type enum label stored in Postgres.
func (t EventType) DBValue() string {
	switch t {
	case ResourceObserved:
		return "ResourceObserved"
	case ResourceChanged:
		return "ResourceChanged"
	case ResourceVanished:
		return "ResourceVanished"
	case ResourceRestored:
		return "ResourceRestored"
	default:
		return ""
	}
}

func (t EventType) proto() observationv1.EventType {
	switch t {
	case ResourceObserved:
		return observationv1.EventType_EVENT_TYPE_RESOURCE_OBSERVED
	case ResourceChanged:
		return observationv1.EventType_EVENT_TYPE_RESOURCE_CHANGED
	case ResourceVanished:
		return observationv1.EventType_EVENT_TYPE_RESOURCE_VANISHED
	case ResourceRestored:
		return observationv1.EventType_EVENT_TYPE_RESOURCE_RESTORED
	default:
		return observationv1.EventType_EVENT_TYPE_UNSPECIFIED
	}
}

// Digest is a sha256 hash. The zero value is the genesis log hash (32 zero
// bytes), matching observation.chain_head's seeded value.
type Digest [sha256.Size]byte

// SumBytes content-addresses a snapshot payload.
func SumBytes(b []byte) Digest { return sha256.Sum256(b) }

// Hex is the lowercase hex form used inside the log chain.
func (d Digest) Hex() string { return hex.EncodeToString(d[:]) }

// Bytes returns the raw 32 bytes for storage as bytea.
func (d Digest) Bytes() []byte { b := d; return b[:] }

// ContentRef is the "sha256:<hex>" form used for content/prev_content hashes.
func (d Digest) ContentRef() string { return "sha256:" + d.Hex() }

// DigestFromBytes rebuilds a Digest from a stored 32-byte slice.
func DigestFromBytes(b []byte) (Digest, bool) {
	var d Digest
	if len(b) != sha256.Size {
		return d, false
	}
	copy(d[:], b)
	return d, true
}
