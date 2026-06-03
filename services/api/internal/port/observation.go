// Package port declares the contracts between usecases and the outside world.
// Usecases depend only on these interfaces; drivers and gateways implement them.
// Ports may reference the inner domain entities, never the outer layers.
package port

import (
	"context"
	"time"

	obs "s4rciv.org/api/internal/domain/observation"
)

// Stream is the immutable identity of one watched Resource.
type Stream struct {
	StreamID       string
	Source         string
	SourceLocalKey string
	CanonicalURL   string
}

// Snapshot is the content-addressed raw payload to persist for an event.
// Bytes (a compressed mirror, justified by Copyright Act art. 30-4 — ADR-000004)
// and ExternalRef are mutually sufficient; at least one must be set.
type Snapshot struct {
	ContentHash obs.Digest
	Bytes       []byte // compressed mirror; nil when only externally referenced
	ExternalRef string // e.g. Internet Archive URL, when not mirrored
	ByteSize    int64  // uncompressed size of the canonical content
	MediaType   string
	WasOCR      bool
}

// AppendCmd carries the business facts of one observation event. seq, stream_seq,
// log_prev_hash and log_hash are assigned/computed inside EventLog.Append.
type AppendCmd struct {
	Stream            Stream
	Type              obs.EventType
	EventID           string // uuidv7
	Source            string
	FetcherVersion    string
	ObservedAt        time.Time
	SourcePublishedAt *time.Time
	Snapshot          *Snapshot // nil on ResourceVanished
	PrevContentHash   *obs.Digest
}

// EventLog is the append-only observation ground truth (write side).
type EventLog interface {
	EnsureStream(ctx context.Context, s Stream) error
	StreamState(ctx context.Context, streamID string) (obs.StreamState, error)
	// Append persists the snapshot (if any) and the event atomically, taking the
	// chain_head lock first so the global log chain stays gap- and fork-free.
	Append(ctx context.Context, cmd AppendCmd) (seq int64, err error)
}
