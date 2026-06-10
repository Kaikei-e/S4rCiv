// Read-only access to the Connect-RPC QueryService, server-side only (SSR/BFF;
// D1): the browser never touches the API, which stays private on the compose
// network. This is the D2 contract — the buf-generated Connect client over a
// connect-node transport, with the proto as the single source of truth.
//
// Each call returns the response as proto3 JSON via Message.toJson(): int64 → string,
// lowerCamelCase keys — exactly the $lib/types shape, and a plain serializable object
// that survives the SvelteKit load boundary (a proto Message instance would not).

import { env } from '$env/dynamic/private';
import { createPromiseClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-node';
import type { Message } from '@bufbuild/protobuf';
import { QueryService } from '$lib/gen/s4rciv/query/v1/query_connect';
import type {
	ListTimelineRequest,
	ListTimelineResponse,
	GetLawResponse,
	GetLawChangesResponse,
	GetMeetingResponse,
	ListLegislatorVotesResponse,
	GetVoteEventResponse,
	ListVoteEventsResponse,
	ListSangiinVoteEventsResponse,
	GetSangiinVoteMapResponse,
	MastheadStatus,
	ListCheckpointsResponse
} from '$lib/types';
// Type-only: the verifier owns the GetStreamVerification JSON shape so the panel
// and this client agree on one definition. import type is erased — no runtime
// (browser-only WebCrypto) code is pulled into the server bundle.
import type { StreamVerificationJson } from '$lib/verification/verifier';

const BASE = (env.API_URL ?? 'http://127.0.0.1:8080').replace(/\/$/, '');

const client = createPromiseClient(
	QueryService,
	// defaultTimeoutMs: a hung upstream must fail the SSR request quickly instead of
	// pinning a Node worker open indefinitely (request-smuggled slowloris resilience).
	createConnectTransport({ baseUrl: BASE, httpVersion: '1.1', defaultTimeoutMs: 10_000 })
);

function json<T>(m: Message): T {
	return m.toJson({ emitDefaultValues: true }) as unknown as T;
}

export async function listTimeline(req: ListTimelineRequest): Promise<ListTimelineResponse> {
	return json<ListTimelineResponse>(await client.listTimeline(req));
}

export async function getMeeting(issueId: string): Promise<GetMeetingResponse> {
	return json<GetMeetingResponse>(await client.getMeeting({ issueId }));
}

export async function getLaw(lawId: string): Promise<GetLawResponse> {
	return json<GetLawResponse>(await client.getLaw({ lawId }));
}

export async function getLawChanges(lawId: string): Promise<GetLawChangesResponse> {
	return json<GetLawChangesResponse>(await client.getLawChanges({ lawId, pageSize: 50 }));
}

export async function listLegislatorVotes(
	personId: string
): Promise<ListLegislatorVotesResponse> {
	return json<ListLegislatorVotesResponse>(
		await client.listLegislatorVotes({ personId, pageSize: 100 })
	);
}

export async function getVoteEvent(voteEventId: string): Promise<GetVoteEventResponse> {
	return json<GetVoteEventResponse>(await client.getVoteEvent({ voteEventId }));
}

// 現会期 (session 0 = latest) の記名投票だけを地図セレクタ用に返す (ADR-000008).
export async function listVoteEvents(session = 0): Promise<ListVoteEventsResponse> {
	return json<ListVoteEventsResponse>(
		await client.listVoteEvents({ session, mappableOnly: true, pageSize: 100 })
	);
}

// 参議院本会議投票結果 (ADR-000010).
export async function listSangiinVoteEvents(session = 0): Promise<ListSangiinVoteEventsResponse> {
	return json<ListSangiinVoteEventsResponse>(
		await client.listSangiinVoteEvents({ session, pageSize: 100 })
	);
}

export async function getSangiinVoteMap(voteEventId: string): Promise<GetSangiinVoteMapResponse> {
	return json<GetSangiinVoteMapResponse>(await client.getSangiinVoteMap({ voteEventId }));
}

// 完全性検証 read surface (ADR-000014): one Stream's events + covering checkpoint,
// for the in-browser verifier. emitDefaultValues keeps the zero/empty HashableEvent
// fields present in the JSON so the verifier re-marshals the exact canonical form.
export async function getStreamVerification(streamId: string): Promise<StreamVerificationJson> {
	return json<StreamVerificationJson>(await client.getStreamVerification({ streamId }));
}

// Global provenance for the masthead (ADR-000018/000019): watch coverage + the latest
// signed checkpoint, if one exists.
export async function getMastheadStatus(): Promise<MastheadStatus> {
	return json<MastheadStatus>(await client.getMastheadStatus({}));
}

// The signed checkpoint feed (ADR-000019), newest first, for passive exposure.
export async function listCheckpoints(limit = 200): Promise<ListCheckpointsResponse> {
	return json<ListCheckpointsResponse>(await client.listCheckpoints({ limit }));
}
