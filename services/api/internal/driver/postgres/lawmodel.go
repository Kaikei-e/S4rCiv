package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"s4rciv.org/api/internal/port"
)

// LawReadModel implements both the ProjectorOffset (for the egov-law projector)
// and the disposable LawReadModelStore over the interpretation schema. The law
// tree is current-tree-only: a law's law_node rows are replaced on each apply so
// a reproject is idempotent.
type LawReadModel struct {
	pool *pgxpool.Pool
}

func NewLawReadModel(pool *pgxpool.Pool) *LawReadModel { return &LawReadModel{pool: pool} }

const truncateLawReadModels = `
	TRUNCATE interpretation.law_node, interpretation.legislative_work
	RESTART IDENTITY CASCADE`

// ── ProjectorOffset ─────────────────────────────────────────────────────────

func (s *LawReadModel) Offset(ctx context.Context, projector string) (int64, error) {
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

func (s *LawReadModel) SetOffset(ctx context.Context, projector string, seq int64) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO interpretation.projector_offset (projector, last_seq, rebuilding)
		VALUES ($1, $2, false)
		ON CONFLICT (projector) DO UPDATE
		  SET last_seq = EXCLUDED.last_seq, rebuilding = false`, projector, seq)
	return err
}

// BeginRebuild marks the projector rebuilding and resets its offset to 0 so Run
// replays from genesis. It does NOT truncate the read model (ADR-000022): ApplyLaw is
// a per-law upsert + law_node replace and the observation log is append-only, so a
// replay overwrites each law's rows in place — readers never see an empty
// legislative_work mid-rebuild (which is what surfaced raw egov-law:<id> stream ids as
// timeline titles). Use TruncateLaws explicitly for a full wipe.
func (s *LawReadModel) BeginRebuild(ctx context.Context, projector string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO interpretation.projector_offset (projector, last_seq, rebuilding)
		VALUES ($1, 0, true)
		ON CONFLICT (projector) DO UPDATE SET last_seq = 0, rebuilding = true`, projector)
	return err
}

// ── LawReadModelStore ───────────────────────────────────────────────────────

func (s *LawReadModel) TruncateLaws(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, truncateLawReadModels)
	return err
}

func (s *LawReadModel) ApplyLaw(ctx context.Context, b port.LawProjectionBatch) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if err := upsertLegislativeWork(ctx, tx, b); err != nil {
		return err
	}
	if err := replaceLawNodes(ctx, tx, b); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func upsertLegislativeWork(ctx context.Context, tx pgx.Tx, b port.LawProjectionBatch) error {
	l := b.Law
	_, err := tx.Exec(ctx, `
		INSERT INTO interpretation.legislative_work
			(law_id, stream_id, law_num, law_type, law_title, law_title_kana, category,
			 promulgation_date, current_revision_id, amendment_promulgate_date,
			 amendment_enforcement_date, current_revision_status, repeal_status, repeal_date,
			 permalink, was_ocr, observation_seq, observed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,false,$16,$17)
		ON CONFLICT (law_id) DO UPDATE SET
			stream_id = EXCLUDED.stream_id, law_num = EXCLUDED.law_num,
			law_type = EXCLUDED.law_type, law_title = EXCLUDED.law_title,
			law_title_kana = EXCLUDED.law_title_kana, category = EXCLUDED.category,
			promulgation_date = EXCLUDED.promulgation_date,
			current_revision_id = EXCLUDED.current_revision_id,
			amendment_promulgate_date = EXCLUDED.amendment_promulgate_date,
			amendment_enforcement_date = EXCLUDED.amendment_enforcement_date,
			current_revision_status = EXCLUDED.current_revision_status,
			repeal_status = EXCLUDED.repeal_status, repeal_date = EXCLUDED.repeal_date,
			permalink = EXCLUDED.permalink, observation_seq = EXCLUDED.observation_seq,
			observed_at = EXCLUDED.observed_at`,
		l.LawID, l.StreamID, nullStr(l.LawNum), nullStr(l.LawType), nullStr(l.Title),
		nullStr(l.TitleKana), nullStr(l.Category), parseDateP(l.PromulgationDate),
		nullStr(l.CurrentRevisionID), nil, parseDateP(l.AmendmentEnforcementDate),
		nullStr(l.CurrentRevisionStatus), nullStr(l.RepealStatus), parseDateP(l.RepealDate),
		nullStr(l.Permalink), b.ObservationSeq, b.ObservedAt)
	return err
}

func replaceLawNodes(ctx context.Context, tx pgx.Tx, b port.LawProjectionBatch) error {
	if _, err := tx.Exec(ctx, `DELETE FROM interpretation.law_node WHERE law_id = $1`, b.Law.LawID); err != nil {
		return err
	}
	for _, n := range b.Nodes {
		if _, err := tx.Exec(ctx, `
			INSERT INTO interpretation.law_node
				(law_id, eid, parent_eid, node_type, num, caption, chapter_num,
				 section_num, is_suppl, sentence_text, ordinal, observation_seq, observed_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
			b.Law.LawID, n.EID, nullStr(n.ParentEID), n.NodeType, nullStr(n.Num),
			nullStr(n.Caption), nullStr(n.ChapterNum), nullStr(n.SectionNum), n.IsSuppl,
			nullStr(n.SentenceText), n.Ordinal, b.ObservationSeq, b.ObservedAt,
		); err != nil {
			return err
		}
	}
	return nil
}
