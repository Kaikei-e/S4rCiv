import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import BackToTop from './BackToTop.svelte';

// BackToTop is the floating "先頭へ" control: a labelled in-page anchor to #main that
// is hidden (out of the tab order and the a11y tree) until the reader scrolls ~1.5
// viewports down, then revealed and keyboard-focusable. These tests pin that reveal
// contract via the bound aria-hidden/tabindex/href, independent of (untestable) CSS.

function setScrollY(y: number) {
	Object.defineProperty(window, 'scrollY', { value: y, configurable: true });
}

describe('BackToTop', () => {
	beforeEach(() => {
		setScrollY(0); // jsdom innerHeight defaults to 768
	});

	it('is a labelled in-page anchor to #main', () => {
		render(BackToTop);
		const link = screen.getByRole('link', { hidden: true });
		expect(link).toHaveAttribute('href', '#main');
		expect(link).toHaveTextContent('先頭へ'); // labelled, not icon-only (NN/g)
	});

	it('is hidden and out of the tab order before scrolling', () => {
		render(BackToTop);
		const link = screen.getByRole('link', { hidden: true });
		expect(link).toHaveAttribute('aria-hidden', 'true');
		expect(link).toHaveAttribute('tabindex', '-1');
	});

	it('reveals and becomes keyboard-focusable after scrolling past ~1.5 viewports', async () => {
		render(BackToTop);
		setScrollY(window.innerHeight * 2);
		window.dispatchEvent(new Event('scroll'));
		const link = await screen.findByRole('link', { name: /先頭へ/ });
		expect(link).toHaveAttribute('aria-hidden', 'false');
		expect(link).not.toHaveAttribute('tabindex');
	});

	it('stays hidden when the reader has barely scrolled', () => {
		render(BackToTop);
		setScrollY(window.innerHeight * 0.5);
		window.dispatchEvent(new Event('scroll'));
		expect(screen.getByRole('link', { hidden: true })).toHaveAttribute('aria-hidden', 'true');
	});
});
