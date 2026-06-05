import { defineConfig, devices } from '@playwright/test';

// Browser E2E runs against the REAL, deterministically seeded compose stack
// (browser → SvelteKit SSR → Go api → Postgres). `make e2e` brings the stack up
// health-gated (docker compose up --wait db migrate api web) and runs the `seed`
// service first; Playwright only points at it via baseURL. We deliberately do NOT
// use Playwright's `webServer` to boot `vite dev` — that front-end would have no
// backend. Override the target with E2E_BASE_URL.
const BASE_URL = process.env.E2E_BASE_URL ?? 'http://127.0.0.1:3000';

export default defineConfig({
	testDir: './e2e',
	fullyParallel: true,
	forbidOnly: !!process.env.CI,
	retries: process.env.CI ? 2 : 0,
	reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : 'list',
	use: {
		baseURL: BASE_URL,
		trace: 'on-first-retry',
		testIdAttribute: 'data-testid'
	},
	projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }]
});
