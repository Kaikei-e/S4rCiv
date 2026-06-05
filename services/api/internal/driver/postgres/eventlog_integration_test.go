//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	obs "s4rciv.org/api/internal/domain/observation"
	"s4rciv.org/api/internal/port"
)

// These tests assert the observation-plane invariants that are enforced by DB
// triggers/constraints (ADR-000001) and therefore CANNOT be verified with a fake:
// the append trigger assigns a global seq and rejects a broken log chain; the
// mutation guard makes stream/snapshot/event/checkpoint append-only; uniqueness
// and the vanished-content CHECK hold. Each test runs on its own cloned database.

var baseObserved = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func uuidN(n int) string { return fmt.Sprintf("00000000-0000-7000-8000-%012d", n) }

func snapshotOf(b []byte) *port.Snapshot {
	d := obs.SumBytes(b)
	return &port.Snapshot{ContentHash: d, Bytes: b, ByteSize: int64(len(b)), MediaType: "application/json"}
}

func testStream(suffix string) port.Stream {
	id := "kokkai:TEST" + suffix
	return port.Stream{StreamID: id, Source: "kokkai", SourceLocalKey: "TEST" + suffix}
}

const insertEventSQL = `
INSERT INTO observation.event
  (event_id, stream_id, stream_seq, type, source, fetcher_version,
   observed_at, source_published_at, content_hash, prev_content_hash,
   log_prev_hash, log_hash)
VALUES ($1,$2,$3,$4::observation.event_type,$5,$6,$7,$8,$9,$10,$11,$12)`

// rawInsert bypasses EventLog and inserts an event whose log_hash is the correct
// canonical hash of its facts — so any failure is attributable to the specific
// guard under test, not to an incidental hash/chain mismatch.
func rawInsert(ctx context.Context, pool *pgxpool.Pool, f obs.EventFacts) error {
	lh, err := f.LogHash()
	if err != nil {
		return err
	}
	var pub *time.Time = f.SourcePublishedAt
	_, err = pool.Exec(ctx, insertEventSQL,
		f.EventID, f.StreamID, f.StreamSeq, f.Type.DBValue(), f.Source, f.FetcherVersion,
		f.ObservedAt, pub, digestBytes(f.ContentHash), digestBytes(f.PrevContentHash),
		f.LogPrevHash.Bytes(), lh.Bytes())
	return err
}

