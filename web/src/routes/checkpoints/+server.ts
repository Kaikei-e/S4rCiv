import type { RequestHandler } from './$types';
import { listCheckpoints } from '$lib/server/queryClient';

// Public, read-only signed-checkpoint feed (ADR-000019, passive exposure). Third-party
// witnesses and archivers (e.g. the Internet Archive) PULL this; S4RCIV never pushes —
// staying a passive sentinel (設計原則①). Each `signedNote` is the canonical C2SP
// signed-note a third party verifies with the published key; it is NOT a self-graded
// "verified" flag. linked-v1 is not yet wired to the public witness network (root is
// the chain head, not a Merkle root) — that arrives with merkle-v1.
export const GET: RequestHandler = async () => {
	// Fail closed (F-14): an upstream failure must not look like "no checkpoints yet"
	// to a verifying witness. 200 + empty list is reserved for a genuinely empty
	// checkpoint table; failures return 503 and are never cached.
	let res;
	try {
		res = await listCheckpoints(200);
	} catch (e) {
		console.error('[checkpoints] RPC failed:', e);
		return new Response(JSON.stringify({ error: 'upstream unavailable' }), {
			status: 503,
			headers: {
				'content-type': 'application/json; charset=utf-8',
				'cache-control': 'no-store'
			}
		});
	}

	const checkpoints = (res.checkpoints ?? []).map((c) => ({
		throughSeq: Number(c.throughSeq ?? 0),
		treeSize: Number(c.treeSize ?? 0),
		rootHash: c.rootHash ?? '',
		algVersion: c.algVersion ?? '',
		signerKeyId: c.signerKeyId ?? '',
		recordedAt: c.recordedAt ?? '',
		// signedNote arrives base64 (proto3 JSON bytes); decode to the UTF-8 note text
		// using web-standard globals (no Node Buffer / @types/node dependency).
		signedNote: c.signedNote
			? new TextDecoder().decode(Uint8Array.from(atob(c.signedNote), (ch) => ch.charCodeAt(0)))
			: ''
	}));

	const body = JSON.stringify(
		{
			about:
				'S4RCIV signed observation-log checkpoints (C2SP signed-note). Pull and verify with the published key; S4RCIV never pushes. See the repository / ADR-000019.',
			checkpoints
		},
		null,
		2
	);
	return new Response(body, {
		headers: {
			'content-type': 'application/json; charset=utf-8',
			'cache-control': 'public, max-age=300'
		}
	});
};
