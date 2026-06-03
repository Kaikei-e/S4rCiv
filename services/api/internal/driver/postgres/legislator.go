package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"s4rciv.org/api/internal/port"
)

// VotesByPerson compiles a legislator's named-vote record (ADR-000006). This is
// the ONLY per-person axis S4rCiv exposes: votes are factual records of an
// accountable public actor, in contrast to speeches (never anthologized;
// ADR-000004). The vote(person_id) index added in migration ...000009 backs this.
//
// Anti-misprofiling: the record is compiled ONLY when the identity is high
// confidence. For medium/low (a possible homonym) we return the person with an
// empty vote list so the UI can explain that votes are not merged.
func (q *QueryReader) VotesByPerson(ctx context.Context, personID string, limit, offset int) (port.LegislatorVotes, bool, error) {
	if limit <= 0 {
		limit = 50
	}
	var out port.LegislatorVotes
	out.PersonID = personID

	err := q.pool.QueryRow(ctx,
		`SELECT name, identity_confidence FROM interpretation.person WHERE person_id = $1`, personID,
	).Scan(&out.PersonName, &out.IdentityConfidence)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, false, nil
	}
	if err != nil {
		return out, false, err
	}
	if out.IdentityConfidence != "high" {
		return out, true, nil // found, but votes deliberately not compiled (ADR-000006)
	}

	rows, err := q.pool.Query(ctx, `
		SELECT v.vote_event_id, ve.issue_id, COALESCE(ve.motion,''), v.option, ve.result,
		       v.confidence, COALESCE(m.meeting_name,''), COALESCE(m.house,''),
		       to_char(m.meeting_date,'YYYY-MM-DD'), COALESCE(m.permalink,''),
		       ve.observation_seq, ve.observed_at, m.stream_id
		FROM interpretation.vote v
		JOIN interpretation.vote_event ve ON ve.vote_event_id = v.vote_event_id
		JOIN interpretation.meeting m     ON m.issue_id = ve.issue_id
		WHERE v.person_id = $1
		ORDER BY m.meeting_date DESC NULLS LAST, ve.vote_event_id
		LIMIT $2 OFFSET $3`, personID, limit, offset)
	if err != nil {
		return out, false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			lv         port.LegislatorVoteView
			date       *string
			permalink  string
			seq        int64
			observedAt time.Time
			streamID   string
		)
		if err := rows.Scan(&lv.VoteEventID, &lv.IssueID, &lv.Motion, &lv.Option, &lv.Result,
			&lv.Confidence, &lv.MeetingName, &lv.House, &date, &permalink,
			&seq, &observedAt, &streamID); err != nil {
			return out, false, err
		}
		if date != nil {
			lv.Date = *date
		}
		lv.Attr = port.Attribution{
			Source: sourceOf(streamID), Permalink: permalink, FetchedAt: observedAt, ObservationSeq: seq,
		}
		out.Votes = append(out.Votes, lv)
	}
	return out, true, rows.Err()
}