func ensureSnapshot(ctx context.Context, pool *pgxpool.Pool, s *port.Snapshot) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO observation.snapshot (content_hash, bytes, byte_size, media_type, was_ocr)
		 VALUES ($1,$2,$3,$4,false) ON CONFLICT (content_hash) DO NOTHING`,
		s.ContentHash.Bytes(), s.Bytes, s.ByteSize, s.MediaType)
	return err
}

func headOf(t *testing.T, pool *pgxpool.Pool) obs.Digest {
	t.Helper()
	var raw []byte
	if err := pool.QueryRow(context.Background(),
		`SELECT log_hash FROM observation.chain_head WHERE id = 1`).Scan(&raw); err != nil {
		t.Fatalf("read chain head: %v", err)
	}
	d, ok := obs.DigestFromBytes(raw)
	if !ok {
		t.Fatal("chain head log_hash is not 32 bytes")
	}
	return d
}

func wantPgMessage(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error containing %q, got nil", substr)
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) && strings.Contains(pg.Message, substr) {
		return
	}
	if strings.Contains(err.Error(), substr) {
		return
	}
	t.Fatalf("error %v does not contain %q", err, substr)
}

func wantPgCode(t *testing.T, err error, code string) *pgconn.PgError {
	t.Helper()
	var pg *pgconn.PgError
	if !errors.As(err, &pg) {
		t.Fatalf("expected *pgconn.PgError, got %v", err)
	}
	if pg.Code != code {
		t.Fatalf("pg SQLSTATE = %s, want %s (message: %s)", pg.Code, code, pg.Message)
	}
	return pg
}

func TestObservationLog_AssignsGlobalSeqAndChainsLog(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := newTestDB(t)
	log := NewEventLog(pool)
	s := testStream("STREAMA0000000000001")
	if err := log.EnsureStream(ctx, s); err != nil {
		t.Fatal(err)
	}

	a := snapshotOf([]byte(`{"v":"a"}`))
	b := snapshotOf([]byte(`{"v":"b"}`))

	seq1, err := log.Append(ctx, port.AppendCmd{
		Stream: s, Type: obs.ResourceObserved, EventID: uuidN(1), Source: "kokkai",
		FetcherVersion: "itest/0.1", ObservedAt: baseObserved, Snapshot: a,
	})
	if err != nil {
		t.Fatal(err)
	}
	if seq1 != 1 {
		t.Fatalf("genesis append seq = %d, want 1 (the trigger assigns the global seq)", seq1)
	}

	seq2, err := log.Append(ctx, port.AppendCmd{
		Stream: s, Type: obs.ResourceChanged, EventID: uuidN(2), Source: "kokkai",
		FetcherVersion: "itest/0.1", ObservedAt: baseObserved.Add(time.Minute),
		Snapshot: b, PrevContentHash: &a.ContentHash,
	})
	if err != nil {
		t.Fatal(err)
	}
	if seq2 != 2 {
		t.Fatalf("second append seq = %d, want 2", seq2)
	}

	// Log chain: row1 links to genesis zero; row2 links to row1; head tracks row2.
	type row struct {
		seq                int64
		logPrev, logHashed []byte
	}
	rows, err := pool.Query(ctx,
		`SELECT seq, log_prev_hash, log_hash FROM observation.event ORDER BY seq`)
	if err != nil {
		t.Fatal(err)
	}
	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.seq, &r.logPrev, &r.logHashed); err != nil {
			t.Fatal(err)
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("event count = %d, want 2", len(got))
	}
	var zero obs.Digest
	if !bytesEqual(got[0].logPrev, zero.Bytes()) {
		t.Errorf("first event log_prev_hash = %x, want genesis zero", got[0].logPrev)
	}
	if !bytesEqual(got[1].logPrev, got[0].logHashed) {
		t.Errorf("log chain broken: event2.log_prev_hash %x != event1.log_hash %x",
			got[1].logPrev, got[0].logHashed)
	}

	// chain_head advanced to the last event.
	headSeq, headHash := chainHead(t, pool)
	if headSeq != 2 || !bytesEqual(headHash, got[1].logHashed) {
		t.Errorf("chain head = (seq %d, %x), want (2, %x)", headSeq, headHash, got[1].logHashed)
	}

	// The stored log_hash equals the canonical hash recomputed in Go (the bytes a
	// third-party verifier would re-derive) — proving persistence didn't alter it.
	wantFacts := obs.EventFacts{
		EventID: uuidN(1), StreamID: s.StreamID, StreamSeq: 1, Type: obs.ResourceObserved,
		Source: "kokkai", FetcherVersion: "itest/0.1", ObservedAt: baseObserved,
		ContentHash: &a.ContentHash, LogPrevHash: zero,
	}
	want, err := wantFacts.LogHash()
	if err != nil {
		t.Fatal(err)
	}
	if !bytesEqual(got[0].logHashed, want.Bytes()) {
		t.Errorf("stored log_hash %x != canonical recompute %x", got[0].logHashed, want.Bytes())
	}
}

func TestObservationLog_RejectsBrokenLogChain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := newTestDB(t)
	s := testStream("BROKENCHAIN000000001")
	if err := NewEventLog(pool).EnsureStream(ctx, s); err != nil {
		t.Fatal(err)
	}

	// Head is genesis zero; supply a non-matching log_prev_hash. ResourceVanished
	// carries no content, so this isolates the chain check from the FK/CHECK paths.
	wrong := obs.SumBytes([]byte("not the chain head"))
	err := rawInsert(ctx, pool, obs.EventFacts{
		EventID: uuidN(9), StreamID: s.StreamID, StreamSeq: 1, Type: obs.ResourceVanished,
		Source: "kokkai", FetcherVersion: "itest/0.1", ObservedAt: baseObserved, LogPrevHash: wrong,
	})
	wantPgMessage(t, err, "broken observation log chain")

	// The rejected insert must not have advanced the head.
	if seq, _ := chainHead(t, pool); seq != 0 {
		t.Errorf("chain head advanced to %d after a rejected append; want 0", seq)
	}
}

func TestObservationEvent_IsAppendOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := newTestDB(t)
	log := NewEventLog(pool)
	s := testStream("APPENDONLY0000000001")
	if err := log.EnsureStream(ctx, s); err != nil {
		t.Fatal(err)
	}
	if _, err := log.Append(ctx, port.AppendCmd{
		Stream: s, Type: obs.ResourceObserved, EventID: uuidN(1), Source: "kokkai",
		FetcherVersion: "itest/0.1", ObservedAt: baseObserved, Snapshot: snapshotOf([]byte("x")),
	}); err != nil {
		t.Fatal(err)
	}

	_, err := pool.Exec(ctx, `UPDATE observation.event SET source = 'tampered' WHERE seq = 1`)
	wantPgMessage(t, err, "append-only")

	_, err = pool.Exec(ctx, `DELETE FROM observation.event WHERE seq = 1`)
	wantPgMessage(t, err, "append-only")
}

func TestObservationTables_AreAppendOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := newTestDB(t)
	log := NewEventLog(pool)
	s := testStream("TABLESGUARD000000001")
	if err := log.EnsureStream(ctx, s); err != nil {
		t.Fatal(err)
	}
	if _, err := log.Append(ctx, port.AppendCmd{
		Stream: s, Type: obs.ResourceObserved, EventID: uuidN(1), Source: "kokkai",
		FetcherVersion: "itest/0.1", ObservedAt: baseObserved, Snapshot: snapshotOf([]byte("y")),
	}); err != nil {
		t.Fatal(err)
	}
	// A checkpoint row (INSERT is allowed; mutation is not).
	if _, err := pool.Exec(ctx,
		`INSERT INTO observation.checkpoint (checkpoint_id, through_seq, tree_size, root_hash, alg_version)
		 VALUES ($1, 1, 1, $2, $3)`,
		uuidN(100), obs.SumBytes([]byte("root")).Bytes(), obs.AlgVersion); err != nil {
		t.Fatalf("insert checkpoint: %v", err)
	}

	for _, tc := range []struct{ name, sql string }{
		{"stream update", `UPDATE observation.stream SET source = 'x' WHERE stream_id = $1`},
		{"snapshot update", `UPDATE observation.snapshot SET was_ocr = true`},
		{"checkpoint update", `UPDATE observation.checkpoint SET tree_size = 99`},
		{"checkpoint delete", `DELETE FROM observation.checkpoint`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			if strings.Contains(tc.sql, "$1") {
				_, err = pool.Exec(ctx, tc.sql, s.StreamID)
			} else {
				_, err = pool.Exec(ctx, tc.sql)
			}
			wantPgMessage(t, err, "append-only")
		})
	}
}

func TestObservationEvent_StreamSeqUnique(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := newTestDB(t)
	log := NewEventLog(pool)
	s := testStream("STREAMSEQUNIQUE00001")
	if err := log.EnsureStream(ctx, s); err != nil {
		t.Fatal(err)
	}
	if _, err := log.Append(ctx, port.AppendCmd{
		Stream: s, Type: obs.ResourceObserved, EventID: uuidN(1), Source: "kokkai",
		FetcherVersion: "itest/0.1", ObservedAt: baseObserved, Snapshot: snapshotOf([]byte("a")),
	}); err != nil {
		t.Fatal(err)
	}

	// A second event reusing stream_seq=1, with a VALID log chain (so only the
	// (stream_id, stream_seq) uniqueness can fail).
	a, b := snapshotOf([]byte("a")), snapshotOf([]byte("b"))
	if err := ensureSnapshot(ctx, pool, b); err != nil {
		t.Fatal(err)
	}
	err := rawInsert(ctx, pool, obs.EventFacts{
		EventID: uuidN(2), StreamID: s.StreamID, StreamSeq: 1, Type: obs.ResourceChanged,
		Source: "kokkai", FetcherVersion: "itest/0.1", ObservedAt: baseObserved.Add(time.Minute),
		ContentHash: &b.ContentHash, PrevContentHash: &a.ContentHash,
		LogPrevHash: headOf(t, pool),
	})
	pg := wantPgCode(t, err, "23505") // unique_violation
	if pg.ConstraintName != "event_stream_seq_unique" {
		t.Errorf("constraint = %q, want event_stream_seq_unique", pg.ConstraintName)
	}
}

func TestObservationEvent_VanishedContentCheck(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := newTestDB(t)
	s := testStream("VANISHEDCHECK0000001")
	if err := NewEventLog(pool).EnsureStream(ctx, s); err != nil {
		t.Fatal(err)
	}

	t.Run("vanished must not carry content", func(t *testing.T) {
		snap := snapshotOf([]byte("present"))
		if err := ensureSnapshot(ctx, pool, snap); err != nil {
			t.Fatal(err)
		}
		err := rawInsert(ctx, pool, obs.EventFacts{
			EventID: uuidN(3), StreamID: s.StreamID, StreamSeq: 1, Type: obs.ResourceVanished,
			Source: "kokkai", FetcherVersion: "itest/0.1", ObservedAt: baseObserved,
			ContentHash: &snap.ContentHash, LogPrevHash: headOf(t, pool),
		})
		wantPgCode(t, err, "23514") // check_violation
	})

	t.Run("non-vanished must carry content", func(t *testing.T) {
		err := rawInsert(ctx, pool, obs.EventFacts{
			EventID: uuidN(4), StreamID: s.StreamID, StreamSeq: 1, Type: obs.ResourceObserved,
			Source: "kokkai", FetcherVersion: "itest/0.1", ObservedAt: baseObserved,
			ContentHash: nil, LogPrevHash: headOf(t, pool),
		})
		wantPgCode(t, err, "23514")
	})
}

func TestObservationLog_ContentChainContinuity(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := newTestDB(t)
	log := NewEventLog(pool)
	s := testStream("CONTENTCHAIN0000001")
	if err := log.EnsureStream(ctx, s); err != nil {
		t.Fatal(err)
	}

	a, b, c := snapshotOf([]byte("a")), snapshotOf([]byte("b")), snapshotOf([]byte("c"))
	steps := []struct {
		typ  obs.EventType
		snap *port.Snapshot
		prev *obs.Digest
	}{
		{obs.ResourceObserved, a, nil},
		{obs.ResourceChanged, b, &a.ContentHash},
		{obs.ResourceChanged, c, &b.ContentHash},
	}
	for i, st := range steps {
		if _, err := log.Append(ctx, port.AppendCmd{
			Stream: s, Type: st.typ, EventID: uuidN(i + 1), Source: "kokkai",
			FetcherVersion: "itest/0.1", ObservedAt: baseObserved.Add(time.Duration(i) * time.Minute),
			Snapshot: st.snap, PrevContentHash: st.prev,
		}); err != nil {
			t.Fatalf("append step %d: %v", i, err)
		}
	}

	rows, err := pool.Query(ctx,
		`SELECT content_hash, prev_content_hash FROM observation.event
		 WHERE stream_id = $1 ORDER BY stream_seq`, s.StreamID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var prevContent []byte
	idx := 0
	for rows.Next() {
		var content, prev []byte
		if err := rows.Scan(&content, &prev); err != nil {
			t.Fatal(err)
		}
		if idx == 0 {
			if prev != nil {
				t.Errorf("first event prev_content_hash = %x, want NULL", prev)
			}
		} else if !bytesEqual(prev, prevContent) {
			t.Errorf("event %d prev_content_hash %x != prior content_hash %x", idx, prev, prevContent)
		}
		prevContent = content
		idx++
	}
	if idx != 3 {
		t.Fatalf("event count = %d, want 3", idx)
	}
}

func chainHead(t *testing.T, pool *pgxpool.Pool) (int64, []byte) {
	t.Helper()
	var seq int64
	var hash []byte
	if err := pool.QueryRow(context.Background(),
		`SELECT seq, log_hash FROM observation.chain_head WHERE id = 1`).Scan(&seq, &hash); err != nil {
		t.Fatalf("read chain head: %v", err)
	}
	return seq, hash
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
