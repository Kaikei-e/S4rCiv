package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"s4rciv.org/api/internal/port"
)

// ReadModel implements both the ProjectorOffset and the disposable ReadModelStore
// over the interpretation schema. All projection writes are merge-safe upserts;
// per-meeting children (speech, vote) are replaced so a reproject is idempotent.
type ReadModel struct {
	pool *pgxpool.Pool
}

func NewReadModel(pool *pgxpool.Pool) *ReadModel { return &ReadModel{pool: pool} }

const truncateReadModels = `
	TRUNCATE interpretation.vote, interpretation.vote_event,
	         interpretation.membership, interpretation.person,
	         interpretation.organization, interpretation.speech,
	         interpretation.meeting
	RESTART IDENTITY CASCADE`

// ── ProjectorOffset ─────────────────────────────────────────────────────────

func (s *ReadModel) Offset(ctx context.Context, projector string) (int64, error) {
	var seq int64
	err := s.pool.QueryRow(ctx,
		`SELECT last_seq FROM interpretation.projector_offset WHERE projector = $1`, projector,
	).Scan(&seq)
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = s.pool.Exec(ctx,
			`INSERT INTO interpretation.projector_offset (projector, last_seq)
			 VALUES ($1, 0) ON CONFLICT (projector) DO NOTHING`, projector)
		return 0, err
	}
	return seq, err
}

func (s *ReadModel) SetOffset(ctx context.Context, projector string, seq int64) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO interpretation.projector_offset (projector, last_seq, rebuilding)
		VALUES ($1, $2, false)
		ON CONFLICT (projector) DO UPDATE
		  SET last_seq = EXCLUDED.last_seq, rebuilding = false`, projector, seq)
	return err
}

// BeginRebuild marks the projector rebuilding and resets its offset to 0 so Run
// replays from genesis. It does NOT truncate the read model (ADR-000022): every apply
// is a merge-safe upsert / per-stream replace and the observation log is append-only,
// so replaying overwrites each stream's rows in place and readers never see an empty
// read model mid-rebuild. Use Truncate explicitly for a full wipe (e.g. when projector
// logic stops emitting some stream and the stale rows must be swept).
func (s *ReadModel) BeginRebuild(ctx context.Context, projector string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO interpretation.projector_offset (projector, last_seq, rebuilding)
		VALUES ($1, 0, true)
		ON CONFLICT (projector) DO UPDATE SET last_seq = 0, rebuilding = true`, projector)
	return err
}

// ── ReadModelStore ──────────────────────────────────────────────────────────

func (s *ReadModel) Truncate(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, truncateReadModels)
	return err
}

func (s *ReadModel) ApplyMeeting(ctx context.Context, b port.ProjectionBatch) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if err := upsertMeeting(ctx, tx, b); err != nil {
		return err
	}
	if err := replaceSpeeches(ctx, tx, b); err != nil {
		return err
	}
	if err := upsertPopolo(ctx, tx, b); err != nil {
		return err
	}
	if err := replaceVotes(ctx, tx, b); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func upsertMeeting(ctx context.Context, tx pgx.Tx, b port.ProjectionBatch) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO interpretation.meeting
			(issue_id, stream_id, session, house, meeting_name, issue,
			 meeting_date, permalink, was_ocr, observation_seq, observed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (issue_id) DO UPDATE SET
			stream_id = EXCLUDED.stream_id, session = EXCLUDED.session,
			house = EXCLUDED.house, meeting_name = EXCLUDED.meeting_name,
			issue = EXCLUDED.issue, meeting_date = EXCLUDED.meeting_date,
			permalink = EXCLUDED.permalink, was_ocr = EXCLUDED.was_ocr,
			observation_seq = EXCLUDED.observation_seq, observed_at = EXCLUDED.observed_at`,
		b.Meeting.IssueID, b.Meeting.StreamID, nullInt(b.Meeting.Session),
		nullStr(b.Meeting.House), nullStr(b.Meeting.MeetingName), nullStr(b.Meeting.Issue),
		parseDateP(b.Meeting.Date), nullStr(b.Meeting.Permalink), b.Meeting.WasOCR,
		b.ObservationSeq, b.ObservedAt)
	return err
}

func replaceSpeeches(ctx context.Context, tx pgx.Tx, b port.ProjectionBatch) error {
	if _, err := tx.Exec(ctx, `DELETE FROM interpretation.speech WHERE issue_id = $1`, b.Meeting.IssueID); err != nil {
		return err
	}
	for _, sp := range b.Speeches {
		if _, err := tx.Exec(ctx, `
			INSERT INTO interpretation.speech
				(speech_id, issue_id, speech_order, speaker, speaker_yomi,
				 speaker_group, speaker_position, speech, speech_url, person_id,
				 observation_seq, observed_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
			sp.SpeechID, sp.IssueID, sp.Order, nullStr(sp.Speaker), nullStr(sp.SpeakerYomi),
			nullStr(sp.SpeakerGroup), nullStr(sp.SpeakerPosition), nullStr(sp.Text),
			nullStr(sp.SpeechURL), nullStr(sp.PersonID), b.ObservationSeq, b.ObservedAt,
		); err != nil {
			return err
		}
	}
	return nil
}

