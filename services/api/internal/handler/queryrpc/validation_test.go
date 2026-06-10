package queryrpc

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"

	queryv1 "s4rciv.org/api/gen/s4rciv/query/v1"
)

func TestParseOffsetCapsForgedTokens(t *testing.T) {
	cases := []struct {
		token string
		want  int
	}{
		{"", 0},
		{"0", 0},
		{"150", 150},
		{"-1", 0},
		{"garbage", 0},
		{"100000", maxPageOffset}, // exactly at the cap
		{"100001", maxPageOffset}, // clamped, not reset to 0
		{"999999999", maxPageOffset},
	}
	for _, tc := range cases {
		if got := parseOffset(tc.token); got != tc.want {
			t.Errorf("parseOffset(%q) = %d, want %d", tc.token, got, tc.want)
		}
	}
}

// Malformed since/until must be rejected as CodeInvalidArgument BEFORE the driver,
// where the ::timestamptz cast would fail (SQLSTATE 22007) and surface as
// CodeInternal. Empty stays allowed (= no filter), valid RFC3339 passes through.
func TestListTimelineValidatesSinceUntil(t *testing.T) {
	h := New(fakeTimelineReader{seqsDesc: []int64{2, 1}}, nil)
	cases := []struct {
		name         string
		since, until string
		wantCode     connect.Code // 0 = success
	}{
		{"both empty", "", "", 0},
		{"valid range", "2026-01-01T00:00:00Z", "2026-02-01T00:00:00+09:00", 0},
		{"since not a timestamp", "yesterday", "", connect.CodeInvalidArgument},
		{"since date-only", "2026-01-01", "", connect.CodeInvalidArgument},
		{"until not a timestamp", "", "not-a-time", connect.CodeInvalidArgument},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := h.ListTimeline(context.Background(), connect.NewRequest(&queryv1.ListTimelineRequest{
				Since: tc.since, Until: tc.until,
			}))
			if tc.wantCode == 0 {
				if err != nil {
					t.Fatalf("ListTimeline: %v", err)
				}
				return
			}
			if connect.CodeOf(err) != tc.wantCode {
				t.Fatalf("code = %v (err %v), want %v", connect.CodeOf(err), err, tc.wantCode)
			}
		})
	}
}

// Caller-supplied identifiers are bounded to maxIDLen bytes, so an over-long ID is
// rejected up front instead of reaching the driver and being reflected back in a
// NotFound message. Exactly maxIDLen passes validation (and then misses → NotFound
// from the fake reader); empty stays required.
func TestGetMeetingBoundsIssueIDLength(t *testing.T) {
	h := New(fakeTimelineReader{}, nil)
	cases := []struct {
		name     string
		issueID  string
		wantCode connect.Code
	}{
		{"empty", "", connect.CodeInvalidArgument},
		{"at the bound", strings.Repeat("a", maxIDLen), connect.CodeNotFound},
		{"over the bound", strings.Repeat("a", maxIDLen+1), connect.CodeInvalidArgument},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := h.GetMeeting(context.Background(), connect.NewRequest(&queryv1.GetMeetingRequest{
				IssueId: tc.issueID,
			}))
			if connect.CodeOf(err) != tc.wantCode {
				t.Fatalf("code = %v (err %v), want %v", connect.CodeOf(err), err, tc.wantCode)
			}
		})
	}
}

// The over-long rejection must not echo the submitted ID back (bounding reflected
// bytes is the point of the check).
func TestOverlongIDNotEchoed(t *testing.T) {
	h := New(fakeTimelineReader{}, nil)
	long := strings.Repeat("z", 500)
	_, err := h.GetStreamVerification(context.Background(), connect.NewRequest(&queryv1.GetStreamVerificationRequest{
		StreamId: long,
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument", connect.CodeOf(err))
	}
	if strings.Contains(err.Error(), long) {
		t.Fatalf("over-long stream_id is echoed back in the error: %v", err)
	}
}
