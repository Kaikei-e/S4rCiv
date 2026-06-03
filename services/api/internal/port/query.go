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
	// Global log-chain linkage of the backing event (hex sha256). Populated by the
	// timeline reader (which joins observation.event); empty for readers that do not.
	// Shown as chain linkage with a "(未検証)" label — not a verification result
	// (ADR-000007 / E1).
	LogHash     string
	PrevLogHash string
}

// TimelineFilter is the cross-source timeline query (ADR-000006). Filters are
// mechanical and source-agnostic; there is deliberately no person axis.
type TimelineFilter struct {
	Source         string // "" = all
	EventType      string // "" = all (ResourceObserved | ResourceChanged | ...)
	Classification string // "" = all (administrative | substantive)
	Since          string // RFC3339, observed_at >= since; "" = open
	Until          string // RFC3339, observed_at < until; "" = open
	Keyword        string // structured-field match (title/subtitle); never speech text
	Limit          int
	CursorSeq      int64 // keyset cursor; 0 = first page (no upper bound)
}

// TimelineItemView is one read-time-composed timeline row: the observation event
// (spine) enriched from the interpretation read models (body).
type TimelineItemView struct {
	Seq                 int64
	EventType           string
	Source              string
	StreamID            string
	ObservedAt          time.Time
	SourcePublishedAt   *time.Time
	Title               string
	Subtitle            string
	IssueID             string
	LawID               string
	FeaturedVoteEventID string
	Classification      string
	ClassConfidence     string
	NodesAdded          int
	NodesDeleted        int
	NodesModified       int
	WasOCR              bool
	Attr                Attribution
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

// VoteEventFilter is the current-session 記名投票 query backing the district-vote-map
// selector (ADR-000008). The map is a present-tense lens (現会期スコープ): Session 0
// resolves to the latest observed session.
type VoteEventFilter struct {
	Session      int    // 0 = current (latest observed session)
	House        string // "" = both
	MappableOnly bool   // true = only events that carry per-person 記名投票 records
	Limit        int
	Offset       int
}

// VoteEventSummaryView is one 記名投票 summary for the map selector: counts only,
// with the meeting context (never a bare option; §7).
type VoteEventSummaryView struct {
	VoteEventID   string
	IssueID       string
	Session       int
	House         string
	MeetingName   string
	Motion        string
	Date          string
	Result        string
	YesCount      int
	NoCount       int
	AbstainCount  int
	HasNamedVotes bool // per-person records exist → renderable on the map
	Attr          Attribution
}

// LegislatorVoteView is one named vote by a legislator, carrying the motion +
// meeting context (never a bare option; DISCIPLINE §7).
type LegislatorVoteView struct {
	VoteEventID string
	IssueID     string
	Motion      string
	Option      string
	Result      string
	MeetingName string
	House       string
	Date        string
	Confidence  string
	Attr        Attribution
}

// LegislatorVotes is a legislator's named-vote record (ADR-000006). Compiled only
// for a high-confidence identity; otherwise Votes is empty and IdentityConfidence
// tells the UI a possible homonym is not merged.
type LegislatorVotes struct {
	PersonID           string
	PersonName         string
	IdentityConfidence string
	Votes              []LegislatorVoteView
}

// ── 参議院本会議投票結果 マップ views (ADR-000010) ────────────────────────────────

type SangiinVoteEventSummaryView struct {
	VoteEventID string
	Session     int
	Motion      string
	Date        string
	YesCount    int
	NoCount     int
	Attr        Attribution
}

// PrefectureTallyView is one 都道府県's 内訳 (raw counts, not a rate; ADR-000010).
type PrefectureTallyView struct {
	DistrictCode string
	DistrictName string
	Yes          int
	No           int
	Abstain      int
}

type SangiinPrVoteView struct {
	VoterName string
	Option    string
	Group     string
}

type SangiinVoteMapView struct {
	VoteEventID  string
	Session      int
	Motion       string
	Date         string
	YesCount     int
	NoCount      int
	Prefectures  []PrefectureTallyView
	PrVotes      []SangiinPrVoteView
	TotalVotes   int
	MatchedVotes int
	Attr         Attribution
}

// QueryReader is the read-only view over the interpretation read models.
type QueryReader interface {
	Meeting(ctx context.Context, issueID string) (MeetingView, []SpeechView, bool, error)
	ListMeetings(ctx context.Context, session int, house string, limit, offset int) ([]MeetingView, error)
	VoteEvent(ctx context.Context, voteEventID string) (VoteEventView, bool, error)
	// ListVoteEvents serves the district-vote-map selector (ADR-000008). It returns
	// the resolved session (echoing the latest when the filter asked for 0) and the
	// 記名投票 summaries for it.
	ListVoteEvents(ctx context.Context, f VoteEventFilter) (int, []VoteEventSummaryView, error)
	ListTimeline(ctx context.Context, f TimelineFilter) ([]TimelineItemView, error)
	VotesByPerson(ctx context.Context, personID string, limit, offset int) (LegislatorVotes, bool, error)
	// 参議院 vote map (ADR-000010): list 記名投票 (session 0 = latest) and the per-都道府県
	// 内訳 + 比例 panel + coverage for one vote.
	ListSangiinVoteEvents(ctx context.Context, session, limit, offset int) (int, []SangiinVoteEventSummaryView, error)
	GetSangiinVoteMap(ctx context.Context, voteEventID string) (SangiinVoteMapView, bool, error)
}
