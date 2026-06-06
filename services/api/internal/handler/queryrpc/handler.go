// Package queryrpc adapts the read-only QueryReader port to the generated
// Connect-RPC QueryService. Every response carries Attribution + permalink
// (ADR-000004). It is the Handler layer: validation + mapping only, no logic.
package queryrpc

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"

	queryv1 "s4rciv.org/api/gen/s4rciv/query/v1"
	"s4rciv.org/api/internal/port"
)

type Handler struct {
	reader    port.QueryReader
	lawReader port.LawQueryReader
}

func New(reader port.QueryReader, lawReader port.LawQueryReader) *Handler {
	return &Handler{reader: reader, lawReader: lawReader}
}

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

func (h *Handler) ListVoteEvents(ctx context.Context, req *connect.Request[queryv1.ListVoteEventsRequest]) (*connect.Response[queryv1.ListVoteEventsResponse], error) {
	limit := int(req.Msg.GetPageSize())
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := parseOffset(req.Msg.GetPageToken())
	f := port.VoteEventFilter{
		Session:      int(req.Msg.GetSession()),
		House:        req.Msg.GetHouse(),
		MappableOnly: req.Msg.GetMappableOnly(),
		Limit:        limit + 1, // over-fetch to detect a next page
		Offset:       offset,
	}
	session, items, err := h.reader.ListVoteEvents(ctx, f)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := &queryv1.ListVoteEventsResponse{Session: int32(session)}
	if len(items) > limit {
		out.NextPageToken = strconv.Itoa(offset + limit)
		items = items[:limit]
	}
	for _, s := range items {
		out.VoteEvents = append(out.VoteEvents, toVoteEventSummary(s))
	}
	return connect.NewResponse(out), nil
}

func toVoteEventSummary(s port.VoteEventSummaryView) *queryv1.VoteEventSummary {
	return &queryv1.VoteEventSummary{
		VoteEventId:   s.VoteEventID,
		IssueId:       s.IssueID,
		Session:       int32(s.Session),
		House:         s.House,
		MeetingName:   s.MeetingName,
		Motion:        s.Motion,
		Date:          s.Date,
		Result:        s.Result,
		YesCount:      int32(s.YesCount),
		NoCount:       int32(s.NoCount),
		AbstainCount:  int32(s.AbstainCount),
		HasNamedVotes: s.HasNamedVotes,
		Attribution:   toAttribution(s.Attr),
	}
}

