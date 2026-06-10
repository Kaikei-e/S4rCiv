import type { PageServerLoad } from './$types';
import { listSangiinVoteEvents } from '$lib/server/queryClient';
import type { SangiinVoteEventSummary } from '$lib/types';

export const load: PageServerLoad = async () => {
	try {
		const res = await listSangiinVoteEvents(0); // 0 = latest session
		return {
			session: res.session ?? 0,
			voteEvents: res.voteEvents ?? [],
			error: null as string | null
		};
	} catch (e) {
		// Degrade like the timeline: render the page with an empty list + a generic
		// banner; never surface the upstream error text to the browser (CWE-209).
		console.error('[sangiin] RPC failed:', e);
		return {
			session: 0,
			voteEvents: [] as SangiinVoteEventSummary[],
			error: '一時的に取得できませんでした。時間をおいて再度お試しください。'
		};
	}
};
