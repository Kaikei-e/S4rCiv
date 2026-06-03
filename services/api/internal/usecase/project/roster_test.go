package project

import (
	"context"
	"testing"

	leg "s4rciv.org/api/internal/domain/legislative"
	"s4rciv.org/api/internal/port"
)

type fakeRosterNormalizer struct{ entries []leg.RosterEntry }

func (n fakeRosterNormalizer) ParseRoster([]byte) ([]leg.RosterEntry, error) { return n.entries, nil }

type fakeRosterStore struct{ batches []port.RosterProjectionBatch }

func (s *fakeRosterStore) ApplyRoster(_ context.Context, b port.RosterProjectionBatch) error {
	s.batches = append(s.batches, b)
	return nil
}
func (s *fakeRosterStore) TruncateRoster(context.Context) error { s.batches = nil; return nil }

// The roster projector must fold ONLY giin-roster streams (it shares the global
// observation log with kokkai/egov), skip ResourceVanished (no snapshot), advance
// the offset past every event, and carry stream_id + provenance into the batch.
func TestRosterProjectorFoldsOnlyRosterStreams(t *testing.T) {
	rosterStream := leg.RosterStreamID("shugiin-1giin")
	evs := []port.ObservedEvent{
		{Seq: 1, StreamID: "kokkai:121815254X00120240115", SnapshotBytes: []byte("x")}, // other source → skip
		{Seq: 2, StreamID: rosterStream, SnapshotBytes: []byte("<html>")},              // roster → project
		{Seq: 3, StreamID: rosterStream, SnapshotBytes: nil},                           // vanished → skip
	}
	norm := fakeRosterNormalizer{entries: []leg.RosterEntry{
		{PersonID: "p:1", House: leg.HouseRepresentatives, DistrictCode: "1301"},
	}}
	store := &fakeRosterStore{}
	offsets := &fakeOffsets{}
	p := NewRoster(fakeReader{evs: evs}, norm, store, offsets, "giin-roster", "giin-roster:")

	n, err := p.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("projected %d, want 1 (only the roster snapshot)", n)
	}
	if len(store.batches) != 1 {
		t.Fatalf("applied %d batches, want 1", len(store.batches))
	}
	b := store.batches[0]
	if b.StreamID != rosterStream || b.ObservationSeq != 2 || len(b.Entries) != 1 {
		t.Fatalf("batch mismatch: %+v", b)
	}
	if offsets.off != 3 {
		t.Fatalf("offset advanced to %d, want 3 (past all events)", offsets.off)
	}
}
