package port

import (
	"context"
	"time"

	leg "s4rciv.org/api/internal/domain/legislative"
)

// FetchResult is one observation of a watched Resource. Present=false means the
// resource was absent (confirmed gone) this cycle — recorded as ResourceVanished
// (silence is information; DISCIPLINE §3).
//
// Present and ContentUnavailable are distinct signals: existence is whether the
// Resource is still listed by the source's authoritative metadata, content is
// whether a retrievable snapshot exists. A missing content artifact while the
// Resource still exists is ContentUnavailable, NOT absence — see below.
type FetchResult struct {
	Present           bool
	Snapshot          *Snapshot // canonical content to record; nil when absent
	SourcePublishedAt *time.Time
	Permalink         string

	// ContentUnavailable means the Resource still exists at the source (confirmed
	// via authoritative metadata) but no retrievable snapshot is published yet —
	// e.g. e-Gov has switched a law's current-revision pointer but has not yet
	// published that revision's 法令標準XML, so law_data 404s while /laws still
	// lists the law. The collector emits NO event and re-polls soon; this must
	// never be read as ResourceVanished (DISCIPLINE §4-3: never write an absence
	// that did not happen into the immutable log). Mutually exclusive with Present.
	ContentUnavailable bool
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
