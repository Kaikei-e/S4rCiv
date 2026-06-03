package port

import (
	"context"
	"time"

	leg "s4rciv.org/api/internal/domain/legislative"
)

// FetchResult is one observation of a watched Resource. Present=false means the
// resource was absent (404/removed) this cycle — recorded as ResourceVanished
// (silence is information; DISCIPLINE §3).
type FetchResult struct {
	Present           bool
	Snapshot          *Snapshot // canonical content to record; nil when absent
	SourcePublishedAt *time.Time
	Permalink         string
}

// ResourceFetcher fetches one Resource over the source's read-only HTTP boundary.
type ResourceFetcher interface {
	Fetch(ctx context.Context, w Watch) (FetchResult, error)
}

// ListScope bounds a discovery traversal.
type ListScope struct {
	From  string // YYYY-MM-DD inclusive
	Until string // YYYY-MM-DD inclusive
	Max   int    // hard cap on refs returned (0 = adapter default)
}

// MeetingRef is a discovered resource to add to the watch list.
type MeetingRef struct {
	StreamID       string
	SourceLocalKey string
	CanonicalURL   string
}

// MeetingLister traverses a source's listing endpoint (kokkai meeting_list).
type MeetingLister interface {
	ListMeetings(ctx context.Context, scope ListScope) ([]MeetingRef, error)
}

// Normalizer is the anti-corruption parse from canonical snapshot bytes to the
// interpretation-plane domain. Pure with respect to a snapshot, so projection
// stays reproject-safe.
type Normalizer interface {
	ParseMeeting(content []byte) (leg.MeetingContent, error)
}
