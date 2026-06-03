import type { PageServerLoad } from './$types';
import { error } from '@sveltejs/kit';
import { getVoteEvent } from '$lib/server/queryClient';

export const load: PageServerLoad = async ({ params }) => {
	let res;
	try {
		res = await getVoteEvent(params.voteEventId);
	} catch (e) {
		throw error(404, e instanceof Error ? e.message : '記名投票の取得に失敗しました');
	}
	if (!res.voteEvent) throw error(404, '記名投票が見つかりません');
	return { voteEvent: res.voteEvent };
};
