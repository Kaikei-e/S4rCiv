import type { PageServerLoad } from './$types';
import { listLegislatorVotes } from '$lib/server/queryClient';
import { rpcError } from '$lib/server/errors';

export const load: PageServerLoad = async ({ params }) => {
	let res;
	try {
		res = await listLegislatorVotes(params.personId);
	} catch (e) {
		rpcError(e, '議員が見つかりません');
	}
	return {
		personId: res.personId ?? params.personId,
		personName: res.personName ?? '',
		identityConfidence: res.identityConfidence ?? '',
		votes: res.votes ?? []
	};
};