func upsertPopolo(ctx context.Context, tx pgx.Tx, b port.ProjectionBatch) error {
	for _, p := range b.Persons {
		if _, err := tx.Exec(ctx, `
			INSERT INTO interpretation.person AS pp
				(person_id, name, yomi, identity_confidence,
				 first_observation_seq, last_observation_seq)
			VALUES ($1,$2,$3,$4,$5,$5)
			ON CONFLICT (person_id) DO UPDATE SET
				name = EXCLUDED.name, yomi = EXCLUDED.yomi,
				identity_confidence = EXCLUDED.identity_confidence,
				last_observation_seq = GREATEST(pp.last_observation_seq, EXCLUDED.last_observation_seq)`,
			p.PersonID, p.Name, p.Yomi, p.IdentityConfidence, b.ObservationSeq,
		); err != nil {
			return err
		}
	}
	for _, o := range b.Organizations {
		if _, err := tx.Exec(ctx, `
			INSERT INTO interpretation.organization AS o (org_id, name, classification)
			VALUES ($1,$2,$3)
			ON CONFLICT (org_id) DO UPDATE SET name = EXCLUDED.name, classification = EXCLUDED.classification`,
			o.OrgID, o.Name, o.Classification,
		); err != nil {
			return err
		}
	}
	for _, m := range b.Memberships {
		if _, err := tx.Exec(ctx, `
			INSERT INTO interpretation.membership AS mm
				(person_id, organization_id, role, first_observation_seq, last_observation_seq)
			VALUES ($1,$2,$3,$4,$4)
			ON CONFLICT (person_id, organization_id) DO UPDATE SET
				role = EXCLUDED.role,
				last_observation_seq = GREATEST(mm.last_observation_seq, EXCLUDED.last_observation_seq)`,
			m.PersonID, m.OrgID, nullStr(m.Role), b.ObservationSeq,
		); err != nil {
			return err
		}
	}
	return nil
}

func replaceVotes(ctx context.Context, tx pgx.Tx, b port.ProjectionBatch) error {
	if _, err := tx.Exec(ctx, `
		DELETE FROM interpretation.vote
		WHERE vote_event_id IN (SELECT vote_event_id FROM interpretation.vote_event WHERE issue_id = $1)`,
		b.Meeting.IssueID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM interpretation.vote_event WHERE issue_id = $1`, b.Meeting.IssueID); err != nil {
		return err
	}
	for _, ve := range b.VoteEvents {
		if _, err := tx.Exec(ctx, `
			INSERT INTO interpretation.vote_event
				(vote_event_id, issue_id, motion, yes_count, no_count, abstain_count,
				 result, confidence, needs_review, extractor_version, source_speech_id,
				 observation_seq, observed_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
			ve.VoteEventID, ve.IssueID, nullStr(ve.Motion), ve.YesCount, ve.NoCount,
			ve.AbstainCount, ve.Result, ve.Confidence, ve.NeedsReview,
			ve.ExtractorVersion, nullStr(ve.SourceSpeechID), b.ObservationSeq, b.ObservedAt,
		); err != nil {
			return err
		}
		for _, v := range ve.Votes {
			if _, err := tx.Exec(ctx, `
				INSERT INTO interpretation.vote (vote_event_id, option, voter_name, person_id, confidence)
				VALUES ($1,$2,$3,$4,$5)`,
				ve.VoteEventID, v.Option, v.VoterName, nullStr(v.PersonID), v.Confidence,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func nullInt(n int) any {
	if n == 0 {
		return nil
	}
	return n
}

func parseDateP(s string) any {
	if s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	return t
}
