import type { PageServerLoad } from './$types';
import { listVoteEvents } from '$lib/server/queryClient';
import type { VoteEventSummary } from '$lib/types';

export const load: PageServerLoad = async () => {
	try {
		// session 0 = 現会期 (latest observed); mappable = 記名投票 records exist.
		const res = await listVoteEvents(0);
		return {
			session: res.session ?? 0,
			voteEvents: res.voteEvents ?? [],
			error: null as string | null
		};
	} catch (e) {
		// Degrade like the timeline: render the page with an empty list + a generic
		// banner; never surface the upstream error text to the browser (CWE-209).
		console.error('[votes] RPC failed:', e);
		return {
			session: 0,
			voteEvents: [] as VoteEventSummary[],
			error: '一時的に取得できませんでした。時間をおいて再度お試しください。'
		};
	}
};
