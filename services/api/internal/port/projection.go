package port

import (
	"context"
	"time"

	leg "s4rciv.org/api/internal/domain/legislative"
	obs "s4rciv.org/api/internal/domain/observation"
)

// ObservedEvent is one observation event plus its decompressed snapshot content,
// as the projector reads it. SnapshotBytes is nil for ResourceVanished.
type ObservedEvent struct {
	Seq           int64
	StreamID      string
	Type          obs.EventType
	ObservedAt    time.Time
	WasOCR        bool
	SnapshotBytes []byte
}

// EventReader streams observation events in seq order for projection.
type EventReader interface {
	EventsSince(ctx context.Context, afterSeq int64, limit int) ([]ObservedEvent, error)
	// PrevContentSnapshot returns the most recent content-bearing (decompressed)
	// snapshot in the stream strictly before beforeSeq. found=false when none
	// exists (e.g. the first observation of a stream). Used by the diff usecase to
	// pair consecutive snapshots per stream.
	PrevContentSnapshot(ctx context.Context, streamID string, beforeSeq int64) ([]byte, bool, error)
}

// ProjectorOffset tracks how far a projector has folded (interpretation.projector_offset).
type ProjectorOffset interface {
	Offset(ctx context.Context, projector string) (int64, error)
	SetOffset(ctx context.Context, projector string, seq int64) error
	// BeginRebuild truncates the read models and resets this projector's offset,
	// atomically, for a reproject (ADR-000002 truncate -> reset -> replay).
	BeginRebuild(ctx context.Context, projector string) error
}

// StoredVote / StoredVoteEvent are the persisted vote read-model rows.
type StoredVote struct {
	Option     string
	VoterName  string
	PersonID   string // empty when unresolved
	Confidence string
	// District binding for the district vote map (ADR-000008), filled at READ TIME
	// by left-joining interpretation.legislator_district on PersonID — never stored
	// on the vote row (read-model independence). All empty when the person is
	// unresolved or absent from the current roster (現会期 → rendered as 記録なし).
	House        string
	DistrictCode string // 国土数値情報-aligned; empty when IsPR
	IsPR         bool   // 比例選出 — shown in the companion panel, never erased (§5)
	PRBlock      string
	Group        string // 会派 (from the roster) — labels the 比例 companion panel
}

type StoredVoteEvent struct {
	VoteEventID      string
	IssueID          string
	Motion           string
	YesCount         int
	NoCount          int
	AbstainCount     int
	Result           string
	Confidence       string
	NeedsReview      bool
	ExtractorVersion string
	SourceSpeechID   string // the speech this vote was parsed from (provenance)
	Votes            []StoredVote
}

// ProjectionBatch is the full set of read-model rows derived from one meeting
// snapshot. The usecase builds it; the store writes it in a single transaction,
// replacing any prior projection of the same meeting (reproject-safe).
type ProjectionBatch struct {
	Meeting        leg.Meeting
	Speeches       []leg.Speech
	Persons        []leg.Person
	Organizations  []leg.Organization
	Memberships    []leg.Membership
	VoteEvents     []StoredVoteEvent
	ObservationSeq int64
	ObservedAt     time.Time
}

// ReadModelStore writes the disposable interpretation-plane read models.
type ReadModelStore interface {
	ApplyMeeting(ctx context.Context, b ProjectionBatch) error
	Truncate(ctx context.Context) error
}

// ── giin-roster (legislator -> electoral district) projection (ADR-000008) ──────

// RosterNormalizer is the anti-corruption parse from a roster page snapshot to the
// legislator->district binding. Pure with respect to a snapshot, so projection
// stays reproject-safe.
type RosterNormalizer interface {
	ParseRoster(content []byte) ([]leg.RosterEntry, error)
}

// RosterProjectionBatch is the legislator_district rows derived from one roster-page
// snapshot. The store replaces that page's rows (by StreamID) in one transaction, so
// a member dropped from the page on re-observation is removed (reproject-safe).
type RosterProjectionBatch struct {
	StreamID       string
	Entries        []leg.RosterEntry
	ObservationSeq int64
	ObservedAt     time.Time
}

// RosterReadModelStore writes the disposable legislator_district read model.
type RosterReadModelStore interface {
	ApplyRoster(ctx context.Context, b RosterProjectionBatch) error
	TruncateRoster(ctx context.Context) error
}
