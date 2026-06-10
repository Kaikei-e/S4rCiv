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
// owns only egov-law change rows; its rebuild replays merge-safe upserts over them
// in place (reader-atomic), and TruncateEgov wipes exactly those when an explicit
// full sweep is needed.
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

// BeginRebuild marks the projector rebuilding and resets its offset to 0 so Run re-diffs
// from genesis. It does NOT truncate (ADR-000024, aligning this projector with the kokkai
// one of ADR-000022): ApplyChange is a merge-safe upsert keyed by UNIQUE(observation_seq)
// and the change set is deterministic from the append-only log, so a replay overwrites each
// row in place and readers never see an empty change read model mid-rebuild. Use TruncateEgov
// for an explicit full wipe (e.g. if the projector stops emitting a seq and stale rows must
// be swept).
func (s *ChangeReadModel) BeginRebuild(ctx context.Context, projector string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO interpretation.projector_offset (projector, last_seq, rebuilding)
		VALUES ($1, 0, true)
		ON CONFLICT (projector) DO UPDATE SET last_seq = 0, rebuilding = true`, projector)
	return err
}

// ── ChangeStore ─────────────────────────────────────────────────────────────

func (s *ChangeReadModel) TruncateEgov(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, deleteEgovChanges)
	return err
}

// ApplyChange upserts the computed change for one observation seq. interpretation.change
// holds exactly one row per seq (UNIQUE(observation_seq), migration 20260603000017), and the
// diff is a deterministic full recompute from that single ResourceChanged event — so a replay
// overwrites the row in place (ON CONFLICT DO UPDATE = EXCLUDED.*, no business CASE, no
// COALESCE-able prior to preserve). This is what lets BeginRebuild skip the truncate and stay
// reader-atomic (ADR-000024), and turns a double-apply (daemon racing a reproject) into a
// loud unique-violation instead of a silently duplicated timeline row.
func (s *ChangeReadModel) ApplyChange(ctx context.Context, r port.ChangeRecord) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO interpretation.change
			(observation_seq, differ_version, diff, classification, class_confidence)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (observation_seq) DO UPDATE SET
			differ_version   = EXCLUDED.differ_version,
			diff             = EXCLUDED.diff,
			classification   = EXCLUDED.classification,
			class_confidence = EXCLUDED.class_confidence`,
		r.ObservationSeq, r.DifferVersion, r.DiffJSON, r.Classification, r.ClassConfidence)
	return err
}
