import type { PageServerLoad } from './$types';
import { error } from '@sveltejs/kit';
import { listLegislatorVotes } from '$lib/server/queryClient';

export const load: PageServerLoad = async ({ params }) => {
	let res;
	try {
		res = await listLegislatorVotes(params.personId);
	} catch (e) {
		throw error(404, e instanceof Error ? e.message : '取得に失敗しました');
	}
	return {
		personId: res.personId ?? params.personId,
		personName: res.personName ?? '',
		identityConfidence: res.identityConfidence ?? '',
		votes: res.votes ?? []
	};
};
