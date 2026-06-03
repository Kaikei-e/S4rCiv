package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"s4rciv.org/api/internal/port"
)

// RosterReadModel implements both the ProjectorOffset and the disposable
// RosterReadModelStore over interpretation.legislator_district (ADR-000008/000010). A
// page's rows are replaced by stream_id on each apply, so a member dropped from the
// page on re-observation is removed — keeping the map "現職のみ". The table is SHARED by
// the 衆 (giin-roster) and 参 (sangiin-roster) sources, so truncation/rebuild is scoped
// by streamPrefix: a reproject of one source never wipes the other's rows.
type RosterReadModel struct {
	pool         *pgxpool.Pool
	streamPrefix string
}

func NewRosterReadModel(pool *pgxpool.Pool, streamPrefix string) *RosterReadModel {
	return &RosterReadModel{pool: pool, streamPrefix: streamPrefix}
}

func (s *RosterReadModel) deleteOwnRows(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx,
		`DELETE FROM interpretation.legislator_district WHERE stream_id LIKE $1`, s.streamPrefix+"%")
	return err
}

// ── ProjectorOffset ─────────────────────────────────────────────────────────

func (s *RosterReadModel) Offset(ctx context.Context, projector string) (int64, error) {
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

func (s *RosterReadModel) SetOffset(ctx context.Context, projector string, seq int64) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO interpretation.projector_offset (projector, last_seq, rebuilding)
		VALUES ($1, $2, false)
		ON CONFLICT (projector) DO UPDATE
		  SET last_seq = EXCLUDED.last_seq, rebuilding = false`, projector, seq)
	return err
}

func (s *RosterReadModel) BeginRebuild(ctx context.Context, projector string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if err := s.deleteOwnRows(ctx, tx); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO interpretation.projector_offset (projector, last_seq, rebuilding)
		VALUES ($1, 0, true)
		ON CONFLICT (projector) DO UPDATE SET last_seq = 0, rebuilding = true`, projector); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ── RosterReadModelStore ────────────────────────────────────────────────────

func (s *RosterReadModel) TruncateRoster(ctx context.Context) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM interpretation.legislator_district WHERE stream_id LIKE $1`, s.streamPrefix+"%")
	return err
}

// ApplyRoster replaces one page's legislator_district rows (by stream_id) and
// inserts the current entries. nullStr maps "" -> NULL so the geometry-binding
// CHECK holds (a 比例 entry has NULL district_code). ON CONFLICT (person_id) covers
// the rare cross-house homonym (same name+yomi) with last-writer-wins.
func (s *RosterReadModel) ApplyRoster(ctx context.Context, b port.RosterProjectionBatch) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx,
		`DELETE FROM interpretation.legislator_district WHERE stream_id = $1`, b.StreamID); err != nil {
		return err
	}
	for _, e := range b.Entries {
		if _, err := tx.Exec(ctx, `
			INSERT INTO interpretation.legislator_district
				(person_id, stream_id, name, name_key, house, district_code, district_name,
				 is_pr, pr_block, parliamentary_group, observation_seq, observed_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
			ON CONFLICT (person_id) DO UPDATE SET
				stream_id = EXCLUDED.stream_id, name = EXCLUDED.name, name_key = EXCLUDED.name_key,
				house = EXCLUDED.house, district_code = EXCLUDED.district_code,
				district_name = EXCLUDED.district_name, is_pr = EXCLUDED.is_pr,
				pr_block = EXCLUDED.pr_block, parliamentary_group = EXCLUDED.parliamentary_group,
				observation_seq = EXCLUDED.observation_seq, observed_at = EXCLUDED.observed_at`,
			e.PersonID, b.StreamID, e.Name, e.NameKey, e.House, nullStr(e.DistrictCode), nullStr(e.DistrictName),
			e.IsPR, nullStr(e.PRBlock), nullStr(e.ParliamentaryGroup), b.ObservationSeq, b.ObservedAt,
		); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
