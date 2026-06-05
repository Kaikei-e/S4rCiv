package main

import (
	"testing"
	"time"
)

// recentScope is the rolling discover window used by autoDiscover (ADR-000012):
// [today-90d, today] inclusive, in YYYY-MM-DD.
func TestRecentScopeIsRolling90DayWindow(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	s := recentScope(now)

	if s.Until != "2026-06-05" {
		t.Fatalf("until = %q, want today 2026-06-05", s.Until)
	}
	from, err := time.Parse("2006-01-02", s.From)
	if err != nil {
		t.Fatalf("from %q is not a date: %v", s.From, err)
	}
	until, _ := time.Parse("2006-01-02", s.Until)
	if d := until.Sub(from); d != discoverWindowDays*24*time.Hour {
		t.Fatalf("window = %v, want %d days", d, discoverWindowDays)
	}
	if s.Max != 0 {
		t.Fatalf("max = %d, want 0 (no cap on auto-discover)", s.Max)
	}
}

// nudge wakes the project loop without ever blocking the poll loop, coalescing
// multiple pending wakes into one (ADR-000015).
func TestNudgeNeverBlocksAndCoalesces(t *testing.T) {
	ch := make(chan struct{}, 1)
	nudge(ch) // fills the buffer

	done := make(chan struct{})
	go func() { nudge(ch); close(done) }() // must not block on a full buffer
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("nudge blocked on a full channel")
	}

	select {
	case <-ch:
	default:
		t.Fatal("expected one pending wake")
	}
	select {
	case <-ch:
		t.Fatal("expected wakes to coalesce to one")
	default:
	}
}
