import { test, expect } from '@playwright/test';

// The cross-source timeline is the citizen's main 動線 ("いつ・何が・どう変わったか").
// Asserts against the deterministic seed (`make seed`): one 国会 meeting (kokkai,
// 予算委員会) and one 法令 change (egov-law, テスト民生安定法 with a substantive diff).
// Per Playwright best practice it never depends on ambient DB data — only the seed.
test.describe('横断タイムライン (home)', () => {
	test('shows cross-source items with state, classification and provenance', async ({ page }) => {
		await page.goto('/');

		// Scope assertions to the timeline region so they don't match the filter
		// <select> options (which also contain "実質的変更" etc.).
		const timeline = page.getByLabel('横断タイムライン');
		await expect(timeline).toBeVisible();

		// Both sources surface in the one cross-source view. The law appears as two
		// items (its ResourceObserved then ResourceChanged events — 1 row per event),
		// so scope to the first match.
		await expect(timeline.getByRole('link', { name: '予算委員会' })).toBeVisible();
		await expect(timeline.getByRole('link', { name: 'テスト民生安定法' }).first()).toBeVisible();

		// The law change is classified substantive — shown as an UNREVIEWED auto-result,
		// never an established judgment (DESIGN §6 / ChangeLogItem).
		await expect(timeline.getByText(/実質的変更/).first()).toBeVisible();
		await expect(timeline.getByText(/自動分類\(未レビュー\)/).first()).toBeVisible();

		// Every row carries source attribution (DISCIPLINE §7/§9).
		await expect(timeline.getByText(/出典/).first()).toBeVisible();
		// ADR-000014: the chip is provenance only — no per-record "未検証" verdict.
		await expect(timeline.getByText(/未検証/)).toHaveCount(0);
	});

	test('filtering by source is reflected in the URL and narrows the list', async ({ page }) => {
		await page.goto('/?source=egov-law');
		await expect(page).toHaveURL(/source=egov-law/);
		await expect(page.getByRole('link', { name: 'テスト民生安定法' }).first()).toBeVisible();
		await expect(page.getByRole('link', { name: '予算委員会' })).toHaveCount(0);
	});

	test('a timeline item deep-links to its detail page', async ({ page }) => {
		await page.goto('/');
		await page.getByRole('link', { name: 'テスト民生安定法' }).first().click();
		await expect(page).toHaveURL(/\/laws\/999AC0000000999/);
		await expect(page.getByRole('heading', { name: 'テスト民生安定法' })).toBeVisible();
	});
});
