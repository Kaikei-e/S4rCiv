package signer

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/mod/sumdb/note"

	obs "s4rciv.org/api/internal/domain/observation"
)

func TestLoadErrorsWhenEnvUnset(t *testing.T) {
	t.Setenv(KeyFileEnv, "")
	if _, err := Load(); err == nil {
		t.Fatal("Load must error when the key-file env is unset")
	}
}

func TestLoadSignsAndVerifies(t *testing.T) {
	// Generate a key, write the private skey to a mounted-secret-style file, and load it.
	skey, vkey, err := Generate("s4rciv-checkpoint")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "checkpoint.key")
	if err := os.WriteFile(path, []byte(skey+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(KeyFileEnv, path)

	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	var root obs.Digest
	for i := range root {
		root[i] = byte(i)
	}
	cp := obs.NewLinkedCheckpoint(8821, root)
	signed, keyID, err := s.SignCheckpoint(cp)
	if err != nil {
		t.Fatal(err)
	}
	if keyID != "s4rciv-checkpoint" {
		t.Fatalf("signer_key_id = %q, want the key name", keyID)
	}

	// The published verifier key (vkey) opens the signed note we produced.
	verifier, err := note.NewVerifier(vkey)
	if err != nil {
		t.Fatal(err)
	}
	n, err := note.Open(signed, note.VerifierList(verifier))
	if err != nil {
		t.Fatalf("published key must verify the checkpoint: %v", err)
	}
	if n.Text != cp.NoteText() {
		t.Fatalf("verified text mismatch:\n got %q\nwant %q", n.Text, cp.NoteText())
	}
}
