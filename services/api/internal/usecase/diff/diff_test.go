package diff

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	obs "s4rciv.org/api/internal/domain/observation"
	"s4rciv.org/api/internal/port"
)

type fakeReader struct {
	evs  []port.ObservedEvent
	prev map[string][]byte // streamID -> prev snapshot bytes
}

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

func (r fakeReader) PrevContentSnapshot(_ context.Context, streamID string, _ int64) ([]byte, bool, error) {
	b, ok := r.prev[streamID]
	return b, ok, nil
}

type fakeClient struct {
	calls int
	last  struct{ prev, curr []byte }
	res   port.DiffResult
}

func (c *fakeClient) ComputeChange(_ context.Context, prev, curr []byte, _, _ string) (port.DiffResult, error) {
	c.calls++
	c.last.prev, c.last.curr = prev, curr
	return c.res, nil
}

type fakeChangeStore struct{ records []port.ChangeRecord }

func (s *fakeChangeStore) ApplyChange(_ context.Context, r port.ChangeRecord) error {
	s.records = append(s.records, r)
	return nil
}
func (s *fakeChangeStore) TruncateEgov(context.Context) error { s.records = nil; return nil }

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
	o.off, o.rebuilt = 0, true
	return nil
}

func TestChangedEventComputesAndStores(t *testing.T) {
	stream := "egov-law:415AC0000000057"
	reader := fakeReader{
		evs: []port.ObservedEvent{
			{Seq: 1, StreamID: stream, Type: obs.ResourceObserved,
				ObservedAt: time.Unix(1, 0).UTC(), SnapshotBytes: []byte("<Law>v1</Law>")},
			{Seq: 2, StreamID: stream, Type: obs.ResourceChanged,
				ObservedAt: time.Unix(2, 0).UTC(), SnapshotBytes: []byte("<Law>v2</Law>")},
		},
		prev: map[string][]byte{stream: []byte("<Law>v1</Law>")},
	}
	client := &fakeClient{res: port.DiffResult{
		DifferVersion: "differ/0.1.0", Classification: "substantive", ClassConfidence: "high",
		NodeChanges: []port.NodeChange{{EID: "art_9", Op: "modified", NodeType: "article", Num: "9", PrevText: "old", CurrText: "new"}},
	}}
	store := &fakeChangeStore{}
	offsets := &fakeOffsets{}
	d := New(reader, client, store, offsets, "egov-differ")

	n, err := d.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n != 1 || client.calls != 1 || len(store.records) != 1 {
		t.Fatalf("processed=%d calls=%d records=%d, want 1/1/1", n, client.calls, len(store.records))
	}

	// ComputeChange paired prev (v1) and curr (v2).
	if string(client.last.prev) != "<Law>v1</Law>" || string(client.last.curr) != "<Law>v2</Law>" {
		t.Fatalf("pairing wrong: prev=%q curr=%q", client.last.prev, client.last.curr)
	}

	rec := store.records[0]
	if rec.ObservationSeq != 2 || rec.DifferVersion != "differ/0.1.0" ||
		rec.Classification != "substantive" || rec.ClassConfidence != "high" {
		t.Fatalf("record meta = %+v", rec)
	}
	var payload struct {
		LawID       string `json:"law_id"`
		NodeChanges []struct {
			EID, Op, NodeType, Num, PrevText, CurrText string
		} `json:"node_changes"`
	}
	if err := json.Unmarshal(rec.DiffJSON, &payload); err != nil {
		t.Fatalf("diff json: %v", err)
	}
	if payload.LawID != "415AC0000000057" || len(payload.NodeChanges) != 1 || payload.NodeChanges[0].EID != "art_9" {
		t.Fatalf("diff payload = %+v", payload)
	}

	if offsets.off != 2 {
		t.Fatalf("offset = %d, want 2", offsets.off)
	}
}

