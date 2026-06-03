import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [sveltekit()],
	ssr: {
		// Bundle the Connect client + protobuf runtime into the server build so the
		// adapter-node output stays self-contained (the runtime image ships no
		// node_modules). Node built-ins they use (http/http2) remain external.
		noExternal: ['@connectrpc/connect', '@connectrpc/connect-node', '@bufbuild/protobuf']
	}
});
