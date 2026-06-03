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
			filters,
			error: null as string | null
		};
	} catch (e) {
		return {
			items: [] as TimelineItem[],
			nextPageToken: '',
			filters,
			error: e instanceof Error ? e.message : 'タイムラインの取得に失敗しました'
		};
	}
};
