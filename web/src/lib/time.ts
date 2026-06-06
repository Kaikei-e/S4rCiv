// Time formatting for display. Observation/fetch timestamps arrive as RFC3339 UTC
// (e.g. "2026-06-02T09:00:00Z"); the dashboard is for a Japanese audience, so all
// human-facing times are shown in JST. The timeZone is pinned to 'Asia/Tokyo' and
// never reads the runtime's local zone, so SSR (container TZ, usually UTC) and the
// browser format identically and hydration is stable (DESIGN_LANGUAGE §4 / ADR-000018).

const JST_MINUTE = new Intl.DateTimeFormat('ja-JP', {
	timeZone: 'Asia/Tokyo',
	year: 'numeric',
	month: '2-digit',
	day: '2-digit',
	hour: '2-digit',
	minute: '2-digit',
	hour12: false
});

function parts(iso: string): Record<string, string> | null {
	const d = new Date(iso);
	if (Number.isNaN(d.getTime())) return null;
	const out: Record<string, string> = {};
	for (const p of JST_MINUTE.formatToParts(d)) out[p.type] = p.value;
	return out;
}

/** RFC3339 UTC → "YYYY-MM-DD HH:mm" in JST. Empty/invalid input → "". */
export function toJstMinute(iso: string | undefined | null): string {
	if (!iso) return '';
	const p = parts(iso);
	if (!p) return '';
	return `${p.year}-${p.month}-${p.day} ${p.hour}:${p.minute}`;
}

/** RFC3339 UTC → "YYYY-MM-DD" in JST (date only). Empty/invalid input → "". */
export function toJstDate(iso: string | undefined | null): string {
	if (!iso) return '';
	const p = parts(iso);
	if (!p) return '';
	return `${p.year}-${p.month}-${p.day}`;
}

/** RFC3339 UTC → "MM-DD HH:mm JST" — compact form with an explicit zone label. */
export function toJstShortLabelled(iso: string | undefined | null): string {
	if (!iso) return '';
	const p = parts(iso);
	if (!p) return '';
	return `${p.month}-${p.day} ${p.hour}:${p.minute} JST`;
}
