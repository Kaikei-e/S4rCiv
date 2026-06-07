package project

import (
	"context"
	"fmt"
	"strings"

	"s4rciv.org/api/internal/port"
)

// RosterProjector folds observation events for giin-roster streams into the
// disposable legislator_district read model (the legislator->electoral-district
// binding, ADR-000008). Pure with respect to snapshot bytes + event metadata, so a
// reproject from seq 0 reproduces the same read model. It folds the shared
// observation log, so it skips other sources' streams, mirroring LawProjector.
type RosterProjector struct {
	reader       port.EventReader
	norm         port.RosterNormalizer
	store        port.RosterReadModelStore
	offsets      port.ProjectorOffset
	name         string
	streamPrefix string // only fold streams under this prefix (giin-roster: or sangiin-roster:)
	batchSize    int
}

func NewRoster(reader port.EventReader, norm port.RosterNormalizer, store port.RosterReadModelStore, offsets port.ProjectorOffset, name, streamPrefix string) *RosterProjector {
	return &RosterProjector{reader: reader, norm: norm, store: store, offsets: offsets, name: name, streamPrefix: streamPrefix, batchSize: DefaultBatchSize}
}

// Run folds every observation event past the stored offset, projecting only
// giin-roster streams. Returns how many roster pages were projected.
func (p *RosterProjector) Run(ctx context.Context) (int, error) {
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
			if ev.SnapshotBytes != nil && strings.HasPrefix(ev.StreamID, p.streamPrefix) {
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

// Reproject resets the offset and replays from 0 over the live read model without
// truncating it (ADR-000022): ApplyRoster replaces each page by stream_id + upserts on
// person_id, so a replay overwrites in place and readers never see an empty read model.
func (p *RosterProjector) Reproject(ctx context.Context) (int, error) {
	if err := p.offsets.BeginRebuild(ctx, p.name); err != nil {
		return 0, fmt.Errorf("begin rebuild: %w", err)
	}
	return p.Run(ctx)
}

func (p *RosterProjector) project(ctx context.Context, ev port.ObservedEvent) error {
	entries, err := p.norm.ParseRoster(ev.SnapshotBytes)
	if err != nil {
		return err
	}
	return p.store.ApplyRoster(ctx, port.RosterProjectionBatch{
		StreamID:       ev.StreamID,
		Entries:        entries,
		ObservationSeq: ev.Seq,
		ObservedAt:     ev.ObservedAt,
	})
}
