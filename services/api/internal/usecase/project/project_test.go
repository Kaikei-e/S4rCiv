package project

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"s4rciv.org/api/internal/blob"
	obs "s4rciv.org/api/internal/domain/observation"
	"s4rciv.org/api/internal/gateway/kokkai"
	"s4rciv.org/api/internal/port"
)

// canonical snapshot bytes built from the gateway fixture, so the projector runs
// against exactly what the collector would have stored.
func fixtureSnapshot(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "gateway", "kokkai", "testdata", "meeting.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	g := kokkai.New(stubGetter{body})
	r, err := g.Fetch(context.Background(), port.Watch{SourceLocalKey: "121815254X00120240115"})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	raw, err := blob.Decompress(r.Snapshot.Bytes)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	return raw
}

type stubGetter struct{ body []byte }

func (s stubGetter) Get(context.Context, string, url.Values) ([]byte, int, error) {
	return s.body, 200, nil
}

type fakeReader struct{ evs []port.ObservedEvent }

func (r fakeReader) EventsSince(_ context.Context, after int64, limit int) ([]port.ObservedEvent, error) {
	var out []port.ObservedEvent
	for _, e := range r.evs {
		if e.Seq > after {
			out = append(out, e)
			if len(out) == limit {
				break
			}
		}
	}
	return out, nil
}

func (r fakeReader) PrevContentSnapshot(context.Context, string, int64) ([]byte, bool, error) {
	return nil, false, nil
}

type fakeOffsets struct {
	off     int64
	rebuilt bool
}

func (o *fakeOffsets) Offset(context.Context, string) (int64, error) { return o.off, nil }
func (o *fakeOffsets) SetOffset(_ context.Context, _ string, seq int64) error {
	o.off = seq
	return nil
}
func (o *fakeOffsets) BeginRebuild(context.Context, string) error {
	o.off = 0
	o.rebuilt = true
	return nil
}

type fakeStore struct{ batches []port.ProjectionBatch }

func (s *fakeStore) ApplyMeeting(_ context.Context, b port.ProjectionBatch) error {
	s.batches = append(s.batches, b)
	return nil
}
func (s *fakeStore) Truncate(context.Context) error { s.batches = nil; return nil }

func TestProjectorFoldsMeetingPopoloAndVotes(t *testing.T) {
	snap := fixtureSnapshot(t)
	reader := fakeReader{evs: []port.ObservedEvent{
		{Seq: 1, StreamID: "kokkai:121815254X00120240115", Type: obs.ResourceObserved,
			ObservedAt: time.Unix(1_700_000_000, 0).UTC(), SnapshotBytes: snap},
	}}
	store := &fakeStore{}
	offsets := &fakeOffsets{}
	p := New(reader, kokkai.New(nil), store, offsets, "kokkai")

	n, err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n != 1 || len(store.batches) != 1 {
		t.Fatalf("projected %d / batches %d, want 1/1", n, len(store.batches))
	}
	b := store.batches[0]

	if b.Meeting.IssueID != "121815254X00120240115" || b.Meeting.House != "衆議院" {
		t.Fatalf("meeting = %+v", b.Meeting)
	}
	if len(b.Speeches) != 2 {
		t.Fatalf("speeches = %d, want 2", len(b.Speeches))
	}
	// 会議録情報 is a pseudo-speaker: only 山田太郎 becomes a Person.
	if len(b.Persons) != 1 || b.Persons[0].Name != "山田太郎" {
		t.Fatalf("persons = %+v, want only 山田太郎", b.Persons)
	}
	if len(b.Organizations) != 1 || b.Organizations[0].Name != "自由民主党" {
		t.Fatalf("orgs = %+v", b.Organizations)
	}
	if len(b.VoteEvents) != 1 {
		t.Fatalf("vote events = %d, want 1", len(b.VoteEvents))
	}
	ve := b.VoteEvents[0]
	if ve.Result != "passed" || ve.Confidence != "high" {
		t.Fatalf("vote: result=%s confidence=%s", ve.Result, ve.Confidence)
	}
	if ve.ExtractorVersion == "" {
		t.Fatal("vote event must carry extractor_version (reproject-safe)")
	}

	// Conservative voter linking: 山田太郎 (a speaker) resolves; others do not.
	linked, unlinked := 0, 0
	for _, v := range ve.Votes {
		if v.PersonID != "" {
			linked++
			if v.VoterName != "山田太郎" {
				t.Fatalf("unexpected linked voter %q", v.VoterName)
			}
		} else {
			unlinked++
		}
	}
	if linked != 1 || unlinked != 3 {
		t.Fatalf("linked=%d unlinked=%d, want 1/3", linked, unlinked)
	}

	// Offset advanced; a second Run is a no-op (idempotent catch-up).
	if offsets.off != 1 {
		t.Fatalf("offset = %d, want 1", offsets.off)
	}
	n2, _ := p.Run(context.Background())
	if n2 != 0 {
		t.Fatalf("second run projected %d, want 0", n2)
	}
}

func TestReprojectResetsAndReplays(t *testing.T) {
	snap := fixtureSnapshot(t)
	reader := fakeReader{evs: []port.ObservedEvent{
		{Seq: 5, Type: obs.ResourceObserved, SnapshotBytes: snap, ObservedAt: time.Unix(1, 0).UTC()},
	}}
	store := &fakeStore{}
	offsets := &fakeOffsets{off: 5} // pretend already caught up
	p := New(reader, kokkai.New(nil), store, offsets, "kokkai")

	if n, _ := p.Run(context.Background()); n != 0 {
		t.Fatalf("caught-up run projected %d, want 0", n)
	}
	n, err := p.Reproject(context.Background())
	if err != nil {
		t.Fatalf("Reproject: %v", err)
	}
	if !offsets.rebuilt || n != 1 {
		t.Fatalf("reproject rebuilt=%v projected=%d, want true/1", offsets.rebuilt, n)
	}
}
