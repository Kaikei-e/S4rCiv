import { test, expect, type Page } from '@playwright/test';

// Mobile-layout regression guard (ADR-000017). Runs against the same deterministic
// seed as the rest of the suite, on both the desktop and the 360px `mobile-chromium`
// project. The no-horizontal-scroll assertions hold at every width; the disclosure /
// touch-target assertions are narrow-only and skip themselves on wide viewports.

// scrollWidth − clientWidth > 0 means content overflows the viewport horizontally —
// the classic responsive break. 1px of rounding slack is tolerated.
async function horizontalOverflow(page: Page): Promise<number> {
	return page.evaluate(() => {
		const d = document.documentElement;
		return d.scrollWidth - d.clientWidth;
	});
}

const isNarrow = (page: Page) => (page.viewportSize()?.width ?? 9999) < 880;

test.describe('レスポンシブ (ADR-000017)', () => {
	test('home fits the viewport width and keeps the timeline reachable', async ({ page }) => {
		await page.goto('/');
		expect(await horizontalOverflow(page)).toBeLessThanOrEqual(1);
		// 市民の主動線（横断タイムライン）は折り畳んだフィルタの下でも到達できる。
		await expect(page.getByLabel('横断タイムライン')).toBeVisible();
	});

	test('the votes map list fits the viewport width', async ({ page }) => {
		await page.goto('/votes');
		expect(await horizontalOverflow(page)).toBeLessThanOrEqual(1);
	});

	test('filters collapse behind a disclosure and toggle open (narrow)', async ({ page }) => {
		await page.goto('/');
		test.skip(!isNarrow(page), 'disclosure is only present on narrow viewports');
		// Closed by default → the controls are hidden until the citizen opens them.
		const source = page.locator('select[name="source"]');
		await expect(source).toBeHidden();
		await page.getByRole('button', { name: /絞り込み/ }).click();
		await expect(source).toBeVisible();
	});

	test('primary nav links meet the 44px touch target (narrow)', async ({ page }) => {
		await page.goto('/');
		test.skip(!isNarrow(page), 'touch-target sizing applies to coarse pointers');
		const box = await page.getByRole('link', { name: /衆院/ }).boundingBox();
		expect(box?.height ?? 0).toBeGreaterThanOrEqual(44);
	});
});
