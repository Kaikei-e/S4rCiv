import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import ChangeLogItem from './ChangeLogItem.svelte';
import type { TimelineItem } from '$lib/types';

// A timeline row must never leak internal identifiers to citizens:
//  - ADR-000022: when the interpretation read model has not produced a human title yet
//    (cold start / projection lag), the row degrades to a typed "（名称未取得）", never the
//    raw stream id (egov-law:<id>).
//  - ADR-000021: the cross-source timeline carries no 記録 #seq (a raw global seq is an
//    internal identifier; the citation handle + verification deep-link live on the 事案
//    detail page, where ProvenanceChip is given a verifyHref).
// These tests pin both so a regression that re-leaks an internal id fails loudly.

const base: TimelineItem = {
	seq: '2002',
	eventType: 'ResourceObserved',
	source: 'egov-law',
	streamId: 'egov-law:325M50010000064',
	observedAt: '2026-06-06T16:12:00Z',
	lawId: '325M50010000064',
	attribution: {
		source: 'egov-law',
		permalink: 'https://laws.e-gov.go.jp/law/325M50010000064',
		fetchedAt: '2026-06-06T16:12:00Z',
		observationSeq: '2002'
	}
};

describe('ChangeLogItem', () => {
	it('shows the human title when the read model has one', () => {
		render(ChangeLogItem, { props: { item: { ...base, title: 'テスト法' } } });
		expect(screen.getByRole('link', { name: 'テスト法' })).toBeInTheDocument();
	});

	it('degrades to a typed "（名称未取得）" — never the raw stream id (ADR-000022)', () => {
		render(ChangeLogItem, { props: { item: { ...base, title: '' } } });
		expect(screen.getByText('法令（名称未取得）')).toBeInTheDocument();
		// The internal stream id must not appear anywhere as a label.
		expect(screen.queryByText(/egov-law:325M50010000064/)).toBeNull();
	});

	it('labels a kokkai row "会議録（名称未取得）" when untitled', () => {
		render(ChangeLogItem, {
			props: {
				item: { ...base, source: 'kokkai', streamId: 'kokkai:abc', lawId: undefined, title: '' }
			}
		});
		expect(screen.getByText('会議録（名称未取得）')).toBeInTheDocument();
	});

	it('carries no 記録 #seq on the timeline (ADR-000021)', () => {
		render(ChangeLogItem, { props: { item: { ...base, title: 'テスト法' } } });
		expect(screen.queryByText(/記録 #/)).toBeNull();
	});
});
