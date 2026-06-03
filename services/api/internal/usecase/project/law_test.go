package project

import (
	"context"
	"testing"
	"time"

	leg "s4rciv.org/api/internal/domain/legislative"
	obs "s4rciv.org/api/internal/domain/observation"
	"s4rciv.org/api/internal/port"
)

// fakeLawNormalizer records every ParseLaw call so a test can assert the law
// projector never hands a foreign (non-egov) snapshot to the law parser.
type fakeLawNormalizer struct{ parsed [][]byte }

func (n *fakeLawNormalizer) ParseLaw(content []byte) (leg.LawContent, error) {
	n.parsed = append(n.parsed, content)
	return leg.LawContent{}, nil
}

type fakeLawStore struct{ batches []port.LawProjectionBatch }

func (s *fakeLawStore) ApplyLaw(_ context.Context, b port.LawProjectionBatch) error {
	s.batches = append(s.batches, b)
	return nil
}
func (s *fakeLawStore) TruncateLaws(context.Context) error { s.batches = nil; return nil }

// Symmetric to TestProjectorSkipsForeignStreams: the law projector folds the same
// shared observation log, which also carries kokkai meeting snapshots (JSON). It
// must project only egov-law streams — never feed a kokkai snapshot to ParseLaw —
// yet still advance its offset past the foreign event, otherwise it wedges on the
// first kokkai event and re-fails every poll.
func TestLawProjectorSkipsForeignStreams(t *testing.T) {
	reader := fakeReader{evs: []port.ObservedEvent{
		// Foreign event first: a kokkai meeting snapshot (JSON).
		{Seq: 6, StreamID: "kokkai:121815254X00120240115", Type: obs.ResourceObserved,
			ObservedAt: time.Unix(1, 0).UTC(), SnapshotBytes: []byte(`{"meetingRecord":[]}`)},
		{Seq: 7, StreamID: "egov-law:322M40000100023", Type: obs.ResourceObserved,
			ObservedAt: time.Unix(2, 0).UTC(), SnapshotBytes: []byte("<Law>…</Law>")},
	}}
	norm := &fakeLawNormalizer{}
	store := &fakeLawStore{}
	offsets := &fakeOffsets{}
	p := NewLaw(reader, norm, store, offsets, "egov-law")

	n, err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("Run errored on a foreign stream: %v", err)
	}
	if n != 1 || len(store.batches) != 1 {
		t.Fatalf("projected %d / batches %d, want 1/1 (only the egov-law event)", n, len(store.batches))
	}
	// The kokkai snapshot must never reach ParseLaw.
	if len(norm.parsed) != 1 {
		t.Fatalf("ParseLaw called %d times, want 1 (kokkai snapshot must be skipped)", len(norm.parsed))
	}
	b := store.batches[0]
	if b.Law.LawID != "322M40000100023" || b.Law.StreamID != "egov-law:322M40000100023" {
		t.Fatalf("projected the wrong stream: %+v", b.Law)
	}
	// Offset advanced past the foreign event, not wedged at 5.
	if offsets.off != 7 {
		t.Fatalf("offset = %d, want 7 (advanced past the kokkai event)", offsets.off)
	}
}
