package observation

import (
	"testing"
	"time"
)

func baseFacts() EventFacts {
	ch := SumBytes([]byte("meeting-record-v1"))
	pub := time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)
	return EventFacts{
		EventID:           "019773b0-0000-7000-8000-000000000001",
		StreamID:          "kokkai:121815254X00120240115",
		StreamSeq:         1,
		Type:              ResourceObserved,
		Source:            "kokkai",
		FetcherVersion:    "S4rCiv-collect/0.1.0",
		ObservedAt:        time.Date(2026, 6, 2, 9, 0, 0, 0, time.UTC),
		SourcePublishedAt: &pub,
		ContentHash:       &ch,
		PrevContentHash:   nil,
		LogPrevHash:       Digest{}, // genesis
	}
}

func TestLogHashDeterministic(t *testing.T) {
	f := baseFacts()
	h1, err := f.LogHash()
	if err != nil {
		t.Fatalf("LogHash: %v", err)
	}
	h2, err := f.LogHash()
	if err != nil {
		t.Fatalf("LogHash: %v", err)
	}
	if h1 != h2 {
		t.Fatalf("non-deterministic log hash: %s != %s", h1.Hex(), h2.Hex())
	}
	if h1 == (Digest{}) {
		t.Fatal("log hash must not be the genesis zero value")
	}
}

func TestLogHashTamperDetection(t *testing.T) {
	base := baseFacts()
	baseHash, err := base.LogHash()
	if err != nil {
		t.Fatalf("LogHash: %v", err)
	}

	otherCH := SumBytes([]byte("tampered"))
	otherTime := time.Date(2026, 6, 2, 9, 0, 1, 0, time.UTC)
	otherPub := time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC)
	otherLogPrev := SumBytes([]byte("different-prev"))

	mutators := map[string]func(*EventFacts){
		"event_id":            func(f *EventFacts) { f.EventID = "019773b0-0000-7000-8000-0000000000ff" },
		"stream_id":           func(f *EventFacts) { f.StreamID = "kokkai:other" },
		"stream_seq":          func(f *EventFacts) { f.StreamSeq = 2 },
		"type":                func(f *EventFacts) { f.Type = ResourceChanged },
		"source":              func(f *EventFacts) { f.Source = "egov-law" },
		"fetcher_version":     func(f *EventFacts) { f.FetcherVersion = "S4rCiv-collect/0.2.0" },
		"observed_at":         func(f *EventFacts) { f.ObservedAt = otherTime },
		"source_published_at": func(f *EventFacts) { f.SourcePublishedAt = &otherPub },
		"content_hash":        func(f *EventFacts) { f.ContentHash = &otherCH },
		"prev_content_hash":   func(f *EventFacts) { f.PrevContentHash = &otherCH },
		"log_prev_hash":       func(f *EventFacts) { f.LogPrevHash = otherLogPrev },
	}

	for field, mutate := range mutators {
		f := baseFacts()
		mutate(&f)
		h, err := f.LogHash()
		if err != nil {
			t.Fatalf("%s: LogHash: %v", field, err)
		}
		if h == baseHash {
			t.Errorf("mutating %s did not change the log hash (tamper went undetected)", field)
		}
	}
}

// A nil source_published_at and the empty-string encoding must hash the same as
// each other only when they represent the same absence — and must differ from a
// present time. Guards the "every field populated, documented empty encoding" rule.
func TestLogHashAbsentVsPresentPublishedAt(t *testing.T) {
	f := baseFacts()
	f.SourcePublishedAt = nil
	absent, _ := f.LogHash()

	pub := time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)
	f.SourcePublishedAt = &pub
	present, _ := f.LogHash()

	if absent == present {
		t.Fatal("absent and present source_published_at must hash differently")
	}
}