func (h *Handler) ListSangiinVoteEvents(ctx context.Context, req *connect.Request[queryv1.ListSangiinVoteEventsRequest]) (*connect.Response[queryv1.ListSangiinVoteEventsResponse], error) {
	limit := int(req.Msg.GetPageSize())
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := parseOffset(req.Msg.GetPageToken())
	session, items, err := h.reader.ListSangiinVoteEvents(ctx, int(req.Msg.GetSession()), limit+1, offset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := &queryv1.ListSangiinVoteEventsResponse{Session: int32(session)}
	if len(items) > limit {
		out.NextPageToken = strconv.Itoa(offset + limit)
		items = items[:limit]
	}
	for _, s := range items {
		out.VoteEvents = append(out.VoteEvents, &queryv1.SangiinVoteEventSummary{
			VoteEventId: s.VoteEventID, Session: int32(s.Session), Motion: s.Motion,
			Date: s.Date, YesCount: int32(s.YesCount), NoCount: int32(s.NoCount),
			Attribution: toAttribution(s.Attr),
		})
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) GetSangiinVoteMap(ctx context.Context, req *connect.Request[queryv1.GetSangiinVoteMapRequest]) (*connect.Response[queryv1.GetSangiinVoteMapResponse], error) {
	id := req.Msg.GetVoteEventId()
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("vote_event_id is required"))
	}
	v, found, err := h.reader.GetSangiinVoteMap(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !found {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("sangiin vote %s not found", id))
	}
	out := &queryv1.GetSangiinVoteMapResponse{
		VoteEventId: v.VoteEventID, Session: int32(v.Session), Motion: v.Motion, Date: v.Date,
		YesCount: int32(v.YesCount), NoCount: int32(v.NoCount),
		TotalVotes: int32(v.TotalVotes), MatchedVotes: int32(v.MatchedVotes),
		Attribution: toAttribution(v.Attr),
	}
	for _, p := range v.Prefectures {
		out.Prefectures = append(out.Prefectures, &queryv1.PrefectureTally{
			DistrictCode: p.DistrictCode, DistrictName: p.DistrictName,
			Yes: int32(p.Yes), No: int32(p.No), Abstain: int32(p.Abstain),
		})
	}
	for _, pr := range v.PrVotes {
		out.PrVotes = append(out.PrVotes, &queryv1.SangiinPrVote{
			VoterName: pr.VoterName, Option: pr.Option, ParliamentaryGroup: pr.Group,
		})
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) GetLaw(ctx context.Context, req *connect.Request[queryv1.GetLawRequest]) (*connect.Response[queryv1.GetLawResponse], error) {
	id := req.Msg.GetLawId()
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("law_id is required"))
	}
	lv, nodes, found, err := h.lawReader.GetLaw(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !found {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("law %s not found", id))
	}
	out := &queryv1.GetLawResponse{Law: toLaw(lv)}
	for _, n := range nodes {
		out.Nodes = append(out.Nodes, toLawNode(n))
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) ListLaws(ctx context.Context, req *connect.Request[queryv1.ListLawsRequest]) (*connect.Response[queryv1.ListLawsResponse], error) {
	limit := int(req.Msg.GetPageSize())
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := parseOffset(req.Msg.GetPageToken())

	views, err := h.lawReader.ListLaws(ctx, req.Msg.GetLawType(), limit+1, offset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := &queryv1.ListLawsResponse{}
	if len(views) > limit {
		out.NextPageToken = strconv.Itoa(offset + limit)
		views = views[:limit]
	}
	for _, lv := range views {
		out.Laws = append(out.Laws, toLaw(lv))
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) GetLawChanges(ctx context.Context, req *connect.Request[queryv1.GetLawChangesRequest]) (*connect.Response[queryv1.GetLawChangesResponse], error) {
	id := req.Msg.GetLawId()
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("law_id is required"))
	}
	limit := int(req.Msg.GetPageSize())
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := parseOffset(req.Msg.GetPageToken())

	changes, err := h.lawReader.GetLawChanges(ctx, id, limit+1, offset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := &queryv1.GetLawChangesResponse{}
	if len(changes) > limit {
		out.NextPageToken = strconv.Itoa(offset + limit)
		changes = changes[:limit]
	}
	for _, c := range changes {
		out.Changes = append(out.Changes, toLawChange(c))
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) ListTimeline(ctx context.Context, req *connect.Request[queryv1.ListTimelineRequest]) (*connect.Response[queryv1.ListTimelineResponse], error) {
	limit := int(req.Msg.GetPageSize())
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	cursorSeq, backward := parseTimelineCursor(req.Msg.GetPageToken())
	f := port.TimelineFilter{
		Source:         req.Msg.GetSource(),
		EventType:      req.Msg.GetEventType(),
		Classification: req.Msg.GetClassification(),
		Since:          req.Msg.GetSince(),
		Until:          req.Msg.GetUntil(),
		Keyword:        capKeyword(req.Msg.GetKeyword()),
		CursorSeq:      cursorSeq,
		Backward:       backward,
		Limit:          limit + 1, // keyset over-fetch to detect a further page in the walk direction
	}
	items, err := h.reader.ListTimeline(ctx, f)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// The driver returns items newest-first (DESC) for both directions. The +1
	// over-fetch sits on the far side of the walk: the oldest row for a forward
	// (older) page, the newest row for a backward (newer) page. Drop it on the
	// matching end so the page itself stays contiguous.
	over := len(items) > limit
	if over {
		if backward {
			items = items[1:] // extra newest row → a newer page exists
		} else {
			items = items[:limit] // extra oldest row → an older page exists
		}
	}

	// Older (next, seq <) / newer (prev, seq >) existence. The over-fetch proves
	// the walk side; the opposite side is implied by the cursor we arrived on — a
	// "before" cursor means newer rows exist, an "after" cursor means older rows do.
	olderExists, newerExists := over, cursorSeq != 0
	if backward {
		olderExists, newerExists = true, over
	}
	out := &queryv1.ListTimelineResponse{}
	if len(items) > 0 {
		if olderExists {
			out.NextPageToken = "b:" + strconv.FormatInt(items[len(items)-1].Seq, 10)
		}
		if newerExists {
			out.PrevPageToken = "a:" + strconv.FormatInt(items[0].Seq, 10)
		}
	}
	// Orientation only — keyset has no random page access (research-confirmed): the
	// total feeds "全 X 件" / total pages, and the current page = (rows newer than
	// this page's head)/pageSize + 1. A full filtered count, so once per request.
	var aboveSeq int64
	if len(items) > 0 {
		aboveSeq = items[0].Seq
	}
	total, above, err := h.reader.CountTimeline(ctx, f, aboveSeq)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out.TotalCount = int64(total)
	if len(items) > 0 {
		out.Page = int32(above/limit + 1)
	}
	for _, it := range items {
		out.Items = append(out.Items, toTimelineItem(it))
	}
	return connect.NewResponse(out), nil
}

func toTimelineItem(v port.TimelineItemView) *queryv1.TimelineItem {
	out := &queryv1.TimelineItem{
		Seq:                 v.Seq,
		EventType:           v.EventType,
		Source:              v.Source,
		StreamId:            v.StreamID,
		ObservedAt:          v.ObservedAt.UTC().Format(time.RFC3339),
		Title:               v.Title,
		Subtitle:            v.Subtitle,
		IssueId:             v.IssueID,
		LawId:               v.LawID,
		FeaturedVoteEventId: v.FeaturedVoteEventID,
		Classification:      v.Classification,
		ClassConfidence:     v.ClassConfidence,
		NodesAdded:          int32(v.NodesAdded),
		NodesDeleted:        int32(v.NodesDeleted),
		NodesModified:       int32(v.NodesModified),
		WasOcr:              v.WasOCR,
		Attribution:         toAttribution(v.Attr),
	}
	if v.SourcePublishedAt != nil {
		out.SourcePublishedAt = v.SourcePublishedAt.UTC().Format(time.RFC3339)
	}
	return out
}

func (h *Handler) ListLegislatorVotes(ctx context.Context, req *connect.Request[queryv1.ListLegislatorVotesRequest]) (*connect.Response[queryv1.ListLegislatorVotesResponse], error) {
	id := req.Msg.GetPersonId()
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("person_id is required"))
	}
	limit := int(req.Msg.GetPageSize())
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := parseOffset(req.Msg.GetPageToken())

	lv, found, err := h.reader.VotesByPerson(ctx, id, limit+1, offset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !found {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("person %s not found", id))
	}
	out := &queryv1.ListLegislatorVotesResponse{
		PersonId:           lv.PersonID,
		PersonName:         lv.PersonName,
		IdentityConfidence: lv.IdentityConfidence,
	}
	votes := lv.Votes
	if len(votes) > limit {
		out.NextPageToken = strconv.Itoa(offset + limit)
		votes = votes[:limit]
	}
	for _, v := range votes {
		out.Votes = append(out.Votes, toLegislatorVote(v))
	}
	return connect.NewResponse(out), nil
}

