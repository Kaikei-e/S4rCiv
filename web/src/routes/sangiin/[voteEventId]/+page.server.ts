import type { PageServerLoad } from './$types';
import { error } from '@sveltejs/kit';
import { getSangiinVoteMap } from '$lib/server/queryClient';
import { rpcError } from '$lib/server/errors';

export const load: PageServerLoad = async ({ params }) => {
	let res;
	try {
		res = await getSangiinVoteMap(params.voteEventId);
	} catch (e) {
		rpcError(e, '記名投票が見つかりません');
	}
	if (!res.voteEventId) throw error(404, '記名投票が見つかりません');
	return { map: res };
};
