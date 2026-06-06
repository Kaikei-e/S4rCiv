//go:build integration

package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/mod/sumdb/note"

	obs "s4rciv.org/api/internal/domain/observation"
	"s4rciv.org/api/internal/driver/signer"
	"s4rciv.org/api/internal/port"
	"s4rciv.org/api/internal/usecase/checkpoint"
)

// seqID is a deterministic IDGenerator for the checkpoint usecase.
type seqID struct{ n int }

func (s *seqID) NewID() string { s.n++; return uuidN(900 + s.n) }

// observe appends a genesis ResourceObserved on its own stream (so each advances the
// global chain head without any content-chain coupling).
func observe(t *testing.T, log *EventLog, n int, body string) {
	t.Helper()
	s := testStream(fmt.Sprintf("CHECKPOINT%010d", n))
	if err := log.EnsureStream(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if _, err := log.Append(context.Background(), port.AppendCmd{
		Stream: s, Type: obs.ResourceObserved, EventID: uuidN(n), Source: "kokkai",
		FetcherVersion: "itest", ObservedAt: baseObserved, Snapshot: snapshotOf([]byte(body)),
	}); err != nil {
		t.Fatalf("append %d: %v", n, err)
	}
}

func TestCheckpointGenerator_SignsHead_IsIdempotent_Advances(t *testing.T) {
	// No t.Parallel(): this test uses t.Setenv to point the signer at a temp key file,
	// which Go forbids alongside parallel tests. It still runs on its own cloned DB.
	ctx := context.Background()
	pool := newTestDB(t)
	log := NewEventLog(pool)
	store := NewCheckpointStore(pool)

	observe(t, log, 1, "a")
	observe(t, log, 2, "b") // chain_head.seq == 2

	// ChainHead must match the stored chain head.
	seq, head, err := store.ChainHead(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if seq != 2 {
		t.Fatalf("ChainHead seq = %d, want 2", seq)
	}
	if head != headOf(t, pool) {
		t.Fatal("ChainHead log_hash must equal chain_head.log_hash")
	}

	// Load a real signer from a generated key written to a mounted-secret-style file.
	skey, vkey, err := signer.Generate("s4rciv-itest")
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(t.TempDir(), "checkpoint.key")
	if err := os.WriteFile(keyPath, []byte(skey), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(signer.KeyFileEnv, keyPath)
	sg, err := signer.Load()
	if err != nil {
		t.Fatal(err)
	}

	uc := checkpoint.New(store, sg, &seqID{})

	// First run signs a checkpoint through seq 2.
	created, through, err := uc.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !created || through != 2 {
		t.Fatalf("first run created=%v through=%d, want true/2", created, through)
	}

	// The stored signed note verifies under the published key and matches the canonical body.
	var stored []byte
	if err := pool.QueryRow(ctx,
		`SELECT signature FROM observation.checkpoint WHERE through_seq = 2`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	verifier, err := note.NewVerifier(vkey)
	if err != nil {
		t.Fatal(err)
	}
	n, err := note.Open(stored, note.VerifierList(verifier))
	if err != nil {
		t.Fatalf("stored checkpoint must verify under the published key: %v", err)
	}
	if want := obs.NewLinkedCheckpoint(2, head).NoteText(); n.Text != want {
		t.Fatalf("stored note text mismatch:\n got %q\nwant %q", n.Text, want)
	}

	// Re-running over the same head is a no-op (idempotent).
	created2, _, err := uc.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if created2 {
		t.Fatal("re-running over an already-covered head must not create a checkpoint")
	}
	if last, _ := store.LatestThroughSeq(ctx); last != 2 {
		t.Fatalf("LatestThroughSeq = %d, want 2", last)
	}

	// A new observation advances the head; the next run checkpoints seq 3.
	observe(t, log, 3, "c")
	created3, through3, err := uc.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !created3 || through3 != 3 {
		t.Fatalf("third run created=%v through=%d, want true/3", created3, through3)
	}
}

func TestMastheadStatus_CountsEnabledWatches_AndLatestCheckpoint(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := newTestDB(t)
	qr := NewQueryReader(pool)
	store := NewCheckpointStore(pool)

	// Empty: zero coverage, no checkpoint.
	if w, _, has, err := qr.MastheadStatus(ctx); err != nil || w != 0 || has {
		t.Fatalf("empty MastheadStatus = (%d, has=%v, err=%v), want (0, false, nil)", w, has, err)
	}

	// Coverage counts ENABLED watches only.
	for i, enabled := range []bool{true, true, false} {
		if _, err := pool.Exec(ctx,
			`INSERT INTO control.watch (stream_id, source, source_local_key, canonical_url, enabled)
			 VALUES ($1, 'kokkai', $2, 'https://example.test', $3)`,
			fmt.Sprintf("kokkai:W%d", i), fmt.Sprintf("W%d", i), enabled); err != nil {
			t.Fatal(err)
		}
	}
	if w, _, _, err := qr.MastheadStatus(ctx); err != nil || w != 2 {
		t.Fatalf("watch coverage = %d (err %v), want 2 (enabled only)", w, err)
	}

	// Latest checkpoint is the highest through_seq, reported as signed when a signature
	// is present.
	for _, seq := range []int64{1, 2} {
		if err := store.AppendCheckpoint(ctx, port.CheckpointRecord{
			CheckpointID: uuidN(50 + int(seq)), ThroughSeq: seq, TreeSize: seq,
			RootHash: obs.SumBytes([]byte(fmt.Sprintf("root-%d", seq))),
			AlgVersion: obs.AlgVersion, SignedNote: []byte("signed-note"), SignerKeyID: "s4rciv-itest",
		}); err != nil {
			t.Fatal(err)
		}
	}
	_, cp, has, err := qr.MastheadStatus(ctx)
	if err != nil || !has {
		t.Fatalf("expected a checkpoint, got has=%v err=%v", has, err)
	}
	if cp.ThroughSeq != 2 || !cp.Signed || cp.SignerKeyID != "s4rciv-itest" {
		t.Fatalf("latest checkpoint = %+v, want through_seq 2, signed, key s4rciv-itest", cp)
	}

	// The public feed returns the signed notes, newest first.
	feed, err := qr.ListCheckpoints(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(feed) != 2 || feed[0].ThroughSeq != 2 || feed[1].ThroughSeq != 1 {
		t.Fatalf("feed = %+v, want through_seq [2, 1] (newest first)", feed)
	}
	if string(feed[0].SignedNote) != "signed-note" || feed[0].SignerKeyID != "s4rciv-itest" {
		t.Fatalf("feed[0] must carry the stored signed note + key: %+v", feed[0])
	}
	if feed[0].RootHash == "" || len(feed[0].RootHash) != 64 {
		t.Fatalf("feed[0].RootHash should be 64 hex chars, got %q", feed[0].RootHash)
	}
}
