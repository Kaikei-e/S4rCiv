package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"s4rciv.org/api/internal/port"
)

// ChangeReadModel implements both the ProjectorOffset (for the egov-differ
// projector) and the ChangeStore over interpretation.change. The differ projector
// owns only egov-law change rows, so its rebuild deletes exactly those.
type ChangeReadModel struct {
	pool *pgxpool.Pool
}

func NewChangeReadModel(pool *pgxpool.Pool) *ChangeReadModel { return &ChangeReadModel{pool: pool} }

// deleteEgovChanges removes change rows whose backing observation event is an
// egov-law stream (a re-diff replaces them, mirroring reproject semantics).
const deleteEgovChanges = `
	DELETE FROM interpretation.change c
	USING observation.event e
	WHERE c.observation_seq = e.seq AND e.stream_id LIKE 'egov-law:%'`

// ── ProjectorOffset ─────────────────────────────────────────────────────────

func (s *ChangeReadModel) Offset(ctx context.Context, projector string) (int64, error) {
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

func (s *ChangeReadModel) SetOffset(ctx context.Context, projector string, seq int64) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO interpretation.projector_offset (projector, last_seq, rebuilding)
		VALUES ($1, $2, false)
		ON CONFLICT (projector) DO UPDATE
		  SET last_seq = EXCLUDED.last_seq, rebuilding = false`, projector, seq)
	return err
}

// BeginRebuild deletes this projector's egov change rows, then resets the offset so Run
// re-diffs from genesis. Unlike the other read models (ADR-000022), ApplyChange is a
// bare INSERT and interpretation.change has no UNIQUE on observation_seq (PK is a
// synthetic bigserial), so a replay without this delete would accumulate duplicate
// change rows and double-count diffs in the timeline. Making this projector reader-atomic
// too is a follow-up (add UNIQUE(observation_seq) + upsert ApplyChange); its rebuild
// window only blanks diff counts / classification on rows that already have a title, not
// the title itself.
func (s *ChangeReadModel) BeginRebuild(ctx context.Context, projector string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx, deleteEgovChanges); err != nil {
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

// ── ChangeStore ─────────────────────────────────────────────────────────────

func (s *ChangeReadModel) TruncateEgov(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, deleteEgovChanges)
	return err
}

func (s *ChangeReadModel) ApplyChange(ctx context.Context, r port.ChangeRecord) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO interpretation.change
			(observation_seq, differ_version, diff, classification, class_confidence)
		VALUES ($1, $2, $3, $4, $5)`,
		r.ObservationSeq, r.DifferVersion, r.DiffJSON, r.Classification, r.ClassConfidence)
	return err
}
