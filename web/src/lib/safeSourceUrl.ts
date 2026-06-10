// Upstream-stored permalinks (attribution.permalink) are rendered as hyperlinks
// only when they point at a known public primary source over https. Anything else
// (javascript:, data:, plain http:, unknown hosts, unparseable strings) returns
// null and the caller shows the same label as plain text — the record itself is
// never hidden; only the link affordance is withheld.
const ALLOWED_HOSTS: ReadonlySet<string> = new Set([
	'kokkai.ndl.go.jp',
	'laws.e-gov.go.jp',
	'www.sangiin.go.jp',
	'www.shugiin.go.jp'
]);

export function safeSourceUrl(url: string): string | null {
	let parsed: URL;
	try {
		parsed = new URL(url);
	} catch {
		return null;
	}
	if (parsed.protocol !== 'https:') return null;
	if (!ALLOWED_HOSTS.has(parsed.hostname)) return null;
	return url;
}
