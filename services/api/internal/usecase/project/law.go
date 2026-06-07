package project

import (
	"context"
	"fmt"
	"strings"

	leg "s4rciv.org/api/internal/domain/legislative"
	"s4rciv.org/api/internal/port"
)

// LawProjector folds observation events for egov-law streams into the disposable
// law read models (legislative_work + law_node, current-tree-only). Pure with
// respect to snapshot bytes + event metadata, so a reproject from seq 0 reproduces
// the same read models. Envelope-only metadata (law_type, category, revision
// status) is not in the XML snapshot and is left to the source listing; the
// snapshot-derivable fields are projected here with provenance.
type LawProjector struct {
	reader    port.EventReader
	norm      port.LawNormalizer
	store     port.LawReadModelStore
	offsets   port.ProjectorOffset
	name      string
	batchSize int
}

func NewLaw(reader port.EventReader, norm port.LawNormalizer, store port.LawReadModelStore, offsets port.ProjectorOffset, name string) *LawProjector {
	return &LawProjector{reader: reader, norm: norm, store: store, offsets: offsets, name: name, batchSize: DefaultBatchSize}
}

// Run folds every observation event past the stored offset, projecting only
// egov-law streams. Returns how many law snapshots were projected.
func (p *LawProjector) Run(ctx context.Context) (int, error) {
	off, err := p.offsets.Offset(ctx, p.name)
	if err != nil {
		return 0, fmt.Errorf("read offset: %w", err)
	}
	processed := 0
	for {
		evs, err := p.reader.EventsSince(ctx, off, p.batchSize)
		if err != nil {
			return processed, fmt.Errorf("read events: %w", err)
		}
		if len(evs) == 0 {
			return processed, nil
		}
		for _, ev := range evs {
			if ev.SnapshotBytes != nil && isEgovStream(ev.StreamID) {
				if err := p.project(ctx, ev); err != nil {
					return processed, fmt.Errorf("project seq %d: %w", ev.Seq, err)
				}
				processed++
			}
			off = ev.Seq
		}
		if err := p.offsets.SetOffset(ctx, p.name, off); err != nil {
			return processed, fmt.Errorf("set offset: %w", err)
		}
		if len(evs) < p.batchSize {
			return processed, nil
		}
	}
}

// Reproject resets the offset and replays from 0 over the live law read models without
// truncating them (ADR-000022): ApplyLaw is a per-law upsert + law_node replace, so a
// replay overwrites each law in place and readers never see an empty legislative_work
// (which had surfaced raw egov-law:<id> stream ids as timeline titles mid-rebuild).
func (p *LawProjector) Reproject(ctx context.Context) (int, error) {
	if err := p.offsets.BeginRebuild(ctx, p.name); err != nil {
		return 0, fmt.Errorf("begin rebuild: %w", err)
	}
	return p.Run(ctx)
}

func (p *LawProjector) project(ctx context.Context, ev port.ObservedEvent) error {
	content, err := p.norm.ParseLaw(ev.SnapshotBytes)
	if err != nil {
		return err
	}
	lawID := lawIDOf(ev.StreamID)
	content.Law.LawID = lawID
	content.Law.StreamID = ev.StreamID
	content.Law.Permalink = "https://laws.e-gov.go.jp/law/" + lawID

	return p.store.ApplyLaw(ctx, port.LawProjectionBatch{
		Law:            content.Law,
		Nodes:          content.Nodes,
		ObservationSeq: ev.Seq,
		ObservedAt:     ev.ObservedAt,
	})
}

func isEgovStream(streamID string) bool {
	return strings.HasPrefix(streamID, leg.LawStreamID(""))
}

// lawIDOf strips the "egov-law:" prefix from a stream id.
func lawIDOf(streamID string) string {
	return strings.TrimPrefix(streamID, leg.LawStreamID(""))
}
