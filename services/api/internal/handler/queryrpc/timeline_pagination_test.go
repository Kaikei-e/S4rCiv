package queryrpc

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	queryv1 "s4rciv.org/api/gen/s4rciv/query/v1"
	"s4rciv.org/api/internal/port"
)

// fakeTimelineReader emulates the postgres keyset driver against an in-memory
// descending seq set, so the handler's over-fetch trim + prev/next token logic is
// exercised end-to-end. Only ListTimeline is meaningful; the rest satisfy the
// port.QueryReader interface.
type fakeTimelineReader struct{ seqsDesc []int64 }

func (f fakeTimelineReader) ListTimeline(_ context.Context, flt port.TimelineFilter) ([]port.TimelineItemView, error) {
	var picked []int64
	if !flt.Backward {
		// seq < cursor (cursor 0 = head), newest-first, up to Limit.
		for _, s := range f.seqsDesc {
			if flt.CursorSeq == 0 || s < flt.CursorSeq {
				picked = append(picked, s)
				if len(picked) == flt.Limit {
					break
				}
			}
		}
	} else {
		// seq > cursor, the Limit smallest such (nearest above), then reversed to
		// DESC — exactly what the driver returns for a backward walk.
		var asc []int64
		for i := len(f.seqsDesc) - 1; i >= 0; i-- {
			if s := f.seqsDesc[i]; s > flt.CursorSeq {
				asc = append(asc, s)
				if len(asc) == flt.Limit {
					break
				}
			}
		}
		for i := len(asc) - 1; i >= 0; i-- {
			picked = append(picked, asc[i])
		}
	}
	out := make([]port.TimelineItemView, len(picked))
	for i, s := range picked {
		out[i] = port.TimelineItemView{Seq: s}
	}
	return out, nil
}

func (f fakeTimelineReader) CountTimeline(_ context.Context, _ port.TimelineFilter, aboveSeq int64) (int, int, error) {
	above := 0
	for _, s := range f.seqsDesc {
		if s > aboveSeq {
			above++
		}
	}
	return len(f.seqsDesc), above, nil
}

// Unused by the timeline path; present to satisfy port.QueryReader.
func (fakeTimelineReader) Meeting(context.Context, string) (port.MeetingView, []port.SpeechView, bool, error) {
	return port.MeetingView{}, nil, false, nil
}
func (fakeTimelineReader) ListMeetings(context.Context, int, string, int, int) ([]port.MeetingView, error) {
	return nil, nil
}
func (fakeTimelineReader) VoteEvent(context.Context, string) (port.VoteEventView, bool, error) {
	return port.VoteEventView{}, false, nil
}
func (fakeTimelineReader) ListVoteEvents(context.Context, port.VoteEventFilter) (int, []port.VoteEventSummaryView, error) {
	return 0, nil, nil
}
func (fakeTimelineReader) VotesByPerson(context.Context, string, int, int) (port.LegislatorVotes, bool, error) {
	return port.LegislatorVotes{}, false, nil
}
func (fakeTimelineReader) ListSangiinVoteEvents(context.Context, int, int, int) (int, []port.SangiinVoteEventSummaryView, error) {
	return 0, nil, nil
}
func (fakeTimelineReader) GetSangiinVoteMap(context.Context, string) (port.SangiinVoteMapView, bool, error) {
	return port.SangiinVoteMapView{}, false, nil
}
func (fakeTimelineReader) StreamVerification(context.Context, string) (port.StreamVerificationView, bool, error) {
	return port.StreamVerificationView{}, false, nil
}
func (fakeTimelineReader) MastheadStatus(context.Context) (int64, port.CheckpointView, bool, error) {
	return 0, port.CheckpointView{}, false, nil
}

func TestListTimelineKeysetPagination(t *testing.T) {
	// Full set: seq 10 (newest) … 1 (oldest), page size 3.
	reader := fakeTimelineReader{seqsDesc: []int64{10, 9, 8, 7, 6, 5, 4, 3, 2, 1}}
	h := New(reader, nil)

	cases := []struct {
		name      string
		token     string
		wantSeqs  []int64
		wantNext  string // older
		wantPrev  string // newer
		wantTotal int64
		wantPage  int32
	}{
		{"head", "", []int64{10, 9, 8}, "b:8", "", 10, 1},
		{"next from head", "b:8", []int64{7, 6, 5}, "b:5", "a:7", 10, 2},
		{"third page", "b:5", []int64{4, 3, 2}, "b:2", "a:4", 10, 3},
		{"last page", "b:2", []int64{1}, "", "a:1", 10, 4}, // no → past the tail
		{"back to head", "a:7", []int64{10, 9, 8}, "b:8", "", 10, 1}, // no dead ← at head
		{"bare int = older (back-compat)", "8", []int64{7, 6, 5}, "b:5", "a:7", 10, 2},
		{"backward over-fetch trims newest extra", "a:3", []int64{6, 5, 4}, "b:4", "a:6", 10, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := h.ListTimeline(context.Background(), connect.NewRequest(&queryv1.ListTimelineRequest{
				PageSize:  3,
				PageToken: tc.token,
			}))
			if err != nil {
				t.Fatalf("ListTimeline: %v", err)
			}
			got := make([]int64, len(resp.Msg.GetItems()))
			for i, it := range resp.Msg.GetItems() {
				got[i] = it.GetSeq()
			}
			if !equalSeqs(got, tc.wantSeqs) {
				t.Errorf("seqs = %v, want %v", got, tc.wantSeqs)
			}
			if resp.Msg.GetNextPageToken() != tc.wantNext {
				t.Errorf("next = %q, want %q", resp.Msg.GetNextPageToken(), tc.wantNext)
			}
			if resp.Msg.GetPrevPageToken() != tc.wantPrev {
				t.Errorf("prev = %q, want %q", resp.Msg.GetPrevPageToken(), tc.wantPrev)
			}
			if resp.Msg.GetTotalCount() != tc.wantTotal {
				t.Errorf("total = %d, want %d", resp.Msg.GetTotalCount(), tc.wantTotal)
			}
			if resp.Msg.GetPage() != tc.wantPage {
				t.Errorf("page = %d, want %d", resp.Msg.GetPage(), tc.wantPage)
			}
		})
	}
}

func TestParseTimelineCursor(t *testing.T) {
	cases := []struct {
		token        string
		wantSeq      int64
		wantBackward bool
	}{
		{"", 0, false},
		{"b:812", 812, false},
		{"a:811", 811, true},
		{"812", 812, false}, // bare int = older, back-compat
		{"garbage", 0, false},
		{"a:-5", 0, false},
	}
	for _, tc := range cases {
		seq, backward := parseTimelineCursor(tc.token)
		if seq != tc.wantSeq || backward != tc.wantBackward {
			t.Errorf("parseTimelineCursor(%q) = (%d,%t), want (%d,%t)", tc.token, seq, backward, tc.wantSeq, tc.wantBackward)
		}
	}
}

func equalSeqs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
