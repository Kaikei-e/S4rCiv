import type { PageServerLoad } from './$types';
import { error } from '@sveltejs/kit';
import { getMeeting, getStreamVerification } from '$lib/server/queryClient';
import { rpcError } from '$lib/server/errors';
import type { StreamVerificationJson } from '$lib/verification/verifier';

export const load: PageServerLoad = async ({ params }) => {
	let res;
	try {
		res = await getMeeting(params.issueId);
	} catch (e) {
		rpcError(e, '会議録が見つかりません');
	}
	if (!res.meeting) throw error(404, '会議録が見つかりません');

	// Per-case verification payload (ADR-000014), keyed by the observation-plane
	// stream_id the backend resolved. Fail soft: a verification hiccup must not 404
	// the page — the panel just won't render.
	let verification: StreamVerificationJson | null = null;
	const streamId = res.meeting.attribution?.streamId;
	if (streamId) {
		try {
			verification = await getStreamVerification(streamId);
		} catch {
			verification = null;
		}
	}
	return { meeting: res.meeting, speeches: res.speeches ?? [], verification };
};
