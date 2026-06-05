//go:build e2eseed

// Package e2eseed deterministically seeds the E2E database with a small, fixed
// fixture set so the browser suite asserts against a KNOWN state (Playwright best
// practice: never depend on "whatever is in the DB"). It is a test (not a cmd) so
// it reuses the production append path — observation events go through the real
// EventLog, so content_hash/log_hash are computed by the same code the collector
// uses and the chain is byte-reproducible across runs.
//
// Determinism rules (the hashed inputs are frozen): fixed UUIDs, fixed UTC-second
// timestamps, no time.Now(), no random. seq is assigned by the DB trigger; we reset
// observation.chain_head to genesis first so a re-seed replays to the same hashes.
//
// Run via the compose `seed` service: go test -tags=e2eseed -run ^TestSeedE2E$ ...
// Names are fictional placeholders only — never real Diet members.
package e2eseed

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	obs "s4rciv.org/api/internal/domain/observation"
	"s4rciv.org/api/internal/driver/postgres"
	"s4rciv.org/api/internal/port"
)

var (
	tMeeting    = time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC)
	tLawObserved = time.Date(2026, 3, 3, 9, 0, 0, 0, time.UTC)
	tLawChanged  = time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
)

func TestSeedE2E(t *testing.T) {
	ctx := context.Background()
	pool, err := postgres.Connect(ctx)
	if err != nil {
		t.Fatalf("connect to E2E database: %v", err)
	}
	defer pool.Close()

	if err := resetToGenesis(ctx, pool); err != nil {
		t.Fatalf("reset to genesis: %v", err)
	}
	log := postgres.NewEventLog(pool)

	// ── Cross-source timeline item 1: a 国会 meeting observed (kokkai) ──────────
	meeting := port.Stream{
		StreamID: "kokkai:100000000X00120260101", Source: "kokkai",
		SourceLocalKey: "100000000X00120260101",
		CanonicalURL:   "https://kokkai.ndl.go.jp/#/detail?minId=100000000X00120260101",
	}
	if err := log.EnsureStream(ctx, meeting); err != nil {
		t.Fatal(err)
	}
	meetingSnap := snapshot([]byte(`{"meeting":"予算委員会","issue":"第1号"}`))
	seqMeeting, err := log.Append(ctx, port.AppendCmd{
		Stream: meeting, Type: obs.ResourceObserved, EventID: "00000000-0000-7000-8000-0000000e2e01",
		Source: "kokkai", FetcherVersion: "kokkai-collector/0.1-e2e", ObservedAt: tMeeting,
		SourcePublishedAt: ptr(tMeeting), Snapshot: meetingSnap,
	})
	if err != nil {
		t.Fatalf("append meeting event: %v", err)
	}
	mustExec(t, ctx, pool,
		`INSERT INTO interpretation.meeting
		   (issue_id, stream_id, session, house, meeting_name, issue, meeting_date,
		    permalink, was_ocr, observation_seq, observed_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,false,$9,$10)`,
		"100000000X00120260101", meeting.StreamID, 213, "衆議院", "予算委員会", "第1号",
		tMeeting, meeting.CanonicalURL, seqMeeting, tMeeting)

	// ── Cross-source timeline items 2 & 3: a 法令 observed then changed (egov) ──
	law := port.Stream{
		StreamID: "egov-law:999AC0000000999", Source: "egov-law",
		SourceLocalKey: "999AC0000000999",
		CanonicalURL:   "https://laws.e-gov.go.jp/law/999AC0000000999",
	}
	if err := log.EnsureStream(ctx, law); err != nil {
		t.Fatal(err)
	}
	lawV1 := snapshot([]byte(`<Law><Article Num="9"><Paragraph Num="1">旧</Paragraph></Article></Law>`))
	lawV2 := snapshot([]byte(`<Law><Article Num="9"><Paragraph Num="1">新</Paragraph><Paragraph Num="2">追加</Paragraph></Article></Law>`))

	if _, err := log.Append(ctx, port.AppendCmd{
		Stream: law, Type: obs.ResourceObserved, EventID: "00000000-0000-7000-8000-0000000e2e02",
		Source: "egov-law", FetcherVersion: "egov-collector/0.1-e2e", ObservedAt: tLawObserved,
		Snapshot: lawV1,
	}); err != nil {
		t.Fatalf("append law observed: %v", err)
	}
	seqLawChanged, err := log.Append(ctx, port.AppendCmd{
		Stream: law, Type: obs.ResourceChanged, EventID: "00000000-0000-7000-8000-0000000e2e03",
		Source: "egov-law", FetcherVersion: "egov-collector/0.1-e2e", ObservedAt: tLawChanged,
		Snapshot: lawV2, PrevContentHash: &lawV1.ContentHash,
	})
	if err != nil {
		t.Fatalf("append law changed: %v", err)
	}
	mustExec(t, ctx, pool,
		`INSERT INTO interpretation.legislative_work
		   (law_id, stream_id, law_num, law_type, law_title, promulgation_date,
		    current_revision_status, repeal_status, permalink, was_ocr, observation_seq, observed_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,false,$10,$11)`,
		"999AC0000000999", law.StreamID, "令和八年法律第九百九十九号", "Act",
		"テスト民生安定法", time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		"CurrentEnforced", "None", law.CanonicalURL, seqLawChanged, tLawChanged)

	// The interpretation.change projection that the timeline folds into node counts
	// + classification. One added paragraph + one modified → substantive.
	mustExec(t, ctx, pool,
		`INSERT INTO interpretation.change
		   (observation_seq, differ_version, diff, classification, class_confidence)
		 VALUES ($1,$2,$3::jsonb,$4,$5)`,
		seqLawChanged, "differ/0.1-e2e",
		`{"node_changes":[
		   {"eid":"art_9__para_1","op":"modified","node_type":"paragraph","prev_text":"旧","curr_text":"新"},
		   {"eid":"art_9__para_2","op":"added","node_type":"paragraph","curr_text":"追加"}
		 ]}`,
		"substantive", "high")

	t.Logf("seeded E2E fixtures: meeting seq=%d, law-changed seq=%d", seqMeeting, seqLawChanged)
}

func resetToGenesis(ctx context.Context, pool *pgxpool.Pool) error {
	// TRUNCATE is not blocked by the append-only row trigger (UPDATE/DELETE only),
	// and CASCADE clears every dependent read model. chain_head is mutable by design.
	if _, err := pool.Exec(ctx, `
		TRUNCATE observation.stream, observation.snapshot, observation.event,
		         observation.checkpoint, interpretation.event
		RESTART IDENTITY CASCADE`); err != nil {
		return err
	}
	zero := make([]byte, 32)
	if _, err := pool.Exec(ctx,
		`UPDATE observation.chain_head SET seq = 0, log_hash = $1 WHERE id = 1`, zero); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx,
		`UPDATE interpretation.chain_head SET seq = 0, log_hash = $1 WHERE id = 1`, zero); err != nil {
		return err
	}
	_, err := pool.Exec(ctx, `UPDATE interpretation.projector_offset SET last_seq = 0, rebuilding = false`)
	return err
}

func snapshot(b []byte) *port.Snapshot {
	d := obs.SumBytes(b)
	return &port.Snapshot{ContentHash: d, Bytes: b, ByteSize: int64(len(b)), MediaType: "application/xml"}
}

func ptr[T any](v T) *T { return &v }

func mustExec(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql, args...); err != nil {
		t.Fatalf("seed exec failed: %v\nSQL: %s", err, sql)
	}
}
