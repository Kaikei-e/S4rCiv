import type { PageServerLoad } from './$types';
import { error } from '@sveltejs/kit';
import { getMeeting } from '$lib/server/queryClient';

export const load: PageServerLoad = async ({ params }) => {
	let res;
	try {
		res = await getMeeting(params.issueId);
	} catch (e) {
		throw error(404, e instanceof Error ? e.message : '会議録の取得に失敗しました');
	}
	if (!res.meeting) throw error(404, '会議録が見つかりません');
	return { meeting: res.meeting, speeches: res.speeches ?? [] };
};
