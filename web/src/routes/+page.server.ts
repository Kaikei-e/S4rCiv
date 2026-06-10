import type { PageServerLoad } from './$types';
import { listTimeline } from '$lib/server/queryClient';
import type { TimelineItem } from '$lib/types';

// SSR/BFF (D1): the timeline is fetched server-side from the private API. Filters
// are read from the URL so the page works without JS (progressive enhancement)
// and every view is a shareable, indexable URL — the "誰でも辿れる" mandate.
// Clamp URL-supplied values before forwarding them upstream: trim and cap the
// length so an oversized query string can't be relayed to the RPC as-is. Filter
// values are short labels (100 is generous); page tokens are opaque cursors (64).
const clamp = (v: string | null, max: number) => (v ?? '').trim().slice(0, max);

export const load: PageServerLoad = async ({ url }) => {
	const filters = {
		source: clamp(url.searchParams.get('source'), 100),
		eventType: clamp(url.searchParams.get('event_type'), 100),
		classification: clamp(url.searchParams.get('classification'), 100),
		keyword: clamp(url.searchParams.get('q'), 100)
	};
	const pageToken = clamp(url.searchParams.get('page'), 64);

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
