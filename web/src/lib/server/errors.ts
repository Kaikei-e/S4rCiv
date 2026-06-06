// Map a Connect-RPC failure onto a SvelteKit error WITHOUT leaking the upstream
// message to the browser (CWE-209). A genuine NotFound becomes a 404 with fixed
// copy; anything else (Internal, Unavailable, transport faults) becomes a 502 with
// generic copy, and the real detail is logged server-side only. The API also
// returns a generic message for Internal errors (queryrpc.SanitizeErrors), so this
// is the second layer of the same guarantee — the browser never sees driver/DB text.
import { error } from '@sveltejs/kit';
import { Code, ConnectError } from '@connectrpc/connect';

export function rpcError(e: unknown, notFoundMessage: string): never {
	if (e instanceof ConnectError && e.code === Code.NotFound) {
		throw error(404, notFoundMessage);
	}
	console.error('[query] RPC failed:', e);
	throw error(502, '一時的に取得できませんでした。時間をおいて再度お試しください。');
}
