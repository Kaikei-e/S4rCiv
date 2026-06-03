import type { PageServerLoad } from './$types';
import { error } from '@sveltejs/kit';
import { getSangiinVoteMap } from '$lib/server/queryClient';

export const load: PageServerLoad = async ({ params }) => {
	let res;
	try {
		res = await getSangiinVoteMap(params.voteEventId);
	} catch (e) {
		throw error(404, e instanceof Error ? e.message : '記名投票の取得に失敗しました');
	}
	if (!res.voteEventId) throw error(404, '記名投票が見つかりません');
	return { map: res };
};
