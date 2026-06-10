import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import DocOutline, { type OutlineItem } from './DocOutline.svelte';

// DocOutline is the desktop reading outline (目次): pure navigation over the read
// model. It renders a <nav> landmark of in-page anchor links and (in a real browser)
// highlights the section in view via IntersectionObserver. These tests pin the static
// contract — landmark, anchor hrefs, readable labels — which holds without IO (jsdom
// has none, so scroll-spy is inert here and nothing is marked current before scroll).

const items: OutlineItem[] = [
	{ id: 'sec-current', label: '現行全文', level: 0 },
	{ id: 'n-art-1', label: '第1条', sub: '目的', level: 1 },
	{ id: 'n-art-2', label: '第2条', level: 1 }
];

describe('DocOutline', () => {
	it('renders a nav landmark labelled by the heading', () => {
		render(DocOutline, { props: { items, heading: '目次' } });
		expect(screen.getByRole('navigation', { name: '目次' })).toBeInTheDocument();
	});

	it('links each entry to its in-page anchor (#id), not an internal identifier label', () => {
		render(DocOutline, { props: { items } });
		expect(screen.getByRole('link', { name: /第1条/ })).toHaveAttribute('href', '#n-art-1');
		expect(screen.getByRole('link', { name: /現行全文/ })).toHaveAttribute('href', '#sec-current');
	});

	it('shows the secondary caption (sub) when present', () => {
		render(DocOutline, { props: { items } });
		expect(screen.getByText('目的')).toBeInTheDocument();
	});

	it('marks no entry as current before any scroll (scroll-spy is inert without IO)', () => {
		render(DocOutline, { props: { items } });
		expect(screen.getByRole('link', { name: /第1条/ })).not.toHaveAttribute('aria-current');
	});

	it('preserves item order — the 議事録 outline stays 発言順 (会議軸, ADR-000004)', () => {
		const speeches: OutlineItem[] = [
			{ id: 'sp-1', label: '議長' },
			{ id: 'sp-2', label: '山田太郎', sub: '与党' },
			{ id: 'sp-3', label: '鈴木花子', sub: '野党' },
			{ id: 'sp-4', label: '山田太郎', sub: '与党' } // same speaker recurs, never grouped
		];
		render(DocOutline, { props: { items: speeches, heading: '目次' } });
		const links = screen.getAllByRole('link');
		expect(links.map((l) => l.getAttribute('href'))).toEqual(['#sp-1', '#sp-2', '#sp-3', '#sp-4']);
	});
});
