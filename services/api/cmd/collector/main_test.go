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
