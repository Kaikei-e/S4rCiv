package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	leg "s4rciv.org/api/internal/domain/legislative"
	"s4rciv.org/api/internal/port"
)

// LawQueryReader serves the egov-law read side from the law read models.
type LawQueryReader struct {
	pool *pgxpool.Pool
}

func NewLawQueryReader(pool *pgxpool.Pool) *LawQueryReader { return &LawQueryReader{pool: pool} }

const lawSelect = `
	SELECT law_id, stream_id, COALESCE(law_num,''), COALESCE(law_type,''),
	       COALESCE(law_title,''), COALESCE(law_title_kana,''), COALESCE(category,''),
	       to_char(promulgation_date, 'YYYY-MM-DD'), COALESCE(current_revision_id,''),
	       to_char(amendment_enforcement_date, 'YYYY-MM-DD'),
	       COALESCE(current_revision_status,''), COALESCE(repeal_status,''),
	       to_char(repeal_date, 'YYYY-MM-DD'), COALESCE(permalink,''),
	       was_ocr, observation_seq, observed_at
	FROM interpretation.legislative_work`

func scanLaw(row pgx.Row) (port.LawView, error) {
	var lv port.LawView
	var promulgation, enforcement, repeal *string
	var observedAt time.Time
	var seq int64
	var wasOCR bool
	err := row.Scan(&lv.Law.LawID, &lv.Law.StreamID, &lv.Law.LawNum, &lv.Law.LawType,
		&lv.Law.Title, &lv.Law.TitleKana, &lv.Law.Category, &promulgation,
		&lv.Law.CurrentRevisionID, &enforcement, &lv.Law.CurrentRevisionStatus,
		&lv.Law.RepealStatus, &repeal, &lv.Law.Permalink, &wasOCR, &seq, &observedAt)
	if err != nil {
		return lv, err
	}
	lv.Law.PromulgationDate = deref(promulgation)
	lv.Law.AmendmentEnforcementDate = deref(enforcement)
	lv.Law.RepealDate = deref(repeal)
	lv.Attr = port.Attribution{
		Source: "egov-law", Permalink: lv.Law.Permalink, FetchedAt: observedAt,
		ObservationSeq: seq, WasOCR: wasOCR, StreamID: lv.Law.StreamID,
	}
	return lv, nil
}

func (q *LawQueryReader) GetLaw(ctx context.Context, lawID string) (port.LawView, []port.LawNodeView, bool, error) {
	lv, err := scanLaw(q.pool.QueryRow(ctx, lawSelect+` WHERE law_id = $1`, lawID))
	if errors.Is(err, pgx.ErrNoRows) {
		return port.LawView{}, nil, false, nil
	}
	if err != nil {
		return port.LawView{}, nil, false, err
	}
	nodes, err := q.nodes(ctx, lawID)
	if err != nil {
		return lv, nil, false, err
	}
	return lv, nodes, true, nil
}

func (q *LawQueryReader) nodes(ctx context.Context, lawID string) ([]port.LawNodeView, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT eid, COALESCE(parent_eid,''), node_type, COALESCE(num,''),
		       COALESCE(caption,''), COALESCE(chapter_num,''), COALESCE(section_num,''),
		       is_suppl, COALESCE(sentence_text,''), ordinal
		FROM interpretation.law_node WHERE law_id = $1 ORDER BY ordinal`, lawID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []port.LawNodeView
	for rows.Next() {
		var v port.LawNodeView
		if err := rows.Scan(&v.Node.EID, &v.Node.ParentEID, &v.Node.NodeType, &v.Node.Num,
			&v.Node.Caption, &v.Node.ChapterNum, &v.Node.SectionNum, &v.Node.IsSuppl,
			&v.Node.SentenceText, &v.Node.Ordinal); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (q *LawQueryReader) ListLaws(ctx context.Context, lawType string, limit, offset int) ([]port.LawView, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := q.pool.Query(ctx, lawSelect+`
		WHERE ($1 = '' OR law_type = $1)
		ORDER BY promulgation_date DESC NULLS LAST, law_id
		LIMIT $2 OFFSET $3`, lawType, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []port.LawView
	for rows.Next() {
		lv, err := scanLaw(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, lv)
	}
	return out, rows.Err()
}

// diffPayload mirrors the JSON written by the diff usecase.
type diffPayload struct {
	NodeChanges []struct {
		EID      string `json:"eid"`
		Op       string `json:"op"`
		NodeType string `json:"node_type"`
		Num      string `json:"num"`
		PrevText string `json:"prev_text"`
		CurrText string `json:"curr_text"`
	} `json:"node_changes"`
}

func (q *LawQueryReader) GetLawChanges(ctx context.Context, lawID string, limit, offset int) ([]port.LawChangeView, error) {
	if limit <= 0 {
		limit = 50
	}
	streamID := leg.LawStreamID(lawID)
	rows, err := q.pool.Query(ctx, `
		SELECT c.observation_seq, c.differ_version, c.classification, c.class_confidence,
		       e.observed_at, c.diff
		FROM interpretation.change c
		JOIN observation.event e ON e.seq = c.observation_seq
		WHERE e.stream_id = $1
		ORDER BY c.observation_seq DESC
		LIMIT $2 OFFSET $3`, streamID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []port.LawChangeView
	for rows.Next() {
		var cv port.LawChangeView
		var raw []byte
		if err := rows.Scan(&cv.ObservationSeq, &cv.DifferVersion, &cv.Classification,
			&cv.ClassConfidence, &cv.ObservedAt, &raw); err != nil {
			return nil, err
		}
		var p diffPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			// A single undecodable diff payload must not fail the whole change list;
			// degrade the row to "no node changes", but say so server-side instead of
			// swallowing the data problem silently (CWE-392).
			slog.Warn("skipping undecodable diff payload",
				"law_id", lawID, "observation_seq", cv.ObservationSeq, "error", err)
		}
		for _, nc := range p.NodeChanges {
			cv.NodeChanges = append(cv.NodeChanges, port.NodeChange{
				EID: nc.EID, Op: nc.Op, NodeType: nc.NodeType, Num: nc.Num,
				PrevText: nc.PrevText, CurrText: nc.CurrText,
			})
		}
		out = append(out, cv)
	}
	return out, rows.Err()
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
