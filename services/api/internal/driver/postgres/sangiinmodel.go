package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	leg "s4rciv.org/api/internal/domain/legislative"
	"s4rciv.org/api/internal/port"
)

// SangiinVoteReadModel implements the ProjectorOffset and the disposable
// SangiinVoteReadModelStore over interpretation.sangiin_vote_event / sangiin_vote
// (ADR-000010). One apply = one vote page; that event's vote rows are replaced.
type SangiinVoteReadModel struct {
	pool *pgxpool.Pool
}

func NewSangiinVoteReadModel(pool *pgxpool.Pool) *SangiinVoteReadModel {
	return &SangiinVoteReadModel{pool: pool}
}

const truncateSangiinVote = `TRUNCATE interpretation.sangiin_vote, interpretation.sangiin_vote_event RESTART IDENTITY CASCADE`

// ── ProjectorOffset ─────────────────────────────────────────────────────────

func (s *SangiinVoteReadModel) Offset(ctx context.Context, projector string) (int64, error) {
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

func (s *SangiinVoteReadModel) SetOffset(ctx context.Context, projector string, seq int64) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO interpretation.projector_offset (projector, last_seq, rebuilding)
		VALUES ($1, $2, false)
		ON CONFLICT (projector) DO UPDATE
		  SET last_seq = EXCLUDED.last_seq, rebuilding = false`, projector, seq)
	return err
}

// BeginRebuild marks the projector rebuilding and resets its offset to 0 so Run
// replays from genesis. It does NOT truncate the read model (ADR-000022):
// ApplySangiinVote upserts each vote event on vote_event_id + replaces its votes, and
// the observation log is append-only, so a replay overwrites each event in place —
// readers never see empty sangiin_vote(_event) mid-rebuild. Use TruncateSangiinVote
// explicitly for a full wipe.
func (s *SangiinVoteReadModel) BeginRebuild(ctx context.Context, projector string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO interpretation.projector_offset (projector, last_seq, rebuilding)
		VALUES ($1, 0, true)
		ON CONFLICT (projector) DO UPDATE SET last_seq = 0, rebuilding = true`, projector)
	return err
}

// ── SangiinVoteReadModelStore ───────────────────────────────────────────────

func (s *SangiinVoteReadModel) TruncateSangiinVote(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, truncateSangiinVote)
	return err
}

// ApplySangiinVote upserts one vote event and replaces its per-member votes. name_key
// is the normalized voter name that joins legislator_district(name_key) for 都道府県.
func (s *SangiinVoteReadModel) ApplySangiinVote(ctx context.Context, b port.SangiinVoteProjectionBatch) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx,
		`DELETE FROM interpretation.sangiin_vote WHERE vote_event_id = $1`, b.VoteEventID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO interpretation.sangiin_vote_event
			(vote_event_id, session, motion, vote_date, yes_count, no_count, permalink,
			 observation_seq, observed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (vote_event_id) DO UPDATE SET
			session = EXCLUDED.session, motion = EXCLUDED.motion, vote_date = EXCLUDED.vote_date,
			yes_count = EXCLUDED.yes_count, no_count = EXCLUDED.no_count, permalink = EXCLUDED.permalink,
			observation_seq = EXCLUDED.observation_seq, observed_at = EXCLUDED.observed_at`,
		b.VoteEventID, b.Page.Session, nullStr(b.Page.Motion), parseDateP(b.Page.Date),
		b.Page.YesCount, b.Page.NoCount, nullStr(b.Permalink), b.ObservationSeq, b.ObservedAt,
	); err != nil {
		return err
	}
	for _, v := range b.Page.Votes {
		if _, err := tx.Exec(ctx, `
			INSERT INTO interpretation.sangiin_vote
				(vote_event_id, option, voter_name, name_key, parliamentary_group)
			VALUES ($1,$2,$3,$4,$5)`,
			b.VoteEventID, v.Option, v.Name, leg.NameKey(v.Name), nullStr(v.Group),
		); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
