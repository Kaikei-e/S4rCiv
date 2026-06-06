import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import ProvenanceChip from './ProvenanceChip.svelte';

// ProvenanceChip carries the source attribution every record must show
// (DISCIPLINE §7/§9: 出典 permalink + 取得時刻) and, per ADR-000014, is provenance
// ONLY — the truncated hash and the "(未検証)" wording were removed because integrity
// is a per-chain/checkpoint property, not a per-record badge. These tests pin that
// contract so a regression that re-introduces a per-record verdict fails loudly.

const attr = {
	source: 'kokkai',
	permalink: 'https://kokkai.ndl.go.jp/txt/12345',
	fetchedAt: '2026-01-02T03:04:05Z',
	observationSeq: '42'
};

describe('ProvenanceChip', () => {
	it('shows 出典 as an external permalink (source attribution mandate)', () => {
		render(ProvenanceChip, { props: { attr } });
		const link = screen.getByRole('link', { name: /出典 kokkai/ });
		expect(link).toHaveAttribute('href', attr.permalink);
		expect(link).toHaveAttribute('target', '_blank');
		expect(link.getAttribute('rel')).toContain('noopener');
	});

	it('shows the 最終取得 fetch timestamp in JST (not raw UTC) — ADR-000018', () => {
		render(ProvenanceChip, { props: { attr } });
		// 2026-01-02T03:04:05Z (UTC) == 2026-01-02 12:04 in Asia/Tokyo.
		expect(screen.getByText(/最終取得 2026-01-02 12:04 JST/)).toBeInTheDocument();
	});

	it('shows 記録 #seq as a plain citation handle when no verify link is given', () => {
		render(ProvenanceChip, { props: { attr } });
		expect(screen.getByText(/記録 #42/)).toBeInTheDocument();
		expect(screen.queryByRole('link', { name: /記録 #42/ })).toBeNull();
	});

	it('deep-links 記録 #seq into the verification panel when verifyHref is set', () => {
		render(ProvenanceChip, { props: { attr, verifyHref: '/laws/abc#verify-42' } });
		expect(screen.getByRole('link', { name: /記録 #42/ })).toHaveAttribute(
			'href',
			'/laws/abc#verify-42'
		);
	});

	it('never renders a per-record "未検証" verdict (ADR-000014 regression guard)', () => {
		render(ProvenanceChip, { props: { attr, verifyHref: '/x#verify-42' } });
		expect(screen.queryByText(/未検証/)).toBeNull();
	});

	it('falls back to a non-linked 出典 — when the source has no permalink', () => {
		render(ProvenanceChip, { props: { attr: { fetchedAt: '2026-01-02T03:04:05Z' } } });
		expect(screen.queryByRole('link')).toBeNull();
		expect(screen.getByText(/出典 —/)).toBeInTheDocument();
	});
});
