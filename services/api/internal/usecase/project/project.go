// Package project is the read-model projector: it folds observation events into
// the disposable interpretation-plane read models (meeting/speech, Popolo, votes).
// Pure with respect to its inputs (snapshot bytes + event metadata) and driven
// only through ports, so a reproject from seq 0 reproduces the same read models.
package project

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	leg "s4rciv.org/api/internal/domain/legislative"
	"s4rciv.org/api/internal/port"
)

const DefaultBatchSize = 100

type Projector struct {
	reader    port.EventReader
	norm      port.Normalizer
	store     port.ReadModelStore
	offsets   port.ProjectorOffset
	name      string
	batchSize int
}

func New(reader port.EventReader, norm port.Normalizer, store port.ReadModelStore, offsets port.ProjectorOffset, name string) *Projector {
	return &Projector{reader: reader, norm: norm, store: store, offsets: offsets, name: name, batchSize: DefaultBatchSize}
}

// Run folds every observation event past the stored offset, projecting only
// kokkai streams. Returns how many meeting snapshots were projected.
func (p *Projector) Run(ctx context.Context) (int, error) {
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
			// ResourceVanished carries no snapshot; other sources' streams (e.g.
			// egov-law AKN XML) share this log but are not meetings. Either way
			// nothing to project, but the offset still advances past it.
			if ev.SnapshotBytes != nil && isKokkaiStream(ev.StreamID) {
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

// Reproject truncates the read models, resets the offset, and replays from 0.
func (p *Projector) Reproject(ctx context.Context) (int, error) {
	if err := p.offsets.BeginRebuild(ctx, p.name); err != nil {
		return 0, fmt.Errorf("begin rebuild: %w", err)
	}
	return p.Run(ctx)
}

// isKokkaiStream reports whether streamID belongs to the kokkai (国会会議録)
// adapter. The meeting projector folds the shared observation log, so it must
// skip other sources' snapshots (e.g. egov-law AKN XML), mirroring LawProjector.
func isKokkaiStream(streamID string) bool {
	return strings.HasPrefix(streamID, leg.MeetingStreamID(""))
}

func (p *Projector) project(ctx context.Context, ev port.ObservedEvent) error {
	content, err := p.norm.ParseMeeting(ev.SnapshotBytes)
	if err != nil {
		return err
	}
	content.Meeting.WasOCR = ev.WasOCR

	ent := leg.BuildEntities(content.Speeches)
	for i := range content.Speeches {
		content.Speeches[i].PersonID = ent.ResolveVoter(content.Speeches[i].Speaker)
	}

	return p.store.ApplyMeeting(ctx, port.ProjectionBatch{
		Meeting:        content.Meeting,
		Speeches:       content.Speeches,
		Persons:        ent.Persons,
		Organizations:  ent.Organizations,
		Memberships:    ent.Memberships,
		VoteEvents:     buildVoteEvents(content, ent),
		ObservationSeq: ev.Seq,
		ObservedAt:     ev.ObservedAt,
	})
}

func buildVoteEvents(content leg.MeetingContent, ent leg.Entities) []port.StoredVoteEvent {
	parsed := leg.ParseVotes(content)
	out := make([]port.StoredVoteEvent, 0, len(parsed))
	for i, pe := range parsed {
		votes := make([]port.StoredVote, 0, len(pe.Votes))
		for _, v := range pe.Votes {
			votes = append(votes, port.StoredVote{
				Option:     v.Option,
				VoterName:  v.VoterName,
				PersonID:   ent.ResolveVoter(v.VoterName),
				Confidence: pe.Confidence,
			})
		}
		out = append(out, port.StoredVoteEvent{
			VoteEventID:      content.Meeting.IssueID + "#" + strconv.Itoa(i+1),
			IssueID:          content.Meeting.IssueID,
			Motion:           pe.Motion,
			YesCount:         pe.YesCount,
			NoCount:          pe.NoCount,
			AbstainCount:     pe.AbstainCount,
			Result:           pe.Result,
			Confidence:       pe.Confidence,
			NeedsReview:      pe.NeedsReview,
			ExtractorVersion: leg.ExtractorVersion,
			SourceSpeechID:   pe.SourceSpeechID,
			Votes:            votes,
		})
	}
	return out
}
