package observation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
)

// This is the PRODUCER side of the consumer-driven contract for the in-browser
// verifier (ADR-000014). It emits the canonical field set, the Deterministic
// wire bytes, and the log_hash for a table of HashableEvent fixtures into a
// committed golden file. The TypeScript verifier's CDC test (web/) consumes the
// SAME file and must reproduce every wireHex and logHashHex byte-for-byte.
//
// proto's Deterministic marshal is explicitly NOT a canonical/portable spec
// (protobuf.dev/programming-guides/serialization-not-canonical): it only holds
// for THIS scalar-only, all-fields-populated schema because proto3 then encodes
// in field-number order and omits zero values identically across implementations
// (ADR-000003). This golden file is what pins that — any drift on either side,
// or an accidental map/optional sneaking into HashableEvent, fails the contract.
//
// Regenerate after an INTENTIONAL schema/encoding change (and bump AlgVersion):
//
//	UPDATE_GOLDEN=1 go test ./internal/domain/observation/ -run TestHashableGoldenVectors

const goldenPath = "testdata/hashable_golden.json"

type goldenFields struct {
	EventID           string `json:"eventId"`
	StreamID          string `json:"streamId"`
	StreamSeq         string `json:"streamSeq"` // int64 as decimal string (JS BigInt; >2^53 safe)
	Type              int32  `json:"type"`      // EventType enum number
	Source            string `json:"source"`
	FetcherVersion    string `json:"fetcherVersion"`
	ObservedAt        string `json:"observedAt"`
	SourcePublishedAt string `json:"sourcePublishedAt"`
	ContentHash       string `json:"contentHash"`
	PrevContentHash   string `json:"prevContentHash"`
	LogPrevHash       string `json:"logPrevHash"`
}

type goldenVector struct {
	Name       string       `json:"name"`
	Fields     goldenFields `json:"fields"`
	WireHex    string       `json:"wireHex"`
	LogHashHex string       `json:"logHashHex"`
}

type goldenFile struct {
	AlgVersion string         `json:"algVersion"`
	Note       string         `json:"note"`
	Vectors    []goldenVector `json:"vectors"`
}

// vectorSpec describes one fixture before hashing; LogPrevHash is threaded from
// the previous event in a chain (genesis = zero) so the contract also exercises
// log-chain linkage, not just single-event encoding.
type vectorSpec struct {
	name string
	mut  func(*EventFacts)
}