// Sub-second jitter must not change the hash: observed_at is canonicalized to
// second precision.
func TestLogHashSecondPrecision(t *testing.T) {
	a := baseFacts()
	a.ObservedAt = time.Date(2026, 6, 2, 9, 0, 0, 123_000_000, time.UTC)
	b := baseFacts()
	b.ObservedAt = time.Date(2026, 6, 2, 9, 0, 0, 999_000_000, time.UTC)
	ha, _ := a.LogHash()
	hb, _ := b.LogHash()
	if ha != hb {
		t.Fatal("sub-second jitter changed the log hash; canonicalization is not second-precision")
	}
}

func TestChainContinuity(t *testing.T) {
	// Build three linked events; each log_prev_hash is the prior log_hash.
	prev := Digest{}
	var hashes []Digest
	for i := int64(1); i <= 3; i++ {
		ch := SumBytes([]byte{byte(i)})
		f := baseFacts()
		f.StreamSeq = i
		f.ContentHash = &ch
		f.LogPrevHash = prev
		if i > 1 {
			pc := SumBytes([]byte{byte(i - 1)})
			f.PrevContentHash = &pc
			f.Type = ResourceChanged
		}
		h, err := f.LogHash()
		if err != nil {
			t.Fatalf("event %d: %v", i, err)
		}
		hashes = append(hashes, h)
		prev = h
	}

	// Independent re-verification: recompute and confirm each link.
	prev = Digest{}
	for i := int64(1); i <= 3; i++ {
		ch := SumBytes([]byte{byte(i)})
		f := baseFacts()
		f.StreamSeq = i
		f.ContentHash = &ch
		f.LogPrevHash = prev
		if i > 1 {
			pc := SumBytes([]byte{byte(i - 1)})
			f.PrevContentHash = &pc
			f.Type = ResourceChanged
		}
		h, _ := f.LogHash()
		if h != hashes[i-1] {
			t.Fatalf("event %d failed re-verification", i)
		}
		prev = h
	}
}

func TestDecideTransitions(t *testing.T) {
	a := SumBytes([]byte("A"))
	b := SumBytes([]byte("B"))

	cases := []struct {
		name     string
		state    StreamState
		observed *Digest
		wantType EventType
		wantEmit bool
	}{
		{"first observation", StreamState{Exists: false}, &a, ResourceObserved, true},
		{"unchanged content", StreamState{Exists: true, LastType: ResourceObserved, LastContentHash: &a}, &a, Unknown, false},
		{"changed content", StreamState{Exists: true, LastType: ResourceObserved, LastContentHash: &a}, &b, ResourceChanged, true},
		{"vanished", StreamState{Exists: true, LastType: ResourceChanged, LastContentHash: &a}, nil, ResourceVanished, true},
		{"still gone", StreamState{Exists: true, LastType: ResourceVanished, LastContentHash: &a}, nil, Unknown, false},
		{"restored", StreamState{Exists: true, LastType: ResourceVanished, LastContentHash: &a}, &b, ResourceRestored, true},
		{"404 on unseen stream", StreamState{Exists: false}, nil, Unknown, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Decide(c.state, c.observed)
			if got.Type != c.wantType || got.Emit != c.wantEmit {
				t.Fatalf("got {%v emit=%v}, want {%v emit=%v}", got.Type, got.Emit, c.wantType, c.wantEmit)
			}
		})
	}
}

func TestDigestEncoding(t *testing.T) {
	d := SumBytes([]byte("x"))
	if got := d.ContentRef(); got[:7] != "sha256:" {
		t.Fatalf("ContentRef missing prefix: %s", got)
	}
	if len(d.Hex()) != 64 {
		t.Fatalf("hex length = %d, want 64", len(d.Hex()))
	}
	rt, ok := DigestFromBytes(d.Bytes())
	if !ok || rt != d {
		t.Fatal("Digest round-trip through bytes failed")
	}
	if _, ok := DigestFromBytes([]byte{1, 2, 3}); ok {
		t.Fatal("DigestFromBytes accepted a wrong-length slice")
	}
}
