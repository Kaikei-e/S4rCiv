import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import VerificationPanel from './VerificationPanel.svelte';
import { verifyStream, type StreamVerificationJson } from '$lib/verification/verifier';

// The verifier itself (WebCrypto recompute + content-chain) is pinned by
// verifier.test.ts and the CDC golden vectors. THIS suite pins the COMPONENT
// contract around it, so the verifyStream dependency is mocked: opt-in gating, the
// connected chain rendering, the post-click walk + chain-level banner, honest
// degradation of the checkpoint panel, and the mismatch path (ADR-000025).
vi.mock('$lib/verification/verifier', () => ({ verifyStream: vi.fn() }));
const mockVerify = vi.mocked(verifyStream);

// One stream event in the GetStreamVerification wire shape. `hashable` must be
// real HashableEvent proto-JSON because the panel re-parses it for the marker
// (type → glyph/label) and timestamp; only valid, known fields are set.
function mkEvent(seq: string, type: string, observedAt: string, logHash: string) {
	return {
		seq,
		logHash,
		hashable: {
			eventId: `event-${seq}`,
			streamId: 'egov-law:325M50010000064',
			streamSeq: seq,
			type,
			source: 'egov-law',
			fetcherVersion: 'test',
			observedAt,
			contentHash: `sha256:${'c'.repeat(64)}`,
			prevContentHash: '',
			logPrevHash: '0'.repeat(64)
		}
	};
}

// A two-event stream: an OBSERVED (oldest) then a CHANGED (newest).
function mkData(overrides: Partial<StreamVerificationJson> = {}): StreamVerificationJson {
	return {
		streamId: 'egov-law:325M50010000064',
		source: 'egov-law',
		algVersion: 'linked-v1',
		hasCheckpoint: false,
		events: [
			mkEvent('1180', 'EVENT_TYPE_RESOURCE_OBSERVED', '2026-05-30T00:11:00Z', 'b'.repeat(64)),
			mkEvent('1240', 'EVENT_TYPE_RESOURCE_CHANGED', '2026-06-07T05:03:00Z', 'a'.repeat(64))
		],
		...overrides
	};
}

// The StreamVerificationResult verifyStream would return; `fail` lists seqs whose
// recomputed log_hash does NOT match (the tamper-detected case).
function mkResult(data: StreamVerificationJson, fail: string[] = []) {
	const events = (data.events ?? []).map((e) => ({
		seq: e.seq,
		storedLogHash: e.logHash,
		recomputedLogHash: e.logHash,
		logHashOk: !fail.includes(e.seq),
		contentLinkOk: true as boolean | null
	}));
	return {
		streamId: data.streamId ?? '',
		algVersion: data.algVersion ?? '',
		events,
		allLogHashesOk: events.every((e) => e.logHashOk),
		contentChainOk: true,
		checkpoint: {
			present: data.hasCheckpoint === true,
			signed: data.checkpoint?.signed === true,
			throughSeq: data.checkpoint?.throughSeq ?? null
		}
	};
}

beforeEach(() => {
	mockVerify.mockReset();
	// jsdom has no matchMedia; the panel reads only `.matches`. Forcing reduced-motion
	// makes the reveal walk instant so assertions don't race the 320ms steps.
	vi.stubGlobal(
		'matchMedia',
		vi.fn().mockReturnValue({ matches: true })
	);
});
afterEach(() => vi.unstubAllGlobals());

describe('VerificationPanel — opt-in (ADR-000025)', () => {
	it('does not recompute on mount: no verdict shown, no verifyStream call, button offered', () => {
		render(VerificationPanel, { props: { data: mkData() } });

		// Nothing computed until the reader presses the button.
		expect(mockVerify).not.toHaveBeenCalled();
		expect(screen.queryByText('✓ 一致')).toBeNull();
		// Each node sits in the "（未計算）" state.
		expect(screen.getAllByText('（未計算）')).toHaveLength(2);
		// The opt-in button is present.
		expect(screen.getByRole('button', { name: /この端末で検証する/ })).toBeInTheDocument();
	});

	it('renders the chain from the payload: 記録 #seq, state label, hash — before any compute', () => {
		render(VerificationPanel, { props: { data: mkData() } });

		expect(screen.getByText('記録 #1240')).toBeInTheDocument();
		expect(screen.getByText('記録 #1180')).toBeInTheDocument();
		// CHANGED → 変化, OBSERVED → 観測 (state, not verdict).
		expect(screen.getByText('変化')).toBeInTheDocument();
		expect(screen.getByText('観測')).toBeInTheDocument();
		// The改ざん検知用の値 (log_hash) is shown per node, shortened.
		expect(screen.getByText('aaaaaaaa…aaaaaa')).toBeInTheDocument();
	});
});

describe('VerificationPanel — recompute walk', () => {
	it('after the button is pressed: verifyStream runs, nodes match, chain-level banner appears', async () => {
		const data = mkData();
		mockVerify.mockResolvedValue(mkResult(data));
		render(VerificationPanel, { props: { data } });

		await fireEvent.click(screen.getByRole('button', { name: /この端末で検証する/ }));

		await waitFor(() => expect(screen.getByText(/再現できました/)).toBeInTheDocument());
		expect(mockVerify).toHaveBeenCalledTimes(1);
		expect(screen.getAllByText('✓ 一致')).toHaveLength(2);
		// The closing claim is chain-level (N件すべて), never a per-record trust badge.
		expect(screen.getByText(/件すべてを.*再現できました/)).toBeInTheDocument();
	});

	it('mismatch path: a non-recomputing log_hash shows ✗ and the ⚠ banner', async () => {
		const data = mkData();
		mockVerify.mockResolvedValue(mkResult(data, ['1240']));
		render(VerificationPanel, { props: { data } });

		await fireEvent.click(screen.getByRole('button', { name: /この端末で検証する/ }));

		await waitFor(() => expect(screen.getByText(/一致しない箇所があります/)).toBeInTheDocument());
		expect(screen.getByText('✗ 不一致')).toBeInTheDocument();
		expect(screen.getByText('✓ 一致')).toBeInTheDocument(); // the other node still matched
	});
});

describe('VerificationPanel — honest degrade (v0)', () => {
	it('no checkpoint → states 連鎖内整合のみ, never a fake signature', () => {
		render(VerificationPanel, { props: { data: mkData({ hasCheckpoint: false }) } });
		expect(screen.getByText(/チェックポイントはまだありません/)).toBeInTheDocument();
		expect(screen.queryByText(/署名あり/)).toBeNull();
	});

	it('present but unsigned → states 未署名（v0…）, not 署名あり', () => {
		const data = mkData({
			hasCheckpoint: true,
			checkpoint: { throughSeq: '1240', signed: false }
		});
		render(VerificationPanel, { props: { data } });
		expect(screen.getByText(/未署名（v0/)).toBeInTheDocument();
		expect(screen.queryByText(/署名あり/)).toBeNull();
	});
});
