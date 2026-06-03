package project

import (
	"context"
	"fmt"
	"strings"

	"s4rciv.org/api/internal/port"
)

// streamPrefix for 参議院 vote-result streams (gateway.StreamID prefix).
const sangiinVotePrefix = "sangiin-vote:"

// SangiinVoteProjector folds observation events for 参議院 vote-result streams into the
// disposable sangiin_vote read model (ADR-000010). Pure w.r.t. snapshot bytes + event
// metadata, so a reproject from seq 0 reproduces the same read model.
type SangiinVoteProjector struct {
	reader    port.EventReader
	norm      port.SangiinVoteNormalizer
	store     port.SangiinVoteReadModelStore
	offsets   port.ProjectorOffset
	name      string
	batchSize int
}

func NewSangiinVote(reader port.EventReader, norm port.SangiinVoteNormalizer, store port.SangiinVoteReadModelStore, offsets port.ProjectorOffset, name string) *SangiinVoteProjector {
	return &SangiinVoteProjector{reader: reader, norm: norm, store: store, offsets: offsets, name: name, batchSize: DefaultBatchSize}
}

// Run folds every observation event past the stored offset, projecting only
// sangiin-vote streams. Returns how many vote pages were projected.
func (p *SangiinVoteProjector) Run(ctx context.Context) (int, error) {
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
			if ev.SnapshotBytes != nil && strings.HasPrefix(ev.StreamID, sangiinVotePrefix) {
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

// Reproject truncates the read model, resets the offset, and replays from 0.
func (p *SangiinVoteProjector) Reproject(ctx context.Context) (int, error) {
	if err := p.offsets.BeginRebuild(ctx, p.name); err != nil {
		return 0, fmt.Errorf("begin rebuild: %w", err)
	}
	return p.Run(ctx)
}

func (p *SangiinVoteProjector) project(ctx context.Context, ev port.ObservedEvent) error {
	page, err := p.norm.ParseVotePage(ev.SnapshotBytes)
	if err != nil {
		return err
	}
	slug := strings.TrimPrefix(ev.StreamID, sangiinVotePrefix)
	return p.store.ApplySangiinVote(ctx, port.SangiinVoteProjectionBatch{
		VoteEventID:    slug,
		Page:           page,
		Permalink:      fmt.Sprintf("https://www.sangiin.go.jp/japanese/touhyoulist/%d/%s.htm", page.Session, slug),
		ObservationSeq: ev.Seq,
		ObservedAt:     ev.ObservedAt,
	})
}
