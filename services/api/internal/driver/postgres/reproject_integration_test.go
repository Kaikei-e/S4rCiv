//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	obs "s4rciv.org/api/internal/domain/observation"
	"s4rciv.org/api/internal/gateway/kokkai"
	"s4rciv.org/api/internal/port"
	"s4rciv.org/api/internal/usecase/project"
)

// Reproject-safety is the core immutable-design-guard invariant for disposable
// read models (ADR-000002): a read model can be TRUNCATEd and replayed from the
// observation log to a byte-identical result. This drives the REAL kokkai
// Projector against a real Postgres — Run → fingerprint A → Reproject (truncate +
// reset offset + replay) → fingerprint B → assert A == B and no duplication.

// A minimal but valid kokkai meeting snapshot (the bytes ParseMeeting consumes).
// Fictional names only — never real Diet members.
const meetingSnapshotJSON = `{
  "issueID": "100000000X00120260101",
  "session": 213,
  "nameOfHouse": "衆議院",
  "nameOfMeeting": "予算委員会",
  "issue": "第1号",
  "date": "2026-01-21",
  "meetingURL": "https://kokkai.ndl.go.jp/#/detail?minId=100000000X00120260101",
  "speechRecord": [
    {"speechID":"100000000X00120260101_000","speechOrder":0,"speaker":"会議録情報","speech":"本日の会議を開きます。"},
    {"speechID":"100000000X00120260101_001","speechOrder":1,"speaker":"山田太郎","speakerGroup":"テスト会派","speech":"質問いたします。"},
    {"speechID":"100000000X00120260101_002","speechOrder":2,"speaker":"鈴木花子","speakerGroup":"別会派","speech":"お答えします。"}
  ]
}`

type readModelFingerprint struct {
	meetings   int
	speeches   int
	meetingKey string // meeting_name|session|house|observation_seq
	speakerSeq string // ordered "order:speaker" join — catches reorder/dup
}

func TestProjector_ReprojectIsByteStable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := newTestDB(t)

	const issueID = "100000000X00120260101"
	stream := port.Stream{
		StreamID: "kokkai:" + issueID, Source: "kokkai", SourceLocalKey: issueID,
		CanonicalURL: "https://kokkai.ndl.go.jp/#/detail?minId=" + issueID,
	}
	log := NewEventLog(pool)
	if err := log.EnsureStream(ctx, stream); err != nil {
		t.Fatal(err)
	}
	snap := snapshotOf([]byte(meetingSnapshotJSON))
	evSeq, err := log.Append(ctx, port.AppendCmd{
		Stream: stream, Type: obs.ResourceObserved, EventID: uuidN(1), Source: "kokkai",
		FetcherVersion: "kokkai-collector/0.1-itest", ObservedAt: baseObserved, Snapshot: snap,
	})
	if err != nil {
		t.Fatalf("append meeting event: %v", err)
	}

	rm := NewReadModel(pool)
	// kokkai.New(nil): ParseMeeting reads only the snapshot bytes, never HTTP.
	projector := project.New(NewEventReader(pool), kokkai.New(nil), rm, rm, "kokkai-reproject-itest")

	// Initial projection.
	n1, err := projector.Run(ctx)
	if err != nil {
		t.Fatalf("initial Run: %v", err)
	}
	if n1 != 1 {
		t.Fatalf("initial Run projected %d meetings, want 1", n1)
	}
	fpA := fingerprint(t, pool, issueID)
	if fpA.meetings != 1 || fpA.speeches != 3 {
		t.Fatalf("after Run: meetings=%d speeches=%d, want 1/3", fpA.meetings, fpA.speeches)
	}
	if off := offsetOf(t, pool, "kokkai-reproject-itest"); off != evSeq {
		t.Errorf("offset = %d after Run, want event seq %d", off, evSeq)
	}

	// Reproject: BeginRebuild truncates the read models + resets the offset, then
	// replays from 0.
	n2, err := projector.Reproject(ctx)
	if err != nil {
		t.Fatalf("Reproject: %v", err)
	}
	if n2 != 1 {
		t.Fatalf("Reproject projected %d meetings, want 1 (replay)", n2)
	}
	fpB := fingerprint(t, pool, issueID)

	// The disposable projection rebuilt to a byte-identical state — and did not
	// duplicate rows (truncate worked; a missing truncate would double the counts).
	if fpA != fpB {
		t.Errorf("reproject not stable:\n  before %+v\n  after  %+v", fpA, fpB)
	}
}

func fingerprint(t *testing.T, pool *pgxpool.Pool, issueID string) readModelFingerprint {
	t.Helper()
	ctx := context.Background()
	var fp readModelFingerprint
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM interpretation.meeting WHERE issue_id = $1`, issueID).Scan(&fp.meetings); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM interpretation.speech WHERE issue_id = $1`, issueID).Scan(&fp.speeches); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT coalesce(meeting_name,'')||'|'||coalesce(session::text,'')||'|'||coalesce(house,'')||'|'||observation_seq::text
		 FROM interpretation.meeting WHERE issue_id = $1`, issueID).Scan(&fp.meetingKey); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT coalesce(string_agg(speech_order::text||':'||coalesce(speaker,''), ',' ORDER BY speech_order),'')
		 FROM interpretation.speech WHERE issue_id = $1`, issueID).Scan(&fp.speakerSeq); err != nil {
		t.Fatal(err)
	}
	return fp
}

func offsetOf(t *testing.T, pool *pgxpool.Pool, projector string) int64 {
	t.Helper()
	var seq int64
	if err := pool.QueryRow(context.Background(),
		`SELECT last_seq FROM interpretation.projector_offset WHERE projector = $1`, projector).Scan(&seq); err != nil {
		t.Fatalf("read offset for %s: %v", projector, err)
	}
	return seq
}
