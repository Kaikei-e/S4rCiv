import { defineConfig } from 'vitest/config';
import { fileURLToPath } from 'node:url';

// Standalone test config: the verification tests are pure TypeScript (protobuf-es
// + WebCrypto + the generated message), so they don't need the SvelteKit plugin —
// keeping it out avoids the plugin's app-only resolution quirks under Vitest. We
// only need the `$lib` alias to match the app's import paths. WebCrypto and the
// node:fs golden-file read run under Vitest's default `node` environment.
export default defineConfig({
	test: {
		environment: 'node',
		include: ['src/**/*.{test,spec}.ts']
	},
	resolve: {
		alias: {
			$lib: fileURLToPath(new URL('./src/lib', import.meta.url))
		}
	}
});
