package egov

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"s4rciv.org/api/internal/blob"
	"s4rciv.org/api/internal/port"
)

// fakeGetter serves canned bodies by endpoint prefix — no live API (DISCIPLINE §1).
type fakeGetter struct {
	bodies map[string][]byte // keyed by endpoint prefix (e.g. "law_data", "laws", "updatelawlists")
	status int
	abs    map[string][]byte // keyed by absolute URL
}

func (f fakeGetter) Get(_ context.Context, endpoint string, _ url.Values) ([]byte, int, error) {
	st := f.status
	if st == 0 {
		st = 200
	}
	for prefix, body := range f.bodies {
		if strings.HasPrefix(endpoint, prefix) {
			return body, st, nil
		}
	}
	return nil, 404, nil
}

func (f fakeGetter) GetAbs(_ context.Context, rawURL string) ([]byte, int, error) {
	if body, ok := f.abs[rawURL]; ok {
		return body, 200, nil
	}
	return nil, 404, nil
}

const lawXML = `<?xml version="1.0" encoding="UTF-8"?>
<Law Era="Heisei" Year="15" LawType="Act" Num="57">
  <LawNum>平成十五年法律第五十七号</LawNum>
  <LawBody>
    <LawTitle Kana="てすとほう">テスト法</LawTitle>
    <MainProvision>
      <Article Num="1">
        <ArticleCaption>（目的）</ArticleCaption>
        <ArticleTitle>第一条</ArticleTitle>
        <Paragraph Num="1">
          <ParagraphSentence><Sentence>本文である。</Sentence></ParagraphSentence>
        </Paragraph>
      </Article>
    </MainProvision>
  </LawBody>
</Law>`

func lawDataBody(t *testing.T) []byte {
	t.Helper()
	env := map[string]any{
		"law_info": map[string]any{
			"law_type": "Act", "law_id": "415AC0000000057",
			"law_num": "平成十五年法律第五十七号", "promulgation_date": "2003-05-30",
		},
		"revision_info": map[string]any{
			"law_revision_id": "415AC0000000057_20240401_506AC0000000010",
			"law_title":       "テスト法",
		},
		"law_full_text": base64.StdEncoding.EncodeToString([]byte(lawXML)),
	}
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal law_data: %v", err)
	}
	return b
}

func TestFetchProducesStableSnapshot(t *testing.T) {
	g := New(fakeGetter{bodies: map[string][]byte{"law_data": lawDataBody(t)}})
	w := port.Watch{SourceLocalKey: "415AC0000000057"}

	r1, err := g.Fetch(context.Background(), w)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !r1.Present || r1.Snapshot == nil {
		t.Fatal("expected present snapshot")
	}
	if r1.Snapshot.MediaType != "application/xml" {
		t.Fatalf("media_type = %q", r1.Snapshot.MediaType)
	}
	if r1.Permalink != "https://laws.e-gov.go.jp/law/415AC0000000057" {
		t.Fatalf("permalink = %q", r1.Permalink)
	}
	if r1.SourcePublishedAt == nil || r1.SourcePublishedAt.Format("2006-01-02") != "2024-04-01" {
		t.Fatalf("source_published_at = %v (want 施行日 2024-04-01)", r1.SourcePublishedAt)
	}

	// C14N is stable: a second fetch yields the same content hash.
	r2, _ := g.Fetch(context.Background(), w)
	if r1.Snapshot.ContentHash != r2.Snapshot.ContentHash {
		t.Fatal("content hash is not stable across fetches")
	}

	// Mirrored bytes decompress to the canonical content that was hashed, and parse.
	raw, err := blob.Decompress(r1.Snapshot.Bytes)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	if int64(len(raw)) != r1.Snapshot.ByteSize {
		t.Fatalf("byte_size %d != decompressed len %d", r1.Snapshot.ByteSize, len(raw))
	}
	content, err := g.ParseLaw(raw)
	if err != nil {
		t.Fatalf("ParseLaw: %v", err)
	}
	if content.Law.Title != "テスト法" || len(content.Nodes) != 2 {
		t.Fatalf("parsed law = %+v nodes=%d", content.Law, len(content.Nodes))
	}
}

func TestFetchVanishedOn404(t *testing.T) {
	g := New(fakeGetter{status: 404})
	r, err := g.Fetch(context.Background(), port.Watch{SourceLocalKey: "x"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if r.Present {
		t.Fatal("404 must report not present (-> ResourceVanished)")
	}
}

func TestListLawsPaging(t *testing.T) {
	// A static fakeGetter returns the same body for every "laws" call; the gateway
	// terminates when next_offset does not advance past the current offset, so a
	// terminal page sets next_offset to 0 (= the starting offset) and returns once.
	page0, _ := json.Marshal(map[string]any{
		"total_count": 1, "count": 1, "next_offset": 0,
		"laws": []any{map[string]any{"law_info": map[string]any{"law_id": "A1"}}},
	})
	g := New(fakeGetter{bodies: map[string][]byte{"laws": page0}})
	refs, err := g.ListLaws(context.Background(), port.ListScope{}, "Act")
	if err != nil {
		t.Fatalf("ListLaws: %v", err)
	}
	if len(refs) != 1 || refs[0].StreamID != "egov-law:A1" {
		t.Fatalf("refs = %+v", refs)
	}
	if refs[0].CanonicalURL != "https://laws.e-gov.go.jp/law/A1" {
		t.Fatalf("canonical_url = %q", refs[0].CanonicalURL)
	}
}

func TestListUpdatedFiltersByEnforcement(t *testing.T) {
	body, _ := json.Marshal([]map[string]any{
		{"LawId": "A1", "EnforcementFlg": "0", "AuthFlg": "0"},
		{"LawId": "A2", "EnforcementFlg": "1", "AuthFlg": "0"}, // 未施行 -> skipped
		{"LawId": "A3", "EnforcementFlg": "0", "AuthFlg": "1"},
	})
	g := New(fakeGetter{bodies: map[string][]byte{"updatelawlists": body}})
	refs, err := g.ListUpdated(context.Background(), port.ListScope{From: "2024-04-01", Until: "2024-04-01"})
	if err != nil {
		t.Fatalf("ListUpdated: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("refs = %d, want 2 (in-force only): %+v", len(refs), refs)
	}
	got := map[string]bool{refs[0].SourceLocalKey: true, refs[1].SourceLocalKey: true}
	if !got["A1"] || !got["A3"] {
		t.Fatalf("expected A1 and A3, got %+v", refs)
	}
}

func TestListUpdatedV1Fallback(t *testing.T) {
	body, _ := json.Marshal([]map[string]any{{"LawId": "B9", "EnforcementFlg": "0"}})
	// v2 endpoint 404s (not in bodies); v1 absolute URL serves the list.
	g := New(fakeGetter{
		bodies: map[string][]byte{},
		abs: map[string][]byte{
			updateListV1Base + "/updatelawlists/20240401": body,
		},
	})
	refs, err := g.ListUpdated(context.Background(), port.ListScope{From: "2024-04-01", Until: "2024-04-01"})
	if err != nil {
		t.Fatalf("ListUpdated: %v", err)
	}
	if len(refs) != 1 || refs[0].SourceLocalKey != "B9" {
		t.Fatalf("v1 fallback refs = %+v", refs)
	}
}
