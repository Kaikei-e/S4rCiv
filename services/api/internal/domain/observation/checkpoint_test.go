package observation

import (
	"crypto/rand"
	"strings"
	"testing"

	"golang.org/x/mod/sumdb/note"
)

func testHead() Digest {
	var d Digest
	for i := range d {
		d[i] = byte(i + 1)
	}
	return d
}

func TestNewLinkedCheckpoint(t *testing.T) {
	head := testHead()
	c := NewLinkedCheckpoint(8821, head)
	if c.ThroughSeq != 8821 || c.TreeSize != 8821 {
		t.Fatalf("seq/size = %d/%d, want 8821/8821", c.ThroughSeq, c.TreeSize)
	}
	if c.RootHash != head {
		t.Fatal("root hash must be the chain head")
	}
	if c.AlgVersion != AlgVersion {
		t.Fatalf("alg = %q, want %q", c.AlgVersion, AlgVersion)
	}
	// Origin must pin the linked variant so it is not mistaken for a Merkle checkpoint.
	if !strings.Contains(c.Origin, AlgVersion) {
		t.Fatalf("origin %q must carry the alg version", c.Origin)
	}
}

func TestNoteTextShape(t *testing.T) {
	c := NewLinkedCheckpoint(42, testHead())
	lines := strings.Split(c.NoteText(), "\n")
	// Three body lines plus the trailing empty element from the final newline.
	if len(lines) != 4 || lines[3] != "" {
		t.Fatalf("note text must be 3 newline-terminated lines, got %q", c.NoteText())
	}
	if lines[0] != CheckpointOrigin {
		t.Fatalf("line 1 = %q, want origin %q", lines[0], CheckpointOrigin)
	}
	if lines[1] != "42" {
		t.Fatalf("line 2 = %q, want decimal tree size 42", lines[1])
	}
	// Line 3 is base64 of the 32-byte root; base64 of 32 bytes is 44 chars.
	if len(lines[2]) != 44 {
		t.Fatalf("line 3 = %q, want base64 of a 32-byte root", lines[2])
	}
}

func TestSignAndOpenRoundTrip(t *testing.T) {
	skey, vkey, err := note.GenerateKey(rand.Reader, "s4rciv-checkpoint-test")
	if err != nil {
		t.Fatal(err)
	}
	signer, err := note.NewSigner(skey)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := note.NewVerifier(vkey)
	if err != nil {
		t.Fatal(err)
	}

	c := NewLinkedCheckpoint(8821, testHead())
	signed, err := c.Sign(signer)
	if err != nil {
		t.Fatal(err)
	}

	n, err := note.Open(signed, note.VerifierList(verifier))
	if err != nil {
		t.Fatalf("open signed checkpoint: %v", err)
	}
	if n.Text != c.NoteText() {
		t.Fatalf("round-tripped text mismatch:\n got %q\nwant %q", n.Text, c.NoteText())
	}
	if len(n.Sigs) != 1 || n.Sigs[0].Name != "s4rciv-checkpoint-test" {
		t.Fatalf("expected one signature by the test key, got %+v", n.Sigs)
	}
}

func TestTamperedNoteFailsToOpen(t *testing.T) {
	skey, vkey, _ := note.GenerateKey(rand.Reader, "s4rciv-checkpoint-test")
	signer, _ := note.NewSigner(skey)
	verifier, _ := note.NewVerifier(vkey)

	signed, err := NewLinkedCheckpoint(8821, testHead()).Sign(signer)
	if err != nil {
		t.Fatal(err)
	}
	// Flip the tree size in the body: the signature no longer matches.
	tampered := []byte(strings.Replace(string(signed), "8821", "9999", 1))
	if _, err := note.Open(tampered, note.VerifierList(verifier)); err == nil {
		t.Fatal("a tampered checkpoint must fail to open")
	}
}

func TestWrongKeyFailsToOpen(t *testing.T) {
	skey, _, _ := note.GenerateKey(rand.Reader, "s4rciv-checkpoint-test")
	signer, _ := note.NewSigner(skey)
	_, otherVkey, _ := note.GenerateKey(rand.Reader, "someone-else")
	otherVerifier, _ := note.NewVerifier(otherVkey)

	signed, _ := NewLinkedCheckpoint(8821, testHead()).Sign(signer)
	if _, err := note.Open(signed, note.VerifierList(otherVerifier)); err == nil {
		t.Fatal("a checkpoint must not open under an unrelated key")
	}
}
