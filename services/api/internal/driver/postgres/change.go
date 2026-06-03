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
