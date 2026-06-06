// Package signer loads the checkpoint signing key and implements port.CheckpointSigner
// over the C2SP/sumdb note format. The Ed25519 private key is read from a mounted
// secret file (CHECKPOINT_SIGNING_KEY_FILE) and is never placed in an env var or
// logged — the same discipline the DB password follows (ADR-000019; cf. postgres.Connect).
package signer

import (
	"crypto/rand"
	"fmt"
	"os"
	"strings"

	"golang.org/x/mod/sumdb/note"

	obs "s4rciv.org/api/internal/domain/observation"
)

// KeyFileEnv names the env var pointing at the mounted signing-key secret file.
const KeyFileEnv = "CHECKPOINT_SIGNING_KEY_FILE"

// Signer signs checkpoints with a loaded note key. It implements port.CheckpointSigner.
type Signer struct{ ns note.Signer }

// SignCheckpoint signs the checkpoint's note and returns the signed-note bytes plus
// the key name (used as signer_key_id).
func (s Signer) SignCheckpoint(c obs.Checkpoint) ([]byte, string, error) {
	b, err := c.Sign(s.ns)
	if err != nil {
		return nil, "", err
	}
	return b, s.ns.Name(), nil
}

// Load reads the note signing key from the mounted secret file and returns a Signer.
func Load() (Signer, error) {
	path := os.Getenv(KeyFileEnv)
	if path == "" {
		return Signer{}, fmt.Errorf("%s is not set", KeyFileEnv)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Signer{}, fmt.Errorf("read signing key secret: %w", err)
	}
	skey := strings.TrimSpace(string(b))
	if skey == "" {
		return Signer{}, fmt.Errorf("signing key secret at %s is empty", path)
	}
	ns, err := note.NewSigner(skey)
	if err != nil {
		return Signer{}, fmt.Errorf("parse signing key: %w", err)
	}
	return Signer{ns: ns}, nil
}

// Generate makes a fresh Ed25519 note keypair. skey is the private signer key (store
// it as the mounted secret); vkey is the public verifier key (publish it so third
// parties can verify checkpoints). Ops-only helper.
func Generate(name string) (skey, vkey string, err error) {
	return note.GenerateKey(rand.Reader, name)
}
