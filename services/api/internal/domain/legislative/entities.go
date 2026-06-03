// Package legislative holds the interpretation-plane domain entities for the
// kokkai (国会会議録) adapter: normalized Akoma-Ntoso/Popolo-shaped meetings,
// speeches, people, organizations and votes. Pure — every value is derived
// deterministically from observation snapshots so projection is reproject-safe.
package legislative

import "time"

// Meeting is one 会議録 (= one observation stream, keyed by the 21-char issueID).
type Meeting struct {
	IssueID     string
	StreamID    string
	Session     int    // 国会回次
	House       string // 衆議院 / 参議院 / 両院
	MeetingName string // 会議名 (本会議 / 委員会名)
	Issue       string // 号
	Date        string // YYYY-MM-DD
	Permalink   string // NDL reference URL (attribution)
	WasOCR      bool
}

// Speech is one 発言 within a meeting. Speaker is an attribute; presentation
// must never compile a single speaker's speeches into an anthology (ADR-000004).
type Speech struct {
	SpeechID        string
	IssueID         string
	Order           int
	Speaker         string
	SpeakerYomi     string
	SpeakerGroup    string // 会派
	SpeakerPosition string // 役職
	Text            string
	SpeechURL       string
	PersonID        string // resolved Popolo link, set during projection; empty if unresolved
}

// MeetingContent is the full normalized parse of one meeting snapshot.
type MeetingContent struct {
	Meeting           Meeting
	Speeches          []Speech
	SourcePublishedAt *time.Time
}
