// Package kokkai is the anti-corruption layer for the 国会会議録検索API: it maps
// kokkai JSON onto the interpretation-plane domain and produces canonical,
// content-addressed snapshots for the observation plane. It implements the
// port source interfaces over an injected HTTP getter.
package kokkai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/gowebpki/jcs"

	"s4rciv.org/api/internal/blob"
	leg "s4rciv.org/api/internal/domain/legislative"
	obs "s4rciv.org/api/internal/domain/observation"
	"s4rciv.org/api/internal/port"
)

// SourceName is the adapter/source identifier and stream_id prefix.
const SourceName = "kokkai"

// maxListPages and maxListRefs bound one ListMeetings traversal regardless of
// what the upstream cursor claims, so a looping or runaway nextRecordPosition
// can never drive an unbounded crawl. A wider backfill must use narrower date
// windows across multiple discover calls.
const (
	maxListPages = 200
	maxListRefs  = 20000
)

// httpGetter is the read-only HTTP-GET boundary (driver/kokkaihttp.Client).
type httpGetter interface {
	Get(ctx context.Context, endpoint string, q url.Values) ([]byte, int, error)
}

type Gateway struct {
	http httpGetter
}

func New(h httpGetter) *Gateway { return &Gateway{http: h} }

// StreamID is the deterministic stream identity for an issueID.
func StreamID(issueID string) string { return leg.MeetingStreamID(issueID) }

// Fetch GETs one meeting by issueID, drops the query envelope, JCS-canonicalizes
// the meetingRecord and content-addresses it. Absence (404 or empty result) is
// reported as not-present so it can be recorded as ResourceVanished.
func (g *Gateway) Fetch(ctx context.Context, w port.Watch) (port.FetchResult, error) {
	q := url.Values{}
	q.Set("issueID", w.SourceLocalKey)
	q.Set("recordPacking", "json")

	body, status, err := g.http.Get(ctx, "meeting", q)
	if err != nil {
		return port.FetchResult{}, err
	}
	if status == 404 {
		return port.FetchResult{Present: false}, nil
	}
	if status != 200 {
		return port.FetchResult{}, fmt.Errorf("kokkai meeting: status %d", status)
	}

	var resp listResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return port.FetchResult{}, fmt.Errorf("decode meeting response: %w", err)
	}
	if len(resp.MeetingRecord) == 0 {
		return port.FetchResult{Present: false}, nil
	}

	canonical, err := jcs.Transform(resp.MeetingRecord[0])
	if err != nil {
		return port.FetchResult{}, fmt.Errorf("canonicalize meeting record: %w", err)
	}
	var mr meetingRecord
	if err := json.Unmarshal(resp.MeetingRecord[0], &mr); err != nil {
		return port.FetchResult{}, fmt.Errorf("decode meeting record: %w", err)
	}

	compressed, err := blob.Compress(canonical)
	if err != nil {
		return port.FetchResult{}, err
	}
	snap := &port.Snapshot{
		ContentHash: obs.SumBytes(canonical),
		Bytes:       compressed,
		ByteSize:    int64(len(canonical)),
		MediaType:   "application/json",
		WasOCR:      false,
	}
	return port.FetchResult{
		Present:           true,
		Snapshot:          snap,
		SourcePublishedAt: parseDate(mr.Date),
		Permalink:         mr.MeetingURL,
	}, nil
}

// ParseMeeting decodes canonical snapshot bytes into the normalized domain. It is
// pure with respect to a snapshot, keeping projection reproject-safe.
func (g *Gateway) ParseMeeting(content []byte) (leg.MeetingContent, error) {
	var mr meetingRecord
	if err := json.Unmarshal(content, &mr); err != nil {
		return leg.MeetingContent{}, fmt.Errorf("parse meeting snapshot: %w", err)
	}
	return g.toContent(mr), nil
}

func (g *Gateway) toContent(mr meetingRecord) leg.MeetingContent {
	m := leg.Meeting{
		IssueID:     mr.IssueID,
		StreamID:    StreamID(mr.IssueID),
		Session:     mr.Session,
		House:       mr.NameOfHouse,
		MeetingName: mr.NameOfMeeting,
		Issue:       mr.Issue,
		Date:        mr.Date,
		Permalink:   mr.MeetingURL,
	}
	speeches := make([]leg.Speech, 0, len(mr.SpeechRecord))
	for _, s := range mr.SpeechRecord {
		speeches = append(speeches, leg.Speech{
			SpeechID:        s.SpeechID,
			IssueID:         mr.IssueID,
			Order:           s.SpeechOrder,
			Speaker:         s.Speaker,
			SpeakerYomi:     s.SpeakerYomi,
			SpeakerGroup:    s.SpeakerGroup,
			SpeakerPosition: s.SpeakerPosition,
			Text:            s.Speech,
			SpeechURL:       s.SpeechURL,
		})
	}
	return leg.MeetingContent{Meeting: m, Speeches: speeches, SourcePublishedAt: parseDate(mr.Date)}
}

// ListMeetings traverses meeting_list over the scope's date range, paging via
// nextRecordPosition, returning stream refs to add to the watch list. The cursor
// is upstream-controlled, so the traversal is bounded: the cursor must advance
// monotonically and the page/ref ceilings cap the crawl independently of scope.Max.
func (g *Gateway) ListMeetings(ctx context.Context, scope port.ListScope) ([]port.MeetingRef, error) {
	var refs []port.MeetingRef
	start := 1
	for page := 0; ; page++ {
		if page >= maxListPages {
			return nil, fmt.Errorf("kokkai meeting_list: exceeded %d pages without finishing; narrow the date window", maxListPages)
		}
		q := url.Values{}
		q.Set("recordPacking", "json")
		q.Set("maximumRecords", "100")
		q.Set("startRecord", strconv.Itoa(start))
		if scope.From != "" {
			q.Set("from", scope.From)
		}
		if scope.Until != "" {
			q.Set("until", scope.Until)
		}

		body, status, err := g.http.Get(ctx, "meeting_list", q)
		if err != nil {
			return nil, err
		}
		if status != 200 {
			return nil, fmt.Errorf("kokkai meeting_list: status %d", status)
		}
		var resp listResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("decode meeting_list: %w", err)
		}
		for _, raw := range resp.MeetingRecord {
			var mr meetingRecord
			if err := json.Unmarshal(raw, &mr); err != nil {
				return nil, err
			}
			if mr.IssueID == "" {
				continue
			}
			refs = append(refs, port.MeetingRef{
				StreamID:       StreamID(mr.IssueID),
				SourceLocalKey: mr.IssueID,
				CanonicalURL:   mr.MeetingURL,
			})
			if scope.Max > 0 && len(refs) >= scope.Max {
				return refs, nil
			}
			if len(refs) >= maxListRefs {
				return nil, fmt.Errorf("kokkai meeting_list: exceeded %d refs; narrow the date window", maxListRefs)
			}
		}
		if resp.NextRecordPosition <= 0 {
			return refs, nil
		}
		// Monotonicity guard: a non-advancing cursor would loop forever, so it is
		// an upstream protocol error, not a signal to keep trusting.
		if resp.NextRecordPosition <= start {
			return nil, fmt.Errorf("kokkai meeting_list: non-advancing nextRecordPosition %d at startRecord %d", resp.NextRecordPosition, start)
		}
		start = resp.NextRecordPosition
	}
}

func parseDate(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	return &t
}