func toLegislatorVote(v port.LegislatorVoteView) *queryv1.LegislatorVote {
	return &queryv1.LegislatorVote{
		VoteEventId: v.VoteEventID,
		IssueId:     v.IssueID,
		Motion:      v.Motion,
		Option:      v.Option,
		Result:      v.Result,
		MeetingName: v.MeetingName,
		House:       v.House,
		Date:        v.Date,
		Confidence:  v.Confidence,
		Attribution: toAttribution(v.Attr),
	}
}

func toLaw(lv port.LawView) *queryv1.Law {
	return &queryv1.Law{
		LawId:                    lv.Law.LawID,
		LawNum:                   lv.Law.LawNum,
		LawType:                  lv.Law.LawType,
		LawTitle:                 lv.Law.Title,
		LawTitleKana:             lv.Law.TitleKana,
		Category:                 lv.Law.Category,
		PromulgationDate:         lv.Law.PromulgationDate,
		CurrentRevisionId:        lv.Law.CurrentRevisionID,
		AmendmentEnforcementDate: lv.Law.AmendmentEnforcementDate,
		CurrentRevisionStatus:    lv.Law.CurrentRevisionStatus,
		RepealStatus:             lv.Law.RepealStatus,
		RepealDate:               lv.Law.RepealDate,
		Attribution:              toAttribution(lv.Attr),
	}
}

func toLawNode(n port.LawNodeView) *queryv1.LawNode {
	return &queryv1.LawNode{
		Eid:          n.Node.EID,
		ParentEid:    n.Node.ParentEID,
		NodeType:     n.Node.NodeType,
		Num:          n.Node.Num,
		Caption:      n.Node.Caption,
		ChapterNum:   n.Node.ChapterNum,
		SectionNum:   n.Node.SectionNum,
		IsSuppl:      n.Node.IsSuppl,
		SentenceText: n.Node.SentenceText,
		Ordinal:      int32(n.Node.Ordinal),
	}
}

func toLawChange(c port.LawChangeView) *queryv1.LawChange {
	out := &queryv1.LawChange{
		ObservationSeq:  c.ObservationSeq,
		DifferVersion:   c.DifferVersion,
		Classification:  c.Classification,
		ClassConfidence: c.ClassConfidence,
		ObservedAt:      c.ObservedAt.UTC().Format(time.RFC3339),
	}
	for _, nc := range c.NodeChanges {
		out.NodeChanges = append(out.NodeChanges, &queryv1.LawNodeChange{
			Eid:      nc.EID,
			Op:       nc.Op,
			NodeType: nc.NodeType,
			Num:      nc.Num,
			PrevText: nc.PrevText,
			CurrText: nc.CurrText,
		})
	}
	return out
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
		SourceSpeechId:   v.Event.SourceSpeechID,
		Attribution:      toAttribution(v.Attr),
	}
	for _, vt := range v.Event.Votes {
		out.Votes = append(out.Votes, &queryv1.Vote{
			Option:             vt.Option,
			VoterName:          vt.VoterName,
			PersonId:           vt.PersonID,
			Confidence:         vt.Confidence,
			House:              vt.House,
			DistrictCode:       vt.DistrictCode,
			IsPr:               vt.IsPR,
			PrBlock:            vt.PRBlock,
			ParliamentaryGroup: vt.Group,
		})
	}
	return out
}

