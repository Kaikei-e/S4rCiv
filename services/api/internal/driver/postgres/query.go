package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"s4rciv.org/api/internal/port"
)

// QueryReader serves the read-only Connect-RPC query side from the read models.
type QueryReader struct {
	pool *pgxpool.Pool
}

func NewQueryReader(pool *pgxpool.Pool) *QueryReader { return &QueryReader{pool: pool} }

func (q *QueryReader) Meeting(ctx context.Context, issueID string) (port.MeetingView, []port.SpeechView, bool, error) {
	var mv port.MeetingView
	var date *string
	var observedAt time.Time
	var seq int64
	err := q.pool.QueryRow(ctx, `
		SELECT issue_id, stream_id, COALESCE(session,0), COALESCE(house,''),
		       COALESCE(meeting_name,''), COALESCE(issue,''),
		       to_char(meeting_date, 'YYYY-MM-DD'), COALESCE(permalink,''),
		       was_ocr, observation_seq, observed_at
		FROM interpretation.meeting WHERE issue_id = $1`, issueID,
	).Scan(&mv.Meeting.IssueID, &mv.Meeting.StreamID, &mv.Meeting.Session, &mv.Meeting.House,
		&mv.Meeting.MeetingName, &mv.Meeting.Issue, &date, &mv.Meeting.Permalink,
		&mv.Meeting.WasOCR, &seq, &observedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return mv, nil, false, nil
	}
	if err != nil {
		return mv, nil, false, err
	}
	if date != nil {
		mv.Meeting.Date = *date
	}
	src := sourceOf(mv.Meeting.StreamID)
	mv.Attr = port.Attribution{
		Source: src, Permalink: mv.Meeting.Permalink, FetchedAt: observedAt,
		ObservationSeq: seq, WasOCR: mv.Meeting.WasOCR,
	}

	speeches, err := q.speeches(ctx, issueID, src)
	if err != nil {
		return mv, nil, false, err
	}
	return mv, speeches, true, nil
}

