package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"s4rciv.org/api/internal/port"
)

// ListTimeline composes the cross-source timeline at read time (ADR-000006):
// the observation.event log is the spine (1 row = 1 item, status = event type,
// order = global seq), enriched from the interpretation read models by joining
// the per-stream current metadata (meeting / legislative_work by stream_id) and
// the per-event change (interpretation.change by observation_seq).
//
// Diff content is never pulled here (§7): only the structural change counts are
// computed in SQL via jsonb, so a large diff payload never crosses the wire for
// a list row. There is no person axis (anti-doxxing, ADR-000004/000006).
func (q *QueryReader) ListTimeline(ctx context.Context, f port.TimelineFilter) ([]port.TimelineItemView, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	// Keyset over the immutable seq spine. Forward (older) walks seq < cursor in
	// DESC; backward (newer) walks seq > cursor ASC then reverses to DESC so the
	// caller always sees newest-first. Text markers (not fmt verbs) keep the
	// literal '%' in the ILIKE keyword filter intact.
	cursorPred, order := "($1 = 0 OR e.seq < $1)", "DESC"
	if f.Backward {
		cursorPred, order = "e.seq > $1", "ASC"
	}
	sql := strings.NewReplacer("__CURSOR__", cursorPred, "__ORDER__", order).Replace(`
		SELECT
		  e.seq, e.type::text, e.source, e.stream_id, e.observed_at, e.source_published_at,
		  encode(e.log_hash, 'hex'), encode(e.log_prev_hash, 'hex'),
		  m.issue_id, COALESCE(m.meeting_name,''), COALESCE(m.session,0), COALESCE(m.house,''),
		  COALESCE(m.issue,''), to_char(m.meeting_date,'YYYY-MM-DD'),
		  COALESCE(m.permalink,''), COALESCE(m.was_ocr,false),
		  lw.law_id, COALESCE(lw.law_title,''), COALESCE(lw.law_num,''),
		  COALESCE(lw.permalink,''), COALESCE(lw.was_ocr,false),
		  COALESCE(c.classification,''), COALESCE(c.class_confidence,''),
		  COALESCE((SELECT count(*) FROM jsonb_array_elements(c.diff->'node_changes') nc WHERE nc->>'op'='added'),0),
		  COALESCE((SELECT count(*) FROM jsonb_array_elements(c.diff->'node_changes') nc WHERE nc->>'op'='deleted'),0),
		  COALESCE((SELECT count(*) FROM jsonb_array_elements(c.diff->'node_changes') nc WHERE nc->>'op'='modified'),0),
		  COALESCE((SELECT ve.vote_event_id FROM interpretation.vote_event ve
		            WHERE ve.observation_seq = e.seq ORDER BY ve.vote_event_id LIMIT 1),'')
		FROM observation.event e
		LEFT JOIN interpretation.meeting m           ON m.stream_id = e.stream_id
		LEFT JOIN interpretation.legislative_work lw ON lw.stream_id = e.stream_id
		LEFT JOIN interpretation.change c            ON c.observation_seq = e.seq
		WHERE __CURSOR__
		  -- The timeline only shows the sources it can ENRICH with a headline read model
		  -- (kokkai 会議録 / egov-law 法令). Vote-map support sources (giin-roster,
		  -- sangiin-roster, sangiin-vote) are reference data surfaced via the 衆院/参院 maps,
		  -- not the citizen change-timeline / Atom feed (ADR-000006/000007/000010). An
		  -- allowlist (not a per-source denylist) keeps future sources from leaking in.
		  AND e.source IN ('kokkai', 'egov-law')
		  AND ($2 = '' OR e.source = $2)
		  AND ($3 = '' OR e.type::text = $3)
		  AND ($4 = '' OR c.classification = $4)
		  AND (NULLIF($5,'')::timestamptz IS NULL OR e.observed_at >= NULLIF($5,'')::timestamptz)
		  AND (NULLIF($6,'')::timestamptz IS NULL OR e.observed_at <  NULLIF($6,'')::timestamptz)
		  AND ($7 = '' OR m.meeting_name ILIKE '%'||$7||'%' OR lw.law_title ILIKE '%'||$7||'%'
		               OR lw.law_num ILIKE '%'||$7||'%' OR lw.category ILIKE '%'||$7||'%')
		ORDER BY e.seq __ORDER__
		LIMIT $8`)
	rows, err := q.pool.Query(ctx, sql,
		f.CursorSeq, f.Source, f.EventType, f.Classification, f.Since, f.Until, f.Keyword, f.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []port.TimelineItemView
	for rows.Next() {
		var (
			v                    port.TimelineItemView
			sourcePublishedAt    *time.Time
			logHash, logPrevHash string
			issueID              *string
			meetingName, house   string
			session              int
			issue                string
			meetingDate          *string
			mPermalink           string
			mWasOCR              bool
			lawID                *string
			lawTitle, lawNum     string
			lwPermalink          string
			lwWasOCR             bool
		)
		if err := rows.Scan(
			&v.Seq, &v.EventType, &v.Source, &v.StreamID, &v.ObservedAt, &sourcePublishedAt,
			&logHash, &logPrevHash,
			&issueID, &meetingName, &session, &house, &issue, &meetingDate, &mPermalink, &mWasOCR,
			&lawID, &lawTitle, &lawNum, &lwPermalink, &lwWasOCR,
			&v.Classification, &v.ClassConfidence,
			&v.NodesAdded, &v.NodesDeleted, &v.NodesModified, &v.FeaturedVoteEventID,
		); err != nil {
			return nil, err
		}
		v.SourcePublishedAt = sourcePublishedAt

		var permalink string
		var wasOCR bool
		switch {
		case issueID != nil: // kokkai meeting stream
			v.IssueID = *issueID
			v.Title = meetingName
			v.Subtitle = meetingSubtitle(session, house, issue, deref(meetingDate))
			permalink, wasOCR = mPermalink, mWasOCR
		case lawID != nil: // egov-law stream
			v.LawID = *lawID
			v.Title = lawTitle
			v.Subtitle = lawNum
			permalink, wasOCR = lwPermalink, lwWasOCR
		default: // event with no enriching read model (e.g. vanished + pruned); fall back to stream id
			v.Title = v.StreamID
		}
		v.WasOCR = wasOCR
		v.Attr = port.Attribution{
			Source: v.Source, Permalink: permalink, FetchedAt: v.ObservedAt,
			ObservationSeq: v.Seq, WasOCR: wasOCR, LogHash: logHash, PrevLogHash: logPrevHash,
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Backward pages are fetched ASC (the rows nearest above the cursor); flip to
	// DESC so every page is newest-first regardless of navigation direction.
	if f.Backward {
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}
	return out, nil
}

// CountTimeline computes the filter's total row count and how many of those rows
// are newer than aboveSeq (seq > aboveSeq), mirroring ListTimeline's FROM/WHERE
// exactly so the counts match the paged set. Keyset has no random page access, so
// this feeds only the "n / N ページ・全 X 件" position display (ADR-000006). It is a
// full filtered scan (unavoidable for an exact total); swap to an estimate
// (pg_class.reltuples / EXPLAIN) if the log grows large enough to need it.
func (q *QueryReader) CountTimeline(ctx context.Context, f port.TimelineFilter, aboveSeq int64) (int, int, error) {
	var total, above int
	err := q.pool.QueryRow(ctx, `
		SELECT
		  count(*),
		  count(*) FILTER (WHERE e.seq > $7)
		FROM observation.event e
		LEFT JOIN interpretation.meeting m           ON m.stream_id = e.stream_id
		LEFT JOIN interpretation.legislative_work lw ON lw.stream_id = e.stream_id
		LEFT JOIN interpretation.change c            ON c.observation_seq = e.seq
		WHERE e.source IN ('kokkai', 'egov-law')
		  AND ($1 = '' OR e.source = $1)
		  AND ($2 = '' OR e.type::text = $2)
		  AND ($3 = '' OR c.classification = $3)
		  AND (NULLIF($4,'')::timestamptz IS NULL OR e.observed_at >= NULLIF($4,'')::timestamptz)
		  AND (NULLIF($5,'')::timestamptz IS NULL OR e.observed_at <  NULLIF($5,'')::timestamptz)
		  AND ($6 = '' OR m.meeting_name ILIKE '%'||$6||'%' OR lw.law_title ILIKE '%'||$6||'%'
		               OR lw.law_num ILIKE '%'||$6||'%' OR lw.category ILIKE '%'||$6||'%')`,
		f.Source, f.EventType, f.Classification, f.Since, f.Until, f.Keyword, aboveSeq,
	).Scan(&total, &above)
	if err != nil {
		return 0, 0, err
	}
	return total, above, nil
}

func meetingSubtitle(session int, house, issue, date string) string {
	var parts []string
	if session > 0 {
		parts = append(parts, fmt.Sprintf("第%d回", session))
	}
	if house != "" {
		parts = append(parts, house)
	}
	if issue != "" {
		parts = append(parts, issue)
	}
	s := strings.Join(parts, " ")
	if date != "" {
		if s != "" {
			s += " · "
		}
		s += date
	}
	return s
}
