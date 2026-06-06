import type { PageServerLoad } from './$types';
import { listTimeline } from '$lib/server/queryClient';
import type { TimelineItem } from '$lib/types';

// SSR/BFF (D1): the timeline is fetched server-side from the private API. Filters
// are read from the URL so the page works without JS (progressive enhancement)
// and every view is a shareable, indexable URL — the "誰でも辿れる" mandate.
export const load: PageServerLoad = async ({ url }) => {
	const filters = {
		source: url.searchParams.get('source') ?? '',
		eventType: url.searchParams.get('event_type') ?? '',
		classification: url.searchParams.get('classification') ?? '',
		keyword: url.searchParams.get('q') ?? ''
	};
	const pageToken = url.searchParams.get('page') ?? '';

	try {
		const res = await listTimeline({ ...filters, pageToken, pageSize: 50 });
		return {
			items: (res.items ?? []) as TimelineItem[],
			nextPageToken: res.nextPageToken ?? '',
			prevPageToken: res.prevPageToken ?? '',
			totalCount: Number(res.totalCount ?? 0), // int64 arrives as a string
			page: res.page ?? 0,
			pageSize: 50,
			filters,
			error: null as string | null
		};
	} catch (e) {
		// Render the page with an empty timeline + a generic banner; never surface the
		// upstream/DB error text to the browser (CWE-209). Detail is logged server-side.
		console.error('[timeline] RPC failed:', e);
		return {
			items: [] as TimelineItem[],
			nextPageToken: '',
			prevPageToken: '',
			totalCount: 0,
			page: 0,
			pageSize: 50,
			filters,
			error: '一時的に取得できませんでした。時間をおいて再度お試しください。'
		};
	}
};
