// Package diff is the change-detection usecase for the egov-law adapter: it folds
// observation events in seq order and, for each ResourceChanged in an egov-law
// stream, pairs the new snapshot with its predecessor, asks the Rust differ to
// compute the structural change (ADR-000005 — Go owns persistence, not the diff),
// and writes interpretation.change. Driven only through ports, so a reproject
// from seq 0 reproduces the same change rows.
package diff

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	leg "s4rciv.org/api/internal/domain/legislative"
	obs "s4rciv.org/api/internal/domain/observation"
	"s4rciv.org/api/internal/port"
)

const DefaultBatchSize = 100

type Differ struct {
	reader    port.EventReader
	client    port.DiffClient
	store     port.ChangeStore
	offsets   port.ProjectorOffset
	name      string
	mediaType string
	batchSize int
}

func New(reader port.EventReader, client port.DiffClient, store port.ChangeStore, offsets port.ProjectorOffset, name string) *Differ {
	return &Differ{
		reader: reader, client: client, store: store, offsets: offsets,
		name: name, mediaType: "application/xml", batchSize: DefaultBatchSize,
	}
}

// diffJSON is the serialized shape persisted in interpretation.change.diff.
type diffJSON struct {
	LawID       string           `json:"law_id"`
	NodeChanges []nodeChangeJSON `json:"node_changes"`
}

type nodeChangeJSON struct {
	EID      string `json:"eid"`
	Op       string `json:"op"`
	NodeType string `json:"node_type"`
	Num      string `json:"num"`
	PrevText string `json:"prev_text"`
	CurrText string `json:"curr_text"`
}

// Run folds every observation event past the stored offset. Returns how many
// changes were computed and written.
func (d *Differ) Run(ctx context.Context) (int, error) {
	off, err := d.offsets.Offset(ctx, d.name)
	if err != nil {
		return 0, fmt.Errorf("read offset: %w", err)
	}
	processed := 0
	for {
		evs, err := d.reader.EventsSince(ctx, off, d.batchSize)
		if err != nil {
			return processed, fmt.Errorf("read events: %w", err)
		}
		if len(evs) == 0 {
			return processed, nil
		}
		for _, ev := range evs {
			ok, err := d.handle(ctx, ev)
			if err != nil {
				return processed, fmt.Errorf("diff seq %d: %w", ev.Seq, err)
			}
			if ok {
				processed++
			}
			off = ev.Seq
		}
		if err := d.offsets.SetOffset(ctx, d.name, off); err != nil {
			return processed, fmt.Errorf("set offset: %w", err)
		}
		if len(evs) < d.batchSize {
			return processed, nil
		}
	}
}

// Reproject truncates the egov change rows, resets the offset, and replays from 0.
func (d *Differ) Reproject(ctx context.Context) (int, error) {
	if err := d.offsets.BeginRebuild(ctx, d.name); err != nil {
		return 0, fmt.Errorf("begin rebuild: %w", err)
	}
	return d.Run(ctx)
}

// handle computes and stores a change for a ResourceChanged egov-law event.
// ResourceObserved (no prior) and ResourceVanished (no content) are skipped.
func (d *Differ) handle(ctx context.Context, ev port.ObservedEvent) (bool, error) {
	if ev.Type != obs.ResourceChanged || ev.SnapshotBytes == nil || !isEgovStream(ev.StreamID) {
		return false, nil
	}
	prev, found, err := d.reader.PrevContentSnapshot(ctx, ev.StreamID, ev.Seq)
	if err != nil {
		return false, fmt.Errorf("prev snapshot: %w", err)
	}
	if !found {
		return false, nil // no predecessor to diff against
	}

	res, err := d.client.ComputeChange(ctx, prev, ev.SnapshotBytes, ev.StreamID, d.mediaType)
	if err != nil {
		return false, fmt.Errorf("compute change: %w", err)
	}

	payload := diffJSON{LawID: lawIDOf(ev.StreamID)}
	for _, nc := range res.NodeChanges {
		payload.NodeChanges = append(payload.NodeChanges, nodeChangeJSON{
			EID: nc.EID, Op: nc.Op, NodeType: nc.NodeType, Num: nc.Num,
			PrevText: nc.PrevText, CurrText: nc.CurrText,
		})
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("marshal diff: %w", err)
	}

	if err := d.store.ApplyChange(ctx, port.ChangeRecord{
		ObservationSeq:  ev.Seq,
		DifferVersion:   res.DifferVersion,
		DiffJSON:        raw,
		Classification:  res.Classification,
		ClassConfidence: res.ClassConfidence,
	}); err != nil {
		return false, fmt.Errorf("apply change: %w", err)
	}
	return true, nil
}

func isEgovStream(streamID string) bool {
	return strings.HasPrefix(streamID, leg.LawStreamID(""))
}

func lawIDOf(streamID string) string {
	return strings.TrimPrefix(streamID, leg.LawStreamID(""))
}
