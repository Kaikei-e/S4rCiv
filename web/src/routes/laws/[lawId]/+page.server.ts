import type { PageServerLoad } from './$types';
import { error } from '@sveltejs/kit';
import { getLaw, getLawChanges } from '$lib/server/queryClient';

export const load: PageServerLoad = async ({ params }) => {
	let law, changes;
	try {
		[law, changes] = await Promise.all([getLaw(params.lawId), getLawChanges(params.lawId)]);
	} catch (e) {
		throw error(404, e instanceof Error ? e.message : '法令の取得に失敗しました');
	}
	if (!law.law) throw error(404, '法令が見つかりません');
	return { law: law.law, nodes: law.nodes ?? [], changes: changes.changes ?? [] };
};
