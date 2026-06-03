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
	GetSangiinVoteMapResponse
} from '$lib/types';

const BASE = (env.API_URL ?? 'http://127.0.0.1:8080').replace(/\/$/, '');

const client = createPromiseClient(
	QueryService,
	createConnectTransport({ baseUrl: BASE, httpVersion: '1.1' })
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