func (q *QueryReader) speeches(ctx context.Context, issueID, src string) ([]port.SpeechView, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT speech_id, issue_id, speech_order, COALESCE(speaker,''),
		       COALESCE(speaker_yomi,''), COALESCE(speaker_group,''),
		       COALESCE(speaker_position,''), COALESCE(speech,''),
		       COALESCE(speech_url,''), COALESCE(person_id,''), observation_seq, observed_at
		FROM interpretation.speech WHERE issue_id = $1 ORDER BY speech_order`, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []port.SpeechView
	for rows.Next() {
		var v port.SpeechView
		var seq int64
		var observedAt time.Time
		if err := rows.Scan(&v.Speech.SpeechID, &v.Speech.IssueID, &v.Speech.Order,
			&v.Speech.Speaker, &v.Speech.SpeakerYomi, &v.Speech.SpeakerGroup,
			&v.Speech.SpeakerPosition, &v.Speech.Text, &v.Speech.SpeechURL,
			&v.Speech.PersonID, &seq, &observedAt); err != nil {
			return nil, err
		}
		v.Attr = port.Attribution{
			Source: src, Permalink: v.Speech.SpeechURL, FetchedAt: observedAt, ObservationSeq: seq,
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (q *QueryReader) ListMeetings(ctx context.Context, session int, house string, limit, offset int) ([]port.MeetingView, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := q.pool.Query(ctx, `
		SELECT issue_id, stream_id, COALESCE(session,0), COALESCE(house,''),
		       COALESCE(meeting_name,''), COALESCE(issue,''),
		       to_char(meeting_date, 'YYYY-MM-DD'), COALESCE(permalink,''),
		       was_ocr, observation_seq, observed_at
		FROM interpretation.meeting
		WHERE ($1 = 0 OR session = $1) AND ($2 = '' OR house = $2)
		ORDER BY meeting_date DESC NULLS LAST, issue_id DESC
		LIMIT $3 OFFSET $4`, session, house, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []port.MeetingView
	for rows.Next() {
		var mv port.MeetingView
		var date *string
		var observedAt time.Time
		var seq int64
		if err := rows.Scan(&mv.Meeting.IssueID, &mv.Meeting.StreamID, &mv.Meeting.Session,
			&mv.Meeting.House, &mv.Meeting.MeetingName, &mv.Meeting.Issue, &date,
			&mv.Meeting.Permalink, &mv.Meeting.WasOCR, &seq, &observedAt); err != nil {
			return nil, err
		}
		if date != nil {
			mv.Meeting.Date = *date
		}
		mv.Attr = port.Attribution{
			Source: sourceOf(mv.Meeting.StreamID), Permalink: mv.Meeting.Permalink,
			FetchedAt: observedAt, ObservationSeq: seq, WasOCR: mv.Meeting.WasOCR,
		}
		out = append(out, mv)
	}
	return out, rows.Err()
}

func (q *QueryReader) VoteEvent(ctx context.Context, voteEventID string) (port.VoteEventView, bool, error) {
	var v port.VoteEventView
	var motion, sourceSpeechID *string
	var observedAt time.Time
	var seq int64
	var streamID, permalink string
	err := q.pool.QueryRow(ctx, `
		SELECT ve.vote_event_id, ve.issue_id, ve.motion, COALESCE(ve.yes_count,0),
		       COALESCE(ve.no_count,0), COALESCE(ve.abstain_count,0), ve.result,
		       ve.confidence, ve.needs_review, ve.extractor_version, ve.source_speech_id,
		       ve.observation_seq, ve.observed_at, COALESCE(m.permalink,''), m.stream_id
		FROM interpretation.vote_event ve
		JOIN interpretation.meeting m ON m.issue_id = ve.issue_id
		WHERE ve.vote_event_id = $1`, voteEventID,
	).Scan(&v.Event.VoteEventID, &v.Event.IssueID, &motion, &v.Event.YesCount,
		&v.Event.NoCount, &v.Event.AbstainCount, &v.Event.Result, &v.Event.Confidence,
		&v.Event.NeedsReview, &v.Event.ExtractorVersion, &sourceSpeechID, &seq, &observedAt, &permalink, &streamID)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, false, nil
	}
	if err != nil {
		return v, false, err
	}
	if motion != nil {
		v.Event.Motion = *motion
	}
	if sourceSpeechID != nil {
		v.Event.SourceSpeechID = *sourceSpeechID
	}
	v.Attr = port.Attribution{
		Source: sourceOf(streamID), Permalink: permalink, FetchedAt: observedAt, ObservationSeq: seq,
	}

	// Read-time district binding (ADR-000008): left-join the giin-roster read model
	// on person_id. LEFT JOIN keeps unresolved / non-current-roster voters (現会期
	// スコープ) — their district fields stay empty and the map renders them 記録なし.
	rows, err := q.pool.Query(ctx, `
		SELECT v.option, v.voter_name, COALESCE(v.person_id,''), v.confidence,
		       COALESCE(ld.house,''), COALESCE(ld.district_code,''),
		       COALESCE(ld.is_pr,false), COALESCE(ld.pr_block,''), COALESCE(ld.parliamentary_group,'')
		FROM interpretation.vote v
		LEFT JOIN interpretation.legislator_district ld ON ld.person_id = v.person_id
		WHERE v.vote_event_id = $1 ORDER BY v.id`, voteEventID)
	if err != nil {
		return v, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var vt port.StoredVote
		if err := rows.Scan(&vt.Option, &vt.VoterName, &vt.PersonID, &vt.Confidence,
			&vt.House, &vt.DistrictCode, &vt.IsPR, &vt.PRBlock, &vt.Group); err != nil {
			return v, false, err
		}
		v.Event.Votes = append(v.Event.Votes, vt)
	}
	return v, true, rows.Err()
}

// ListVoteEvents backs the district-vote-map selector (ADR-000008): the 記名投票 of a
// Diet session (Session 0 = the latest observed one, 現会期スコープ). Counts only; the
// per-district options come from VoteEvent. has_named_votes marks the events that
// actually carry per-person records and so can be rendered on the map.
func (q *QueryReader) ListVoteEvents(ctx context.Context, f port.VoteEventFilter) (int, []port.VoteEventSummaryView, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	session := f.Session
	if session == 0 {
		if err := q.pool.QueryRow(ctx, `
			SELECT COALESCE(MAX(m.session), 0)
			FROM interpretation.vote_event ve
			JOIN interpretation.meeting m ON m.issue_id = ve.issue_id`).Scan(&session); err != nil {
			return 0, nil, err
		}
	}
	rows, err := q.pool.Query(ctx, `
		SELECT ve.vote_event_id, ve.issue_id, COALESCE(m.session,0), COALESCE(m.house,''),
		       COALESCE(m.meeting_name,''), COALESCE(ve.motion,''),
		       to_char(m.meeting_date,'YYYY-MM-DD'), COALESCE(ve.result,'unknown'),
		       COALESCE(ve.yes_count,0), COALESCE(ve.no_count,0), COALESCE(ve.abstain_count,0),
		       EXISTS(SELECT 1 FROM interpretation.vote v WHERE v.vote_event_id = ve.vote_event_id),
		       COALESCE(m.permalink,''), ve.observation_seq, ve.observed_at, m.stream_id
		FROM interpretation.vote_event ve
		JOIN interpretation.meeting m ON m.issue_id = ve.issue_id
		WHERE m.session = $1 AND ($2 = '' OR m.house = $2)
		  AND (NOT $3 OR EXISTS(SELECT 1 FROM interpretation.vote v WHERE v.vote_event_id = ve.vote_event_id))
		ORDER BY m.meeting_date DESC NULLS LAST, ve.vote_event_id
		LIMIT $4 OFFSET $5`, session, f.House, f.MappableOnly, limit, f.Offset)
	if err != nil {
		return session, nil, err
	}
	defer rows.Close()

	var out []port.VoteEventSummaryView
	for rows.Next() {
		var s port.VoteEventSummaryView
		var date *string
		var observedAt time.Time
		var seq int64
		var streamID, permalink string
		if err := rows.Scan(&s.VoteEventID, &s.IssueID, &s.Session, &s.House,
			&s.MeetingName, &s.Motion, &date, &s.Result,
			&s.YesCount, &s.NoCount, &s.AbstainCount, &s.HasNamedVotes,
			&permalink, &seq, &observedAt, &streamID); err != nil {
			return session, nil, err
		}
		if date != nil {
			s.Date = *date
		}
		s.Attr = port.Attribution{
			Source: sourceOf(streamID), Permalink: permalink, FetchedAt: observedAt, ObservationSeq: seq,
		}
		out = append(out, s)
	}
	return session, out, rows.Err()
}

func sourceOf(streamID string) string {
	if i := strings.IndexByte(streamID, ':'); i > 0 {
		return streamID[:i]
	}
	return ""
}
