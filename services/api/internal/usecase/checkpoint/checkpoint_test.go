package checkpoint

import (
	"context"
	"errors"
	"testing"

	obs "s4rciv.org/api/internal/domain/observation"
	"s4rciv.org/api/internal/port"
)

type fakeStore struct {
	seq       int64
	head      obs.Digest
	latest    int64
	appended  []port.CheckpointRecord
	appendErr error
}

func (f *fakeStore) ChainHead(context.Context) (int64, obs.Digest, error) {
	return f.seq, f.head, nil
}
func (f *fakeStore) LatestThroughSeq(context.Context) (int64, error) { return f.latest, nil }
func (f *fakeStore) AppendCheckpoint(_ context.Context, rec port.CheckpointRecord) error {
	if f.appendErr != nil {
		return f.appendErr
	}
	f.appended = append(f.appended, rec)
	return nil
}

type fakeSigner struct {
	keyID string
	err   error
}

func (s fakeSigner) SignCheckpoint(c obs.Checkpoint) ([]byte, string, error) {
	if s.err != nil {
		return nil, "", s.err
	}
	return []byte("signed-note:" + c.NoteText()), s.keyID, nil
}

type fakeIDs struct{ id string }

func (f fakeIDs) NewID() string { return f.id }

func head42() obs.Digest {
	var d obs.Digest
	for i := range d {
		d[i] = 42
	}
	return d
}

func TestRunCreatesCheckpointOverNewHead(t *testing.T) {
	store := &fakeStore{seq: 10, head: head42(), latest: 5}
	u := New(store, fakeSigner{keyID: "s4rciv-checkpoint"}, fakeIDs{id: "cp-uuid-1"})

	created, seq, err := u.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !created || seq != 10 {
		t.Fatalf("created=%v seq=%d, want true/10", created, seq)
	}
	if len(store.appended) != 1 {
		t.Fatalf("want 1 appended checkpoint, got %d", len(store.appended))
	}
	rec := store.appended[0]
	if rec.CheckpointID != "cp-uuid-1" || rec.ThroughSeq != 10 || rec.TreeSize != 10 {
		t.Fatalf("bad record id/seq/size: %+v", rec)
	}
	if rec.RootHash != head42() {
		t.Fatal("root hash must be the chain head")
	}
	if rec.AlgVersion != obs.AlgVersion || rec.SignerKeyID != "s4rciv-checkpoint" {
		t.Fatalf("bad alg/key: %q/%q", rec.AlgVersion, rec.SignerKeyID)
	}
	if len(rec.SignedNote) == 0 {
		t.Fatal("signed note must be stored")
	}
}

func TestRunIsIdempotentWhenHeadAlreadyCovered(t *testing.T) {
	store := &fakeStore{seq: 5, head: head42(), latest: 5}
	u := New(store, fakeSigner{keyID: "k"}, fakeIDs{id: "x"})

	created, seq, err := u.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if created || seq != 5 {
		t.Fatalf("created=%v seq=%d, want false/5", created, seq)
	}
	if len(store.appended) != 0 {
		t.Fatal("an already-covered head must not append a checkpoint")
	}
}

func TestRunDoesNothingAtGenesis(t *testing.T) {
	store := &fakeStore{seq: 0, latest: 0}
	u := New(store, fakeSigner{keyID: "k"}, fakeIDs{id: "x"})

	created, _, err := u.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if created || len(store.appended) != 0 {
		t.Fatal("genesis (nothing observed) must not produce a checkpoint")
	}
}

func TestRunPropagatesSignerError(t *testing.T) {
	store := &fakeStore{seq: 10, head: head42(), latest: 0}
	u := New(store, fakeSigner{err: errors.New("kaboom")}, fakeIDs{id: "x"})

	if _, _, err := u.Run(context.Background()); err == nil {
		t.Fatal("a signer error must abort without appending")
	}
	if len(store.appended) != 0 {
		t.Fatal("nothing must be appended when signing fails")
	}
}
