package kokkai

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"s4rciv.org/api/internal/blob"
	leg "s4rciv.org/api/internal/domain/legislative"
	"s4rciv.org/api/internal/port"
)

// fakeGetter serves recorded fixtures by endpoint — no live API (DISCIPLINE §1).
type fakeGetter struct {
	bodies map[string][]byte
	status int
}

func (f fakeGetter) Get(_ context.Context, endpoint string, _ url.Values) ([]byte, int, error) {
	st := f.status
	if st == 0 {
		st = 200
	}
	return f.bodies[endpoint], st, nil
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func TestFetchProducesStableSnapshot(t *testing.T) {
	g := New(fakeGetter{bodies: map[string][]byte{"meeting": fixture(t, "meeting.json")}})
	w := port.Watch{SourceLocalKey: "121815254X00120240115"}

	r1, err := g.Fetch(context.Background(), w)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !r1.Present || r1.Snapshot == nil {
		t.Fatal("expected present snapshot")
	}
	if r1.Permalink == "" {
		t.Fatal("permalink must be carried for attribution")
	}
	if r1.SourcePublishedAt == nil || r1.SourcePublishedAt.Format("2006-01-02") != "2024-01-15" {
		t.Fatalf("source_published_at = %v", r1.SourcePublishedAt)
	}

	// Canonicalization is stable: a second fetch yields the same content hash.
	r2, _ := g.Fetch(context.Background(), w)
	if r1.Snapshot.ContentHash != r2.Snapshot.ContentHash {
		t.Fatal("content hash is not stable across fetches")
	}

	// Mirrored bytes decompress back to the canonical content that was hashed.
	raw, err := blob.Decompress(r1.Snapshot.Bytes)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	if int64(len(raw)) != r1.Snapshot.ByteSize {
		t.Fatalf("byte_size %d != decompressed len %d", r1.Snapshot.ByteSize, len(raw))
	}
}

func TestFetchVanishedOn404(t *testing.T) {
	g := New(fakeGetter{status: 404})
	r, err := g.Fetch(context.Background(), port.Watch{SourceLocalKey: "x"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if r.Present {
		t.Fatal("404 must report not present (-> ResourceVanished)")
	}
}

func TestParseMeetingRoundTripAndVotes(t *testing.T) {
	g := New(fakeGetter{bodies: map[string][]byte{"meeting": fixture(t, "meeting.json")}})
	r, _ := g.Fetch(context.Background(), port.Watch{SourceLocalKey: "121815254X00120240115"})

	raw, _ := blob.Decompress(r.Snapshot.Bytes)
	content, err := g.ParseMeeting(raw)
	if err != nil {
		t.Fatalf("ParseMeeting: %v", err)
	}
	if content.Meeting.IssueID != "121815254X00120240115" {
		t.Fatalf("issue_id = %s", content.Meeting.IssueID)
	}
	if content.Meeting.StreamID != "kokkai:121815254X00120240115" {
		t.Fatalf("stream_id = %s", content.Meeting.StreamID)
	}
	if content.Meeting.House != "衆議院" || content.Meeting.Session != 213 {
		t.Fatalf("house/session = %s/%d", content.Meeting.House, content.Meeting.Session)
	}
	if len(content.Speeches) != 2 {
		t.Fatalf("speeches = %d, want 2", len(content.Speeches))
	}

	evs := leg.ParseVotes(content)
	if len(evs) != 1 {
		t.Fatalf("vote events = %d, want 1", len(evs))
	}
	if evs[0].Confidence != leg.ConfidenceHigh || evs[0].Result != "passed" {
		t.Fatalf("vote: confidence=%s result=%s", evs[0].Confidence, evs[0].Result)
	}
}

func TestListMeetings(t *testing.T) {
	g := New(fakeGetter{bodies: map[string][]byte{"meeting_list": fixture(t, "meeting_list.json")}})
	refs, err := g.ListMeetings(context.Background(), port.ListScope{From: "2024-01-15", Until: "2024-01-16"})
	if err != nil {
		t.Fatalf("ListMeetings: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("refs = %d, want 2", len(refs))
	}
	if refs[0].StreamID != "kokkai:121815254X00120240115" {
		t.Fatalf("ref[0].StreamID = %s", refs[0].StreamID)
	}
}

// pagerGetter computes each meeting_list page from the startRecord query param,
// simulating an upstream whose cursor we do not control.
type pagerGetter struct {
	page func(start int) []byte
}

func (p pagerGetter) Get(_ context.Context, _ string, q url.Values) ([]byte, int, error) {
	start, _ := strconv.Atoi(q.Get("startRecord"))
	return p.page(start), 200, nil
}

func listPage(start, next, records int) []byte {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`{"numberOfRecords":%d,"nextRecordPosition":%d,"meetingRecord":[`, records, next))
	for i := 0; i < records; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf(`{"issueID":"ISSUE%08d","meetingURL":"https://example.test/m"}`, start+i))
	}
	sb.WriteString("]}")
	return []byte(sb.String())
}

// An upstream that returns a non-advancing nextRecordPosition must abort the
// traversal with an error instead of looping forever on a trusted cursor.
func TestListMeetingsAbortsOnNonAdvancingCursor(t *testing.T) {
	g := New(pagerGetter{page: func(start int) []byte {
		return listPage(start, start, 1) // cursor never advances
	}})
	_, err := g.ListMeetings(context.Background(), port.ListScope{From: "2024-01-15", Until: "2024-01-16"})
	if err == nil || !strings.Contains(err.Error(), "non-advancing") {
		t.Fatalf("non-advancing cursor must abort with an error, got %v", err)
	}
}

// Even an always-advancing cursor is bounded by the page ceiling, so a runaway
// upstream listing cannot drive an unbounded crawl.
func TestListMeetingsAbortsAfterPageCeiling(t *testing.T) {
	g := New(pagerGetter{page: func(start int) []byte {
		return listPage(start, start+1, 1) // advances forever
	}})
	_, err := g.ListMeetings(context.Background(), port.ListScope{From: "2024-01-15", Until: "2024-01-16"})
	if err == nil || !strings.Contains(err.Error(), "pages") {
		t.Fatalf("runaway pagination must abort at the page ceiling, got %v", err)
	}
}

// An upstream that overstuffs a single page is bounded by the total-refs ceiling
// independently of scope.Max.
func TestListMeetingsAbortsAfterRefsCeiling(t *testing.T) {
	g := New(pagerGetter{page: func(start int) []byte {
		return listPage(start, 0, maxListRefs+1) // one giant "terminal" page
	}})
	_, err := g.ListMeetings(context.Background(), port.ListScope{From: "2024-01-15", Until: "2024-01-16"})
	if err == nil || !strings.Contains(err.Error(), "refs") {
		t.Fatalf("oversized page must abort at the refs ceiling, got %v", err)
	}
}
