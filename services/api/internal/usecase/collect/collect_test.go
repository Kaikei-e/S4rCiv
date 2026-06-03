package collect

import (
	"context"
	"testing"
	"time"

	obs "s4rciv.org/api/internal/domain/observation"
	"s4rciv.org/api/internal/port"
)

// --- fakes ------------------------------------------------------------------

type fakeLog struct{ events map[string][]port.AppendCmd }

func newFakeLog() *fakeLog { return &fakeLog{events: map[string][]port.AppendCmd{}} }

func (f *fakeLog) EnsureStream(context.Context, port.Stream) error { return nil }

func (f *fakeLog) StreamState(_ context.Context, streamID string) (obs.StreamState, error) {
	evs := f.events[streamID]
	if len(evs) == 0 {
		return obs.StreamState{}, nil
	}
	st := obs.StreamState{Exists: true, LastType: evs[len(evs)-1].Type}
	for i := len(evs) - 1; i >= 0; i-- {
		if evs[i].Snapshot != nil {
			d := evs[i].Snapshot.ContentHash
			st.LastContentHash = &d
			break
		}
	}
	return st, nil
}

func (f *fakeLog) Append(_ context.Context, cmd port.AppendCmd) (int64, error) {
	f.events[cmd.Stream.StreamID] = append(f.events[cmd.Stream.StreamID], cmd)
	return int64(len(f.events[cmd.Stream.StreamID])), nil
}

type fakeFetcher struct {
	results []port.FetchResult
	i       int
}

func (f *fakeFetcher) Fetch(context.Context, port.Watch) (port.FetchResult, error) {
	r := f.results[f.i]
	f.i++
	return r, nil
}

type fakeControl struct {
	due      []port.Watch
	upserted []port.Watch
}

func (c *fakeControl) Source(context.Context, string) (port.SourceConfig, error) {
	return port.SourceConfig{}, nil
}
func (c *fakeControl) DueWatches(context.Context, string, time.Time, int) ([]port.Watch, error) {
	return c.due, nil
}
func (c *fakeControl) UpsertWatch(_ context.Context, w port.Watch) error {
	c.upserted = append(c.upserted, w)
	return nil
}
func (c *fakeControl) MarkPolled(context.Context, string, time.Time, time.Time, bool) error {
	return nil
}

type fakeLister struct{ refs []port.MeetingRef }

func (l fakeLister) ListMeetings(context.Context, port.ListScope) ([]port.MeetingRef, error) {
	return l.refs, nil
}

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time { c.t = c.t.Add(time.Second); return c.t }

type fakeIDs struct{ n int }

func (g *fakeIDs) NewID() string { g.n++; return "id-" + string(rune('0'+g.n)) }

// --- helpers ----------------------------------------------------------------

func present(s string) port.FetchResult {
	ch := obs.SumBytes([]byte(s))
	return port.FetchResult{Present: true, Snapshot: &port.Snapshot{ContentHash: ch, Bytes: []byte(s), ByteSize: int64(len(s))}}
}

func absent() port.FetchResult { return port.FetchResult{Present: false} }

func newCollector(log port.EventLog, f port.ResourceFetcher, ctrl port.ControlStore, l port.MeetingLister) *Collector {
	return New(log, f, ctrl, l, &fakeClock{t: time.Unix(1_700_000_000, 0).UTC()}, &fakeIDs{}, Config{FetcherVersion: "test/0.1.0"})
}

// --- tests ------------------------------------------------------------------

func TestPollStreamLifecycle(t *testing.T) {
	log := newFakeLog()
	fetch := &fakeFetcher{results: []port.FetchResult{
		present("A"), // observed
		present("A"), // unchanged -> no emit
		present("B"), // changed
		absent(),     // vanished
		absent(),     // still gone -> no emit
		present("A"), // restored
	}}
	c := newCollector(log, fetch, &fakeControl{}, fakeLister{})
	w := port.Watch{StreamID: "kokkai:X", Source: "kokkai", SourceLocalKey: "X"}

	wantEmit := []bool{true, false, true, true, false, true}
	for i, we := range wantEmit {
		ok, err := c.PollStream(context.Background(), w)
		if err != nil {
			t.Fatalf("poll %d: %v", i, err)
		}
		if ok != we {
			t.Fatalf("poll %d: emit=%v want %v", i, ok, we)
		}
	}

	got := log.events["kokkai:X"]
	wantTypes := []obs.EventType{obs.ResourceObserved, obs.ResourceChanged, obs.ResourceVanished, obs.ResourceRestored}
	if len(got) != len(wantTypes) {
		t.Fatalf("emitted %d events, want %d", len(got), len(wantTypes))
	}
	for i, wt := range wantTypes {
		if got[i].Type != wt {
			t.Fatalf("event %d type = %v, want %v", i, got[i].Type, wt)
		}
	}
	// Vanished carries no snapshot; the content chain skips it on restore.
	if got[2].Snapshot != nil {
		t.Fatal("vanished event must not carry a snapshot")
	}
	if got[2].PrevContentHash == nil {
		t.Fatal("vanished should link to the pre-vanish snapshot in the content chain")
	}
}

func TestPollOnceCountsEmissions(t *testing.T) {
	log := newFakeLog()
	ctrl := &fakeControl{due: []port.Watch{{StreamID: "kokkai:X", Source: "kokkai", SourceLocalKey: "X"}}}
	fetch := &fakeFetcher{results: []port.FetchResult{present("A")}}
	c := newCollector(log, fetch, ctrl, fakeLister{})

	n, err := c.PollOnce(context.Background(), "kokkai", 10)
	if err != nil {
		t.Fatalf("PollOnce: %v", err)
	}
	if n != 1 {
		t.Fatalf("emitted = %d, want 1", n)
	}
}

func TestDiscoverUpsertsWatches(t *testing.T) {
	ctrl := &fakeControl{}
	lister := fakeLister{refs: []port.MeetingRef{
		{StreamID: "kokkai:A", SourceLocalKey: "A"},
		{StreamID: "kokkai:B", SourceLocalKey: "B"},
	}}
	c := newCollector(newFakeLog(), &fakeFetcher{}, ctrl, lister)

	n, err := c.Discover(context.Background(), port.ListScope{From: "2024-01-01", Until: "2024-01-31"})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if n != 2 || len(ctrl.upserted) != 2 {
		t.Fatalf("discovered %d / upserted %d, want 2/2", n, len(ctrl.upserted))
	}
}
