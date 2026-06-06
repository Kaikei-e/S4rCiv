import { describe, it, expect } from 'vitest';
import { toJstMinute, toJstDate, toJstShortLabelled } from './time';

// Pins the JST contract (ADR-000018): RFC3339 UTC in, Asia/Tokyo out, regardless of
// the runtime's local zone. 03:04Z + 9h = 12:04 JST; a late-evening UTC time crosses
// the date boundary into the next JST day.
describe('time (JST formatting)', () => {
	it('converts UTC minute to JST', () => {
		expect(toJstMinute('2026-01-02T03:04:05Z')).toBe('2026-01-02 12:04');
	});

	it('crosses the date boundary (UTC evening → next JST day)', () => {
		expect(toJstMinute('2026-01-02T15:30:00Z')).toBe('2026-01-03 00:30');
		expect(toJstDate('2026-01-02T15:30:00Z')).toBe('2026-01-03');
	});

	it('date-only form is JST, not a raw UTC slice', () => {
		expect(toJstDate('2026-01-02T03:04:05Z')).toBe('2026-01-02');
	});

	it('short labelled form carries the JST zone', () => {
		expect(toJstShortLabelled('2026-06-06T09:00:00Z')).toBe('06-06 18:00 JST');
	});

	it('empty/invalid input yields empty string', () => {
		expect(toJstMinute('')).toBe('');
		expect(toJstMinute(undefined)).toBe('');
		expect(toJstMinute('not-a-date')).toBe('');
		expect(toJstDate(null)).toBe('');
	});
});
