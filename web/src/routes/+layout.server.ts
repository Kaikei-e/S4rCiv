import type { LayoutServerLoad } from './$types';
import { getMastheadStatus } from '$lib/server/queryClient';
import type { MastheadStatus } from '$lib/types';

// The masthead's provenance row (coverage + latest signed checkpoint) loads on every
// page via the root layout. A failure here must never break the page — degrade to the
// stance line only (the masthead hides coverage/checkpoint when data is absent).
export const load: LayoutServerLoad = async () => {
	try {
		return { masthead: (await getMastheadStatus()) as MastheadStatus };
	} catch (e) {
		console.error('[masthead] RPC failed:', e);
		return { masthead: null as MastheadStatus | null };
	}
};
