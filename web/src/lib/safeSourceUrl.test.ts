import { describe, it, expect } from 'vitest';
import { safeSourceUrl } from './safeSourceUrl';

// attribution.permalink is stored upstream-derived data; it is rendered as a
// hyperlink only when it is https and points at a known primary-source host.
// Everything else degrades to plain text (the caller receives null).

describe('safeSourceUrl', () => {
	it.each([
		'https://kokkai.ndl.go.jp/txt/121505261X01219890613/0',
		'https://laws.e-gov.go.jp/law/140AC0000000045',
		'https://www.sangiin.go.jp/japanese/joho1/kousei/vote/220/220-0101-1.html',
		'https://www.shugiin.go.jp/internet/itdb_gian.nsf/html/gian/keika/1DDDE63.htm'
	])('passes the allowlisted primary-source host: %s', (url) => {
		expect(safeSourceUrl(url)).toBe(url);
	});

	it.each([
		['javascript: scheme', 'javascript:alert(1)'],
		['data: scheme', 'data:text/html,<script>alert(1)</script>'],
		['plain http', 'http://kokkai.ndl.go.jp/txt/1'],
		['foreign https host', 'https://example.com/kokkai.ndl.go.jp'],
		['allowlisted host as userinfo', 'https://kokkai.ndl.go.jp@example.com/'],
		['subdomain of an allowlisted host', 'https://evil.kokkai.ndl.go.jp/'],
		['unparseable string', 'not a url'],
		['empty string', '']
	])('rejects %s', (_label, url) => {
		expect(safeSourceUrl(url)).toBeNull();
	});
});
