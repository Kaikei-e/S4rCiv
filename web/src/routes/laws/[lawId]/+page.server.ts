import type { PageServerLoad } from './$types';
import { error } from '@sveltejs/kit';
import { getLaw, getLawChanges, getStreamVerification } from '$lib/server/queryClient';
import type { StreamVerificationJson } from '$lib/verification/verifier';

export const load: PageServerLoad = async ({ params }) => {
	let law, changes;
	try {
		[law, changes] = await Promise.all([getLaw(params.lawId), getLawChanges(params.lawId)]);
	} catch (e) {
		throw error(404, e instanceof Error ? e.message : '法令の取得に失敗しました');
	}
	if (!law.law) throw error(404, '法令が見つかりません');

	// Per-case verification payload (ADR-000014). Fail soft: it must not 404 the page.
	let verification: StreamVerificationJson | null = null;
	const streamId = law.law.attribution?.streamId;
	if (streamId) {
		try {
			verification = await getStreamVerification(streamId);
		} catch {
			verification = null;
		}
	}
	return {
		law: law.law,
		nodes: law.nodes ?? [],
		changes: changes.changes ?? [],
		verification
	};
};
