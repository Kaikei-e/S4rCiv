import type { PageServerLoad } from './$types';
import { listSangiinVoteEvents } from '$lib/server/queryClient';

export const load: PageServerLoad = async () => {
	const res = await listSangiinVoteEvents(0); // 0 = latest session
	return { session: res.session ?? 0, voteEvents: res.voteEvents ?? [] };
};