func buildGoldenVectors(t *testing.T) []goldenVector {
	t.Helper()

	at := func(s string) time.Time {
		ts, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatalf("parse time %q: %v", s, err)
		}
		return ts
	}
	digest := func(s string) *Digest { d := SumBytes([]byte(s)); return &d }
	published := at("2026-01-15T00:00:00Z")

	// Fictional placeholders only — no real legislator names anywhere.
	const stream = "kokkai:100000000X00120260101"

	// A 4-event chain plus two stress fixtures. Each mutates a baseline.
	specs := []vectorSpec{
		{"genesis_observed", func(f *EventFacts) {
			f.StreamSeq, f.Type = 1, ResourceObserved
			f.ContentHash = digest("snapshot-1")
			f.PrevContentHash = nil
			f.SourcePublishedAt = &published // source asserts a publish time
		}},
		{"second_changed", func(f *EventFacts) {
			f.StreamSeq, f.Type = 2, ResourceChanged
			f.ContentHash = digest("snapshot-2")
			f.PrevContentHash = digest("snapshot-1")
			f.SourcePublishedAt = nil // source asserts none -> "" encoding
		}},
		{"third_vanished", func(f *EventFacts) {
			f.StreamSeq, f.Type = 3, ResourceVanished
			f.ContentHash = nil // vanished carries no content -> "" encoding
			f.PrevContentHash = digest("snapshot-2")
			f.SourcePublishedAt = nil
		}},
		{"fourth_restored", func(f *EventFacts) {
			f.StreamSeq, f.Type = 4, ResourceRestored
			f.ContentHash = digest("snapshot-4")
			f.PrevContentHash = digest("snapshot-2")
			f.SourcePublishedAt = &published
		}},
		{"large_stream_seq_beyond_js_safe_int", func(f *EventFacts) {
			f.StreamSeq = 9007199254740993 // 2^53 + 1: lossy as a JS number, exact as BigInt
			f.Type = ResourceObserved
			f.ContentHash = digest("snapshot-big")
			f.PrevContentHash = nil
			f.SourcePublishedAt = nil
		}},
		{"zero_stream_seq_unspecified_type", func(f *EventFacts) {
			f.StreamSeq = 0       // zero int64 -> omitted by both encoders
			f.Type = Unknown      // EVENT_TYPE_UNSPECIFIED(0) -> omitted by both encoders
			f.ContentHash = nil   // -> "" omitted
			f.PrevContentHash = nil
			f.SourcePublishedAt = nil
		}},
	}

	var out []goldenVector
	var prevLog Digest // genesis = 32 zero bytes
	for i, sp := range specs {
		facts := EventFacts{
			EventID:        "00000000-0000-7000-8000-" + string16(i),
			StreamID:       stream,
			Source:         "kokkai",
			FetcherVersion: "kokkai-collector/0.1-test",
			ObservedAt:     at("2026-06-02T09:00:00Z").Add(time.Duration(i) * time.Hour),
			LogPrevHash:    prevLog,
		}
		sp.mut(&facts)

		he := facts.Hashable()
		wire, err := proto.MarshalOptions{Deterministic: true}.Marshal(he)
		if err != nil {
			t.Fatalf("%s: marshal: %v", sp.name, err)
		}
		logHash, err := facts.LogHash()
		if err != nil {
			t.Fatalf("%s: log hash: %v", sp.name, err)
		}
		// Self-check: LogHash must equal sha256(Deterministic-marshal(Hashable())).
		if want := sha256.Sum256(wire); want != logHash {
			t.Fatalf("%s: LogHash() diverged from sha256(marshal(Hashable()))", sp.name)
		}

		out = append(out, goldenVector{
			Name: sp.name,
			Fields: goldenFields{
				EventID:           he.GetEventId(),
				StreamID:          he.GetStreamId(),
				StreamSeq:         strconv.FormatInt(he.GetStreamSeq(), 10),
				Type:              int32(he.GetType()),
				Source:            he.GetSource(),
				FetcherVersion:    he.GetFetcherVersion(),
				ObservedAt:        he.GetObservedAt(),
				SourcePublishedAt: he.GetSourcePublishedAt(),
				ContentHash:       he.GetContentHash(),
				PrevContentHash:   he.GetPrevContentHash(),
				LogPrevHash:       he.GetLogPrevHash(),
			},
			WireHex:    hex.EncodeToString(wire),
			LogHashHex: logHash.Hex(),
		})
		// Chain the first four; the stress fixtures stand alone.
		if i < 3 {
			prevLog = logHash
		}
	}
	return out
}

// string16 yields a stable 12-hex-digit tail so each fixture has a distinct,
// canonical-looking uuidv7 without pulling in a uuid dependency.
func string16(i int) string {
	return strconv.FormatInt(int64(0x100000000000+i), 16)
}

func TestHashableGoldenVectors(t *testing.T) {
	got := goldenFile{
		AlgVersion: AlgVersion,
		Note: "CDC contract for the in-browser verifier (ADR-000014). PRODUCER: " +
			"services/api/internal/domain/observation/golden_test.go. CONSUMER: web/ " +
			"verifier CDC test. Regenerate with UPDATE_GOLDEN=1 and bump AlgVersion on " +
			"any intentional HashableEvent encoding change.",
		Vectors: buildGoldenVectors(t),
	}
	data, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("marshal golden: %v", err)
	}
	data = append(data, '\n')

	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(goldenPath, data, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("updated %s", goldenPath)
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (first run? regenerate with UPDATE_GOLDEN=1): %v", err)
	}
	if !bytes.Equal(want, data) {
		t.Errorf("CDC golden drift: a fresh marshal no longer matches %s.\n"+
			"If HashableEvent's schema/encoding changed INTENTIONALLY, bump AlgVersion and "+
			"rerun with UPDATE_GOLDEN=1. Otherwise a proto-library change broke byte-identity "+
			"and the in-browser verifier would silently disagree with Go.", goldenPath)
	}
}
