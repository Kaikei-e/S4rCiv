import type { PageServerLoad } from './$types';
import { listVoteEvents } from '$lib/server/queryClient';

export const load: PageServerLoad = async () => {
	// session 0 = 現会期 (latest observed); mappable = 記名投票 records exist.
	const res = await listVoteEvents(0);
	return { session: res.session ?? 0, voteEvents: res.voteEvents ?? [] };
};
