//go:build integration

package postgres

import (
	"context"
	"testing"

	obs "s4rciv.org/api/internal/domain/observation"
	"s4rciv.org/api/internal/port"
)

// A single interpretation.change row whose diff serializes node_changes as a JSON
// scalar (e.g. a legacy `null` from a nil-slice marshal) must NOT take down the whole
// timeline. ListTimeline expands node_changes with jsonb_array_elements; on a scalar
// Postgres raises SQLSTATE 22023 ("cannot extract elements from a scalar"), and because
// that expression is in the SELECT list the fault fails the ENTIRE query — one poisoned
// read-model row would blank the public timeline. The CASE guard (ADR-000024) coerces a
// non-array node_changes to an empty array so the row degrades to "0 changes" instead.
func TestListTimeline_ScalarNodeChangesDoesNotFailQuery(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := newTestDB(t)

	// Seed one egov-law ResourceChanged observation event (stream FK → observation.stream,
	// content_hash FK → observation.snapshot).
	const streamID = "egov-law:327R00000001003"
	if err := NewEventLog(pool).EnsureStream(ctx, port.Stream{
		StreamID: streamID, Source: "egov-law", SourceLocalKey: "327R00000001003",
		CanonicalURL: "https://laws.e-gov.go.jp/law/327R00000001003",
	}); err != nil {
		t.Fatalf("ensure stream: %v", err)
	}
	body := []byte("<Law>327R00000001003 v2</Law>")
	snap := &port.Snapshot{
		ContentHash: obs.SumBytes(body), Bytes: body, ByteSize: int64(len(body)), MediaType: "application/xml",
	}
	if err := ensureSnapshot(ctx, pool, snap); err != nil {
		t.Fatalf("ensure snapshot: %v", err)
	}
	ch := snap.ContentHash
	if err := rawInsert(ctx, pool, obs.EventFacts{
		EventID: uuidN(1), StreamID: streamID, StreamSeq: 1,
		Type: obs.ResourceChanged, Source: "egov-law", FetcherVersion: "itest/0.1",
		ObservedAt: baseObserved, ContentHash: &ch, LogPrevHash: headOf(t, pool),
	}); err != nil {
		t.Fatalf("insert event: %v", err)
	}

	var seq int64
	if err := pool.QueryRow(ctx,
		`SELECT seq FROM observation.event WHERE event_id = $1`, uuidN(1)).Scan(&seq); err != nil {
		t.Fatalf("read seq: %v", err)
	}

	// The poison row: node_changes is JSON null (what a nil Go slice marshaled to).
	if _, err := pool.Exec(ctx, `
		INSERT INTO interpretation.change
			(observation_seq, differ_version, diff, classification, class_confidence)
		VALUES ($1, 'differ/test', $2::jsonb, 'administrative', 'high')`,
		seq, `{"law_id":"327R00000001003","node_changes":null}`); err != nil {
		t.Fatalf("seed poison change: %v", err)
	}

	items, err := NewQueryReader(pool).ListTimeline(ctx, port.TimelineFilter{Limit: 50})
	if err != nil {
		t.Fatalf("ListTimeline must tolerate scalar node_changes, got: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("timeline rows = %d, want 1", len(items))
	}
	if got := items[0]; got.NodesAdded != 0 || got.NodesDeleted != 0 || got.NodesModified != 0 {
		t.Fatalf("scalar node_changes must degrade to 0 counts, got +%d -%d ~%d",
			got.NodesAdded, got.NodesDeleted, got.NodesModified)
	}
}

// The keyword filter must match LIKE/ILIKE metacharacters (% _ \) LITERALLY:
// escapeLike escapes the user keyword and the queries declare ESCAPE '\'.
// Unescaped, "100%" would act as "100<anything>" and also match "1000…" titles
// (and the bare ESCAPE syntax itself must round-trip through pgx/Postgres).
func TestListTimeline_KeywordMatchesLikeMetacharactersLiterally(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := newTestDB(t)

	// Two egov-law streams whose read-model titles differ only in literal-vs-wildcard
	// interpretation of "100%". Fictional law titles.
	laws := []struct {
		n     int
		lawID string
		title string
	}{
		{1, "599AC9990000001", "架空数値を100%とする法律"},
		{2, "599AC9990000002", "架空数値を1000とする法律"},
	}
	for _, l := range laws {
		streamID := "egov-law:" + l.lawID
		if err := NewEventLog(pool).EnsureStream(ctx, port.Stream{
			StreamID: streamID, Source: "egov-law", SourceLocalKey: l.lawID,
			CanonicalURL: "https://laws.e-gov.go.jp/law/" + l.lawID,
		}); err != nil {
			t.Fatalf("ensure stream %s: %v", l.lawID, err)
		}
		body := []byte("<Law>" + l.lawID + "</Law>")
		snap := &port.Snapshot{
			ContentHash: obs.SumBytes(body), Bytes: body, ByteSize: int64(len(body)), MediaType: "application/xml",
		}
		if err := ensureSnapshot(ctx, pool, snap); err != nil {
			t.Fatalf("ensure snapshot %s: %v", l.lawID, err)
		}
		ch := snap.ContentHash
		if err := rawInsert(ctx, pool, obs.EventFacts{
			EventID: uuidN(l.n), StreamID: streamID, StreamSeq: 1,
			Type: obs.ResourceObserved, Source: "egov-law", FetcherVersion: "itest/0.1",
			ObservedAt: baseObserved, ContentHash: &ch, LogPrevHash: headOf(t, pool),
		}); err != nil {
			t.Fatalf("insert event %s: %v", l.lawID, err)
		}
		var seq int64
		if err := pool.QueryRow(ctx,
			`SELECT seq FROM observation.event WHERE event_id = $1`, uuidN(l.n)).Scan(&seq); err != nil {
			t.Fatalf("read seq %s: %v", l.lawID, err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO interpretation.legislative_work
				(law_id, stream_id, law_title, observation_seq, observed_at)
			VALUES ($1, $2, $3, $4, $5)`,
			l.lawID, streamID, l.title, seq, baseObserved); err != nil {
			t.Fatalf("seed legislative_work %s: %v", l.lawID, err)
		}
	}

	reader := NewQueryReader(pool)
	cases := []struct {
		keyword string
		want    int
	}{
		{"100%", 1}, // literal "100%" only — NOT a "100<anything>" wildcard
		{"100_", 0}, // literal "100_" nowhere — NOT a one-rune wildcard over "1000"/"100%"
		{`100\`, 0}, // a lone backslash must not break the pattern (22025) or match
		{"100", 2},  // sanity: plain substring still matches both
	}
	for _, tc := range cases {
		f := port.TimelineFilter{Keyword: tc.keyword, Limit: 50}
		items, err := reader.ListTimeline(ctx, f)
		if err != nil {
			t.Fatalf("ListTimeline(keyword=%q): %v", tc.keyword, err)
		}
		if len(items) != tc.want {
			t.Errorf("ListTimeline(keyword=%q) rows = %d, want %d", tc.keyword, len(items), tc.want)
		}
		total, _, err := reader.CountTimeline(ctx, f, 0)
		if err != nil {
			t.Fatalf("CountTimeline(keyword=%q): %v", tc.keyword, err)
		}
		if total != tc.want {
			t.Errorf("CountTimeline(keyword=%q) total = %d, want %d", tc.keyword, total, tc.want)
		}
	}
}
