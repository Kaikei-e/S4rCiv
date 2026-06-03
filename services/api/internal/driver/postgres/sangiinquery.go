package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"s4rciv.org/api/internal/port"
)

// ListSangiinVoteEvents lists 参議院 記名投票 (session 0 = latest) for the map selector.
func (q *QueryReader) ListSangiinVoteEvents(ctx context.Context, session, limit, offset int) (int, []port.SangiinVoteEventSummaryView, error) {
	if limit <= 0 {
		limit = 50
	}
	if session == 0 {
		if err := q.pool.QueryRow(ctx,
			`SELECT COALESCE(MAX(session),0) FROM interpretation.sangiin_vote_event`).Scan(&session); err != nil {
			return 0, nil, err
		}
	}
	rows, err := q.pool.Query(ctx, `
		SELECT vote_event_id, COALESCE(session,0), COALESCE(motion,''),
		       to_char(vote_date,'YYYY-MM-DD'), COALESCE(yes_count,0), COALESCE(no_count,0),
		       COALESCE(permalink,''), observation_seq, observed_at
		FROM interpretation.sangiin_vote_event
		WHERE session = $1
		ORDER BY vote_date DESC NULLS LAST, vote_event_id
		LIMIT $2 OFFSET $3`, session, limit, offset)
	if err != nil {
		return session, nil, err
	}
	defer rows.Close()
	var out []port.SangiinVoteEventSummaryView
	for rows.Next() {
		var s port.SangiinVoteEventSummaryView
		var date *string
		var permalink string
		var seq int64
		var observedAt time.Time
		if err := rows.Scan(&s.VoteEventID, &s.Session, &s.Motion, &date, &s.YesCount, &s.NoCount,
			&permalink, &seq, &observedAt); err != nil {
			return session, nil, err
		}
		if date != nil {
			s.Date = *date
		}
		s.Attr = port.Attribution{Source: "sangiin-vote", Permalink: permalink, FetchedAt: observedAt, ObservationSeq: seq}
		out = append(out, s)
	}
	return session, out, rows.Err()
}

// GetSangiinVoteMap returns the per-都道府県 内訳 + 比例 panel + coverage for one vote.
// Votes join the roster by name_key; unmatched rows are 未集計 (matched < total).
func (q *QueryReader) GetSangiinVoteMap(ctx context.Context, voteEventID string) (port.SangiinVoteMapView, bool, error) {
	var v port.SangiinVoteMapView
	v.VoteEventID = voteEventID
	var date *string
	var permalink string
	var seq int64
	var observedAt time.Time
	err := q.pool.QueryRow(ctx, `
		SELECT COALESCE(session,0), COALESCE(motion,''), to_char(vote_date,'YYYY-MM-DD'),
		       COALESCE(yes_count,0), COALESCE(no_count,0), COALESCE(permalink,''),
		       observation_seq, observed_at
		FROM interpretation.sangiin_vote_event WHERE vote_event_id = $1`, voteEventID,
	).Scan(&v.Session, &v.Motion, &date, &v.YesCount, &v.NoCount, &permalink, &seq, &observedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, false, nil
	}
	if err != nil {
		return v, false, err
	}
	if date != nil {
		v.Date = *date
	}
	v.Attr = port.Attribution{Source: "sangiin-vote", Permalink: permalink, FetchedAt: observedAt, ObservationSeq: seq}

	// Per-都道府県 内訳 (district members only).
	prows, err := q.pool.Query(ctx, `
		SELECT ld.district_code, COALESCE(ld.district_name,''),
		       count(*) FILTER (WHERE sv.option='yes'),
		       count(*) FILTER (WHERE sv.option='no'),
		       count(*) FILTER (WHERE sv.option='abstain')
		FROM interpretation.sangiin_vote sv
		JOIN interpretation.legislator_district ld
		  ON ld.name_key = sv.name_key AND ld.house = '参議院'
		WHERE sv.vote_event_id = $1 AND NOT ld.is_pr
		GROUP BY ld.district_code, ld.district_name
		ORDER BY ld.district_code`, voteEventID)
	if err != nil {
		return v, false, err
	}
	defer prows.Close()
	for prows.Next() {
		var p port.PrefectureTallyView
		if err := prows.Scan(&p.DistrictCode, &p.DistrictName, &p.Yes, &p.No, &p.Abstain); err != nil {
			return v, false, err
		}
		v.Prefectures = append(v.Prefectures, p)
	}
	if err := prows.Err(); err != nil {
		return v, false, err
	}

	// 比例 companion panel (全国区, by 会派).
	rrows, err := q.pool.Query(ctx, `
		SELECT sv.voter_name, sv.option, COALESCE(sv.parliamentary_group,'')
		FROM interpretation.sangiin_vote sv
		JOIN interpretation.legislator_district ld
		  ON ld.name_key = sv.name_key AND ld.house = '参議院'
		WHERE sv.vote_event_id = $1 AND ld.is_pr
		ORDER BY sv.parliamentary_group, sv.voter_name`, voteEventID)
	if err != nil {
		return v, false, err
	}
	defer rrows.Close()
	for rrows.Next() {
		var pr port.SangiinPrVoteView
		if err := rrows.Scan(&pr.VoterName, &pr.Option, &pr.Group); err != nil {
			return v, false, err
		}
		v.PrVotes = append(v.PrVotes, pr)
	}
	if err := rrows.Err(); err != nil {
		return v, false, err
	}

	// Coverage: how many per-member rows joined a roster member.
	if err := q.pool.QueryRow(ctx, `
		SELECT count(*), count(ld.name_key)
		FROM interpretation.sangiin_vote sv
		LEFT JOIN interpretation.legislator_district ld
		  ON ld.name_key = sv.name_key AND ld.house = '参議院'
		WHERE sv.vote_event_id = $1`, voteEventID).Scan(&v.TotalVotes, &v.MatchedVotes); err != nil {
		return v, false, err
	}
	return v, true, nil
}
