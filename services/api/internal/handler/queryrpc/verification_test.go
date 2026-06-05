package queryrpc

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"

	queryv1 "s4rciv.org/api/gen/s4rciv/query/v1"
	obs "s4rciv.org/api/internal/domain/observation"
	"s4rciv.org/api/internal/port"
)

// fakeVerifReader implements port.QueryReader but only StreamVerification is real;
// the rest return zero values (this handler test exercises one RPC).
type fakeVerifReader struct {
	view  port.StreamVerificationView
	found bool
}

func (f fakeVerifReader) StreamVerification(context.Context, string) (port.StreamVerificationView, bool, error) {
	return f.view, f.found, nil
}
func (fakeVerifReader) Meeting(context.Context, string) (port.MeetingView, []port.SpeechView, bool, error) {
	return port.MeetingView{}, nil, false, nil
}
func (fakeVerifReader) ListMeetings(context.Context, int, string, int, int) ([]port.MeetingView, error) {
	return nil, nil
}
func (fakeVerifReader) VoteEvent(context.Context, string) (port.VoteEventView, bool, error) {
	return port.VoteEventView{}, false, nil
}
func (fakeVerifReader) ListVoteEvents(context.Context, port.VoteEventFilter) (int, []port.VoteEventSummaryView, error) {
	return 0, nil, nil
}
func (fakeVerifReader) ListTimeline(context.Context, port.TimelineFilter) ([]port.TimelineItemView, error) {
	return nil, nil
}
func (fakeVerifReader) CountTimeline(context.Context, port.TimelineFilter, int64) (int, int, error) {
	return 0, 0, nil
}
func (fakeVerifReader) VotesByPerson(context.Context, string, int, int) (port.LegislatorVotes, bool, error) {
	return port.LegislatorVotes{}, false, nil
}
func (fakeVerifReader) ListSangiinVoteEvents(context.Context, int, int, int) (int, []port.SangiinVoteEventSummaryView, error) {
	return 0, nil, nil
}
func (fakeVerifReader) GetSangiinVoteMap(context.Context, string) (port.SangiinVoteMapView, bool, error) {
	return port.SangiinVoteMapView{}, false, nil
}

func sampleView() port.StreamVerificationView {
	content := obs.SumBytes([]byte("snapshot-1"))
	return port.StreamVerificationView{
		StreamID:   "kokkai:100000000X00120260101",
		Source:     "kokkai",
		AlgVersion: obs.AlgVersion,
		Events: []port.VerifiableEventView{{
			Seq: 42,
			Facts: obs.EventFacts{
				EventID:        "00000000-0000-7000-8000-100000000000",
				StreamID:       "kokkai:100000000X00120260101",
				StreamSeq:      1,
				Type:           obs.ResourceObserved,
				Source:         "kokkai",
				FetcherVersion: "kokkai-collector/0.1-test",
				ObservedAt:     time.Date(2026, 6, 2, 9, 0, 0, 0, time.UTC),
				ContentHash:    &content,
			},
			LogHash: "abc123",
		}},
	}
}

func TestGetStreamVerification(t *testing.T) {
	t.Run("maps facts to canonical HashableEvent and echoes stored log_hash", func(t *testing.T) {
		h := New(fakeVerifReader{view: sampleView(), found: true}, nil)
		resp, err := h.GetStreamVerification(context.Background(),
			connect.NewRequest(&queryv1.GetStreamVerificationRequest{StreamId: "kokkai:100000000X00120260101"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		msg := resp.Msg
		if msg.GetAlgVersion() != obs.AlgVersion {
			t.Errorf("alg_version = %q, want %q", msg.GetAlgVersion(), obs.AlgVersion)
		}
		if len(msg.GetEvents()) != 1 {
			t.Fatalf("events = %d, want 1", len(msg.GetEvents()))
		}
		ev := msg.GetEvents()[0]
		if ev.GetSeq() != 42 {
			t.Errorf("seq = %d, want 42", ev.GetSeq())
		}
		if ev.GetLogHash() != "abc123" {
			t.Errorf("log_hash = %q, want abc123", ev.GetLogHash())
		}
		he := ev.GetHashable()
		if he == nil {
			t.Fatal("hashable is nil — EventFacts.Hashable() not mapped")
		}
		// The canonical projection must be fully populated (the bytes a verifier re-hashes).
		if he.GetEventId() != "00000000-0000-7000-8000-100000000000" {
			t.Errorf("hashable.event_id = %q", he.GetEventId())
		}
		if he.GetObservedAt() != "2026-06-02T09:00:00Z" {
			t.Errorf("hashable.observed_at = %q, want canonical RFC3339", he.GetObservedAt())
		}
		if he.GetContentHash() == "" {
			t.Error("hashable.content_hash empty — content ref not projected")
		}
		if msg.GetHasCheckpoint() {
			t.Error("has_checkpoint = true, want false (no checkpoint in view)")
		}
	})

	t.Run("maps a covering checkpoint when present", func(t *testing.T) {
		v := sampleView()
		v.Checkpoint = &port.CheckpointView{
			ThroughSeq: 100, TreeSize: 100, RootHash: "deadbeef",
			AlgVersion: obs.AlgVersion, Signed: false, RecordedAt: time.Now(),
		}
		h := New(fakeVerifReader{view: v, found: true}, nil)
		resp, err := h.GetStreamVerification(context.Background(),
			connect.NewRequest(&queryv1.GetStreamVerificationRequest{StreamId: "x"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.Msg.GetHasCheckpoint() {
			t.Fatal("has_checkpoint = false, want true")
		}
		cp := resp.Msg.GetCheckpoint()
		if cp.GetThroughSeq() != 100 || cp.GetSigned() {
			t.Errorf("checkpoint = %+v, want through_seq 100 / signed false", cp)
		}
	})

	t.Run("empty stream_id is InvalidArgument", func(t *testing.T) {
		h := New(fakeVerifReader{}, nil)
		_, err := h.GetStreamVerification(context.Background(),
			connect.NewRequest(&queryv1.GetStreamVerificationRequest{StreamId: ""}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("code = %v, want InvalidArgument", connect.CodeOf(err))
		}
	})

	t.Run("unknown stream is NotFound", func(t *testing.T) {
		h := New(fakeVerifReader{found: false}, nil)
		_, err := h.GetStreamVerification(context.Background(),
			connect.NewRequest(&queryv1.GetStreamVerificationRequest{StreamId: "kokkai:missing"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("code = %v, want NotFound", connect.CodeOf(err))
		}
	})
}
