package port

import (
	"context"
	"time"

	leg "s4rciv.org/api/internal/domain/legislative"
)

// Attribution travels with every read-model view: source, the NDL reference
// permalink, and the event-time fetch timestamp (DISCIPLINE §9; ADR-000004).
type Attribution struct {
	Source         string
	Permalink      string
	FetchedAt      time.Time
	ObservationSeq int64
	WasOCR         bool
}

type MeetingView struct {
	Meeting leg.Meeting
	Attr    Attribution
}

type SpeechView struct {
	Speech leg.Speech
	Attr   Attribution
}

type VoteEventView struct {
	Event StoredVoteEvent
	Attr  Attribution
}

// QueryReader is the read-only view over the interpretation read models.
type QueryReader interface {
	Meeting(ctx context.Context, issueID string) (MeetingView, []SpeechView, bool, error)
	ListMeetings(ctx context.Context, session int, house string, limit, offset int) ([]MeetingView, error)
	VoteEvent(ctx context.Context, voteEventID string) (VoteEventView, bool, error)
}
