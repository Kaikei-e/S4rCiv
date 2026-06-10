package signer

import (
	"os"
	"path/filepath"
	"strings"
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

// A signing key readable by group/other would let other accounts forge
// checkpoints, so Load must refuse it and tell the operator to chmod 600.
func TestLoadRejectsGroupOtherReadableKey(t *testing.T) {
	skey, _, err := Generate("s4rciv-checkpoint")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "checkpoint.key")
	if err := os.WriteFile(path, []byte(skey+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(KeyFileEnv, path)

	_, err = Load()
	if err == nil {
		t.Fatal("Load must reject a group/other-readable key file")
	}
	if !strings.Contains(err.Error(), "chmod 600") {
		t.Fatalf("error must tell the operator to chmod 600, got %v", err)
	}

	// 0400 (read-only owner) is as legitimate as 0600.
	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err != nil {
		t.Fatalf("Load must accept a 0400 key file: %v", err)
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
