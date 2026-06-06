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
	streamId?: string;
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
	nextPageToken?: string; // older page (seq <); "" = no older page
	prevPageToken?: string; // newer page (seq >); "" = at head / no newer page
	totalCount?: string; // int64 → JSON string; total rows matching the filter (total pages = ceil/page_size)
	page?: number; // 1-based current page for "n / N ページ" display; 0 when empty
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

// ── 選挙区投票地図 (district vote map; ADR-000008) ────────────────────────────────

export interface Vote {
	option?: string; // yes | no | abstain
	voterName?: string;
	personId?: string;
	confidence?: string;
	house?: string; // 衆議院 | 参議院 (from the roster, read-time join)
	districtCode?: string; // == GeoJSON kucode; "" when isPr
	isPr?: boolean; // 比例選出 — shown in the companion panel, never erased (§5)
	prBlock?: string;
	parliamentaryGroup?: string; // 会派
}

export interface VoteEvent {
	voteEventId?: string;
	issueId?: string;
	motion?: string;
	yesCount?: number;
	noCount?: number;
	abstainCount?: number;
	result?: string;
	confidence?: string;
	needsReview?: boolean;
	extractorVersion?: string;
	sourceSpeechId?: string;
	votes?: Vote[];
	attribution?: Attribution;
}

export interface GetVoteEventResponse {
	voteEvent?: VoteEvent;
}

export interface VoteEventSummary {
	voteEventId?: string;
	issueId?: string;
	session?: number;
	house?: string;
	meetingName?: string;
	motion?: string;
	date?: string;
	result?: string;
	yesCount?: number;
	noCount?: number;
	abstainCount?: number;
	hasNamedVotes?: boolean;
	attribution?: Attribution;
}

export interface ListVoteEventsResponse {
	session?: number;
	voteEvents?: VoteEventSummary[];
	nextPageToken?: string;
}

// ── 参議院本会議投票結果 マップ (ADR-000010) ──────────────────────────────────────

export interface SangiinVoteEventSummary {
	voteEventId?: string;
	session?: number;
	motion?: string;
	date?: string;
	yesCount?: number;
	noCount?: number;
	attribution?: Attribution;
}

export interface ListSangiinVoteEventsResponse {
	session?: number;
	voteEvents?: SangiinVoteEventSummary[];
	nextPageToken?: string;
}

export interface PrefectureTally {
	districtCode?: string; // JIS prefecture code(s); "31,32" for a 合区
	districtName?: string;
	yes?: number;
	no?: number;
	abstain?: number;
}

export interface SangiinPrVote {
	voterName?: string;
	option?: string; // yes | no | abstain
	parliamentaryGroup?: string;
}

export interface GetSangiinVoteMapResponse {
	voteEventId?: string;
	session?: number;
	motion?: string;
	date?: string;
	yesCount?: number;
	noCount?: number;
	prefectures?: PrefectureTally[];
	prVotes?: SangiinPrVote[];
	totalVotes?: number;
	matchedVotes?: number;
	attribution?: Attribution;
}

// Global provenance for the masthead (ADR-000018/000019). A commitment, never a
// self-graded "verified" flag.
export interface MastheadCheckpoint {
	throughSeq?: string;
	treeSize?: string;
	rootHash?: string;
	algVersion?: string;
	signed?: boolean;
	signerKeyId?: string;
	recordedAt?: string;
}
export interface MastheadStatus {
	watchCount?: string;
	hasCheckpoint?: boolean;
	checkpoint?: MastheadCheckpoint;
}

// One signed checkpoint for the public passive-exposure feed (ADR-000019).
export interface SignedCheckpoint {
	throughSeq?: string;
	treeSize?: string;
	rootHash?: string;
	algVersion?: string;
	signerKeyId?: string;
	recordedAt?: string;
	signedNote?: string; // base64 (proto3 JSON bytes) of the C2SP signed-note
}
export interface ListCheckpointsResponse {
	checkpoints?: SignedCheckpoint[];
}