// GetStreamVerification serves the per-Stream verifiable export (ADR-000014).
// It maps each event's domain facts to the canonical HashableEvent via
// EventFacts.Hashable() — the same projection the collector hashed — so the
// bytes the reader's browser re-marshals are identical. The server asserts no
// "verified" flag; it returns ground-truth fields and lets the reader judge.
func (h *Handler) GetStreamVerification(ctx context.Context, req *connect.Request[queryv1.GetStreamVerificationRequest]) (*connect.Response[queryv1.GetStreamVerificationResponse], error) {
	streamID := req.Msg.GetStreamId()
	if streamID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("stream_id is required"))
	}
	view, found, err := h.reader.StreamVerification(ctx, streamID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !found {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("stream %s not found", streamID))
	}
	out := &queryv1.GetStreamVerificationResponse{
		StreamId:   view.StreamID,
		Source:     view.Source,
		AlgVersion: view.AlgVersion,
	}
	for _, e := range view.Events {
		out.Events = append(out.Events, &queryv1.VerifiableEvent{
			Seq:      e.Seq,
			Hashable: e.Facts.Hashable(),
			LogHash:  e.LogHash,
		})
	}
	if cp := view.Checkpoint; cp != nil {
		out.HasCheckpoint = true
		out.Checkpoint = &queryv1.VerificationCheckpoint{
			ThroughSeq:  cp.ThroughSeq,
			TreeSize:    cp.TreeSize,
			RootHash:    cp.RootHash,
			AlgVersion:  cp.AlgVersion,
			Signed:      cp.Signed,
			SignerKeyId: cp.SignerKeyID,
			RecordedAt:  cp.RecordedAt.UTC().Format(time.RFC3339),
		}
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) GetMastheadStatus(ctx context.Context, _ *connect.Request[queryv1.GetMastheadStatusRequest]) (*connect.Response[queryv1.GetMastheadStatusResponse], error) {
	watch, cp, has, err := h.reader.MastheadStatus(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := &queryv1.GetMastheadStatusResponse{WatchCount: watch}
	if has {
		out.HasCheckpoint = true
		out.Checkpoint = &queryv1.VerificationCheckpoint{
			ThroughSeq:  cp.ThroughSeq,
			TreeSize:    cp.TreeSize,
			RootHash:    cp.RootHash,
			AlgVersion:  cp.AlgVersion,
			Signed:      cp.Signed,
			SignerKeyId: cp.SignerKeyID,
			RecordedAt:  cp.RecordedAt.UTC().Format(time.RFC3339),
		}
	}
	return connect.NewResponse(out), nil
}

func toAttribution(a port.Attribution) *queryv1.Attribution {
	return &queryv1.Attribution{
		Source:         a.Source,
		Permalink:      a.Permalink,
		FetchedAt:      a.FetchedAt.UTC().Format(time.RFC3339),
		ObservationSeq: a.ObservationSeq,
		WasOcr:         a.WasOCR,
		LogHash:        a.LogHash,
		PrevLogHash:    a.PrevLogHash,
		StreamId:       a.StreamID,
	}
}

// parseTimelineCursor decodes an opaque keyset page token into a seq cursor and a
// walk direction. "b:<seq>" (or a bare "<seq>", for back-compat) walks older rows
// (seq < cursor); "a:<seq>" walks newer rows (seq > cursor). An empty or malformed
// token is the head page (cursor 0, forward).
func parseTimelineCursor(token string) (seq int64, backward bool) {
	if token == "" {
		return 0, false
	}
	s := token
	switch {
	case strings.HasPrefix(s, "a:"):
		backward, s = true, s[2:]
	case strings.HasPrefix(s, "b:"):
		s = s[2:]
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, backward
}

// maxKeywordRunes caps the free-text timeline filter so an over-long pattern can't
// amplify the per-row ILIKE cost (CWE-400). 128 runes is far beyond any real query.
const maxKeywordRunes = 128

func capKeyword(s string) string {
	if r := []rune(s); len(r) > maxKeywordRunes {
		return string(r[:maxKeywordRunes])
	}
	return s
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