// An administrative change with zero structural node deltas must still serialize
// node_changes as a JSON array. A nil Go slice marshals to `null` (a JSON scalar),
// and the timeline read model expands this field with jsonb_array_elements, which
// raises SQLSTATE 22023 on a scalar and fails the whole list query — one such row
// would blank the public timeline (regression guard for ADR-000024).
func TestEmptyNodeChangesSerializeAsArrayNotNull(t *testing.T) {
	stream := "egov-law:327R00000001003"
	reader := fakeReader{
		evs: []port.ObservedEvent{
			{Seq: 1, StreamID: stream, Type: obs.ResourceChanged,
				SnapshotBytes: []byte("<Law>v2</Law>")},
		},
		prev: map[string][]byte{stream: []byte("<Law>v1</Law>")},
	}
	// Differ reports an administrative change with NO node changes (nil slice).
	client := &fakeClient{res: port.DiffResult{
		DifferVersion: "differ/0.1.0", Classification: "administrative", ClassConfidence: "high",
		NodeChanges: nil,
	}}
	store := &fakeChangeStore{}
	d := New(reader, client, store, &fakeOffsets{}, "egov-differ")

	if _, err := d.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(store.records) != 1 {
		t.Fatalf("records=%d, want 1", len(store.records))
	}
	raw := string(store.records[0].DiffJSON)
	if strings.Contains(raw, `"node_changes":null`) {
		t.Fatalf("node_changes serialized as JSON null (scalar) — poisons timeline: %s", raw)
	}
	if !strings.Contains(raw, `"node_changes":[]`) {
		t.Fatalf("node_changes not an empty array: %s", raw)
	}
}

func TestObservedAndVanishedAreSkipped(t *testing.T) {
	stream := "egov-law:X"
	reader := fakeReader{
		evs: []port.ObservedEvent{
			{Seq: 1, StreamID: stream, Type: obs.ResourceObserved, SnapshotBytes: []byte("<Law/>")},
			{Seq: 2, StreamID: stream, Type: obs.ResourceVanished}, // no content
		},
		prev: map[string][]byte{stream: []byte("<Law/>")},
	}
	client := &fakeClient{}
	store := &fakeChangeStore{}
	d := New(reader, client, store, &fakeOffsets{}, "egov-differ")

	n, err := d.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n != 0 || client.calls != 0 || len(store.records) != 0 {
		t.Fatalf("processed=%d calls=%d records=%d, want all 0", n, client.calls, len(store.records))
	}
}

func TestChangedWithoutPredecessorIsSkipped(t *testing.T) {
	stream := "egov-law:Y"
	reader := fakeReader{
		evs: []port.ObservedEvent{
			{Seq: 7, StreamID: stream, Type: obs.ResourceChanged, SnapshotBytes: []byte("<Law/>")},
		},
		prev: map[string][]byte{}, // no predecessor recorded
	}
	client := &fakeClient{}
	store := &fakeChangeStore{}
	d := New(reader, client, store, &fakeOffsets{}, "egov-differ")

	n, _ := d.Run(context.Background())
	if n != 0 || client.calls != 0 {
		t.Fatalf("processed=%d calls=%d, want 0/0 (no prior snapshot)", n, client.calls)
	}
}

func TestReprojectResetsAndReplays(t *testing.T) {
	stream := "egov-law:Z"
	reader := fakeReader{
		evs: []port.ObservedEvent{
			{Seq: 5, StreamID: stream, Type: obs.ResourceChanged, SnapshotBytes: []byte("<Law>v2</Law>")},
		},
		prev: map[string][]byte{stream: []byte("<Law>v1</Law>")},
	}
	client := &fakeClient{res: port.DiffResult{DifferVersion: "d", Classification: "administrative", ClassConfidence: "low"}}
	store := &fakeChangeStore{}
	offsets := &fakeOffsets{off: 5} // pretend already caught up

	d := New(reader, client, store, offsets, "egov-differ")
	if n, _ := d.Run(context.Background()); n != 0 {
		t.Fatalf("caught-up run processed %d, want 0", n)
	}
	n, err := d.Reproject(context.Background())
	if err != nil {
		t.Fatalf("Reproject: %v", err)
	}
	if !offsets.rebuilt || n != 1 {
		t.Fatalf("reproject rebuilt=%v processed=%d, want true/1", offsets.rebuilt, n)
	}
}
