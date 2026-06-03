// Shared read-model view types, mirroring services/api/proto/.../query.proto.
// proto3 JSON mapping: int64 → string, field names lowerCamelCase. These will be
// superseded by the buf-generated types (D2) once codegen runs; kept here (not in
// $lib/server) so Svelte components may import the types without pulling server code.

export interface Attribution {
	source?: string;
	permalink?: string;
	fetchedAt?: string;
	observationSeq?: string;
	wasOcr?: boolean;
	logHash?: string;
	prevLogHash?: string;
}

export interface TimelineItem {
	seq?: string;
	eventType?: string;
	source?: string;
	streamId?: string;
	observedAt?: string;
	sourcePublishedAt?: string;
	title?: string;
	subtitle?: string;
	issueId?: string;
	lawId?: string;
	featuredVoteEventId?: string;
	classification?: string;
	classConfidence?: string;
	nodesAdded?: number;
	nodesDeleted?: number;
	nodesModified?: number;
	wasOcr?: boolean;
	attribution?: Attribution;
}

export interface ListTimelineRequest {
	source?: string;
	eventType?: string;
	classification?: string;
	since?: string;
	until?: string;
	keyword?: string;
	pageSize?: number;
	pageToken?: string;
}

export interface ListTimelineResponse {
	items?: TimelineItem[];
	nextPageToken?: string;
}

// ── Meeting detail (kokkai; full speeches, §7-safe) ─────────────────────────

export interface Meeting {
	issueId?: string;
	session?: number;
	house?: string;
	meetingName?: string;
	issue?: string;
	date?: string;
	attribution?: Attribution;
}

export interface Speech {
	speechId?: string;
	issueId?: string;
	speechOrder?: number;
	speaker?: string;
	speakerGroup?: string;
	speakerPosition?: string;
	speech?: string;
	personId?: string;
	attribution?: Attribution;
}

export interface GetMeetingResponse {
	meeting?: Meeting;
	speeches?: Speech[];
}

// ── Law detail + change (E2) ────────────────────────────────────────────────

export interface Law {
	lawId?: string;
	lawNum?: string;
	lawType?: string;
	lawTitle?: string;
	lawTitleKana?: string;
	category?: string;
	promulgationDate?: string;
	currentRevisionId?: string;
	amendmentEnforcementDate?: string;
	currentRevisionStatus?: string;
	repealStatus?: string;
	repealDate?: string;
	attribution?: Attribution;
}

export interface LawNode {
	eid?: string;
	parentEid?: string;
	nodeType?: string;
	num?: string;
	caption?: string;
	chapterNum?: string;
	sectionNum?: string;
	isSuppl?: boolean;
	sentenceText?: string;
	ordinal?: number;
}

export interface GetLawResponse {
	law?: Law;
	nodes?: LawNode[];
}

export interface LawNodeChange {
	eid?: string;
	op?: string; // added | deleted | modified | moved
	nodeType?: string;
	num?: string;
	prevText?: string;
	currText?: string;
}

export interface LawChange {
	observationSeq?: string;
	differVersion?: string;
	classification?: string;
	classConfidence?: string;
	observedAt?: string;
	nodeChanges?: LawNodeChange[];
}

export interface GetLawChangesResponse {
	changes?: LawChange[];
	nextPageToken?: string;
}

// ── Per-legislator vote record (B2) ─────────────────────────────────────────

export interface LegislatorVote {
	voteEventId?: string;
	issueId?: string;
	motion?: string;
	option?: string; // yes | no | abstain
	result?: string;
	meetingName?: string;
	house?: string;
	date?: string;
	confidence?: string;
	attribution?: Attribution;
}

export interface ListLegislatorVotesResponse {
	personId?: string;
	personName?: string;
	identityConfidence?: string;
	votes?: LegislatorVote[];
	nextPageToken?: string;
}
