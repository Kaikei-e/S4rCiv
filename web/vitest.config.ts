import { defineConfig } from 'vitest/config';
import { fileURLToPath } from 'node:url';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { svelteTesting } from '@testing-library/svelte/vite';

// Two test projects with different needs:
//   - verification: pure-TS CDC + unit tests (protobuf-es + WebCrypto + golden file),
//     plus top-level $lib utilities (e.g. time.ts — Intl only, no DOM). Runs under
//     `node`; no SvelteKit plugin, avoiding its app-only resolution quirks.
//   - components: Svelte component tests under jsdom with Testing Library. The tested
//     components import only `import type` from $lib (erased), so no SvelteKit alias
//     resolution is needed at runtime. Browser-only components (MapLibre maps) and
//     full pages are covered by the Playwright E2E suite instead.
const lib = fileURLToPath(new URL('./src/lib', import.meta.url));

export default defineConfig({
	test: {
		projects: [
			{
				resolve: { alias: { $lib: lib } },
				test: {
					name: 'verification',
					environment: 'node',
					include: [
						'src/lib/verification/**/*.{test,spec}.ts',
						'src/lib/*.{test,spec}.ts'
					]
				}
			},
			{
				plugins: [svelte(), svelteTesting()],
				resolve: { alias: { $lib: lib } },
				test: {
					name: 'components',
					environment: 'jsdom',
					setupFiles: ['./vitest-setup.ts'],
					include: ['src/lib/components/**/*.{test,spec}.ts']
				}
			}
		],
		coverage: {
			provider: 'v8',
			include: ['src/lib/**/*.{ts,svelte}'],
			exclude: ['src/lib/gen/**', 'src/**/*.{test,spec}.*']
		}
	}
});
