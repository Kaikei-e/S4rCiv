// Package blob is the snapshot payload codec. Mirrored snapshot bytes are gzip
// (ADR-000001 "compressed payload"); Decompress is magic-byte tolerant so a
// future codec change or an uncompressed external payload still reads.
package blob

import (
	"bytes"
	"compress/gzip"
	"io"
)

func Compress(b []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(b); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func Decompress(b []byte) ([]byte, error) {
	if len(b) < 2 || b[0] != 0x1f || b[1] != 0x8b {
		return b, nil // not gzip — return as stored
	}
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}
