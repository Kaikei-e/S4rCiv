// Package queryrpc adapts the read-only QueryReader port to the generated
// Connect-RPC QueryService. Every response carries Attribution + permalink
// (ADR-000004). It is the Handler layer: validation + mapping only, no logic.
package queryrpc

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"connectrpc.com/connect"

	queryv1 "s4rciv.org/api/gen/s4rciv/query/v1"
	"s4rciv.org/api/internal/port"
)

type Handler struct {
	reader port.QueryReader
}

func New(reader port.QueryReader) *Handler { return &Handler{reader: reader} }

func (h *Handler) GetMeeting(ctx context.Context, req *connect.Request[queryv1.GetMeetingRequest]) (*connect.Response[queryv1.GetMeetingResponse], error) {
	id := req.Msg.GetIssueId()
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("issue_id is required"))
	}
	mv, speeches, found, err := h.reader.Meeting(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !found {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("meeting %s not found", id))
	}
	out := &queryv1.GetMeetingResponse{Meeting: toMeeting(mv)}
	for _, s := range speeches {
		out.Speeches = append(out.Speeches, toSpeech(s))
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) ListMeetings(ctx context.Context, req *connect.Request[queryv1.ListMeetingsRequest]) (*connect.Response[queryv1.ListMeetingsResponse], error) {
	limit := int(req.Msg.GetPageSize())
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := parseOffset(req.Msg.GetPageToken())

	views, err := h.reader.ListMeetings(ctx, int(req.Msg.GetSession()), req.Msg.GetHouse(), limit+1, offset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := &queryv1.ListMeetingsResponse{}
	if len(views) > limit { // there is a next page
		out.NextPageToken = strconv.Itoa(offset + limit)
		views = views[:limit]
	}
	for _, mv := range views {
		out.Meetings = append(out.Meetings, toMeeting(mv))
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) GetVoteEvent(ctx context.Context, req *connect.Request[queryv1.GetVoteEventRequest]) (*connect.Response[queryv1.GetVoteEventResponse], error) {
	id := req.Msg.GetVoteEventId()
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("vote_event_id is required"))
	}
	v, found, err := h.reader.VoteEvent(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !found {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("vote event %s not found", id))
	}
	return connect.NewResponse(&queryv1.GetVoteEventResponse{VoteEvent: toVoteEvent(v)}), nil
}

func toMeeting(mv port.MeetingView) *queryv1.Meeting {
	return &queryv1.Meeting{
		IssueId:     mv.Meeting.IssueID,
		Session:     int32(mv.Meeting.Session),
		House:       mv.Meeting.House,
		MeetingName: mv.Meeting.MeetingName,
		Issue:       mv.Meeting.Issue,
		Date:        mv.Meeting.Date,
		Attribution: toAttribution(mv.Attr),
	}
}

func toSpeech(s port.SpeechView) *queryv1.Speech {
	return &queryv1.Speech{
		SpeechId:        s.Speech.SpeechID,
		IssueId:         s.Speech.IssueID,
		SpeechOrder:     int32(s.Speech.Order),
		Speaker:         s.Speech.Speaker,
		SpeakerGroup:    s.Speech.SpeakerGroup,
		SpeakerPosition: s.Speech.SpeakerPosition,
		Speech:          s.Speech.Text,
		PersonId:        s.Speech.PersonID,
		Attribution:     toAttribution(s.Attr),
	}
}

func toVoteEvent(v port.VoteEventView) *queryv1.VoteEvent {
	out := &queryv1.VoteEvent{
		VoteEventId:      v.Event.VoteEventID,
		IssueId:          v.Event.IssueID,
		Motion:           v.Event.Motion,
		YesCount:         int32(v.Event.YesCount),
		NoCount:          int32(v.Event.NoCount),
		AbstainCount:     int32(v.Event.AbstainCount),
		Result:           v.Event.Result,
		Confidence:       v.Event.Confidence,
		NeedsReview:      v.Event.NeedsReview,
		ExtractorVersion: v.Event.ExtractorVersion,
		Attribution:      toAttribution(v.Attr),
	}
	for _, vt := range v.Event.Votes {
		out.Votes = append(out.Votes, &queryv1.Vote{
			Option:     vt.Option,
			VoterName:  vt.VoterName,
			PersonId:   vt.PersonID,
			Confidence: vt.Confidence,
		})
	}
	return out
}

func toAttribution(a port.Attribution) *queryv1.Attribution {
	return &queryv1.Attribution{
		Source:         a.Source,
		Permalink:      a.Permalink,
		FetchedAt:      a.FetchedAt.UTC().Format(time.RFC3339),
		ObservationSeq: a.ObservationSeq,
		WasOcr:         a.WasOCR,
	}
}

func parseOffset(token string) int {
	if token == "" {
		return 0
	}
	n, err := strconv.Atoi(token)
	if err != nil || n < 0 {
		return 0
	}
	return n
}
