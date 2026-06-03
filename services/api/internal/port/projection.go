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
