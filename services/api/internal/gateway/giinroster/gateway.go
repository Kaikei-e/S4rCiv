// Package giinroster is the anti-corruption layer for the 両院公式議員名簿 (ADR-000008).
// It GETs the public roster pages over a read-only HTTP boundary, decodes the
// Shift_JIS HTML to a canonical UTF-8 snapshot for the observation plane, and
// normalizes each member row into the legislator->district binding. Legislators
// are accountable public officials, so this binding is not private-person
// profiling (DISCIPLINE §4; ADR-000006).
package giinroster

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"

	"s4rciv.org/api/internal/blob"
	leg "s4rciv.org/api/internal/domain/legislative"
	obs "s4rciv.org/api/internal/domain/observation"
	"s4rciv.org/api/internal/port"
)

// SourceName is the adapter/source identifier and stream_id prefix.
const SourceName = "giin-roster"

// mediaTypeHTML is stamped on the canonical (UTF-8) snapshot.
const mediaTypeHTML = "text/html; charset=utf-8"

// httpGetter is the read-only HTTP-GET boundary. The roster pages live at absolute
// URLs on www.shugiin.go.jp / www.sangiin.go.jp, so GetAbs is what we need.
type httpGetter interface {
	GetAbs(ctx context.Context, rawURL string) ([]byte, int, error)
}

type Gateway struct {
	http httpGetter
}

func New(h httpGetter) *Gateway { return &Gateway{http: h} }

// StreamID is the deterministic stream identity for one roster page.
func StreamID(pageKey string) string { return leg.RosterStreamID(pageKey) }

// rosterPages are the 衆議院 議員一覧 (五十音) pages to watch. NOTE: this is the あ行 page
// only — the live list paginates the 五十音, so enumerating the full set from the roster
// index is a follow-on. 参 pages differ (1:N selection districts) and are not yet wired.
var rosterPages = []port.MeetingRef{{
	StreamID:       StreamID("shugiin-1giin"),
	SourceLocalKey: "shugiin-1giin",
	CanonicalURL:   "https://www.shugiin.go.jp/internet/itdb_annai.nsf/html/statics/syu/1giin.htm",
}}

// ListMeetings returns the roster pages to watch (satisfies port.MeetingLister, which
// the generic collector's discover reuses). scope is ignored: the roster is a fixed
// page set, not a date-windowed listing.
func (g *Gateway) ListMeetings(ctx context.Context, scope port.ListScope) ([]port.MeetingRef, error) {
	return rosterPages, nil
}

// Fetch GETs one roster page, decodes Shift_JIS -> UTF-8, and content-addresses the
// UTF-8 HTML. Decoding at fetch makes the snapshot canonical, so identical roster
// content yields an identical content_hash across polls. Absence (404) is reported
// as not-present so it can be recorded as ResourceVanished (DISCIPLINE §3).
func (g *Gateway) Fetch(ctx context.Context, w port.Watch) (port.FetchResult, error) {
	body, status, err := g.http.GetAbs(ctx, w.CanonicalURL)
	if err != nil {
		return port.FetchResult{}, err
	}
	if status == 404 {
		return port.FetchResult{Present: false}, nil
	}
	if status != 200 {
		return port.FetchResult{}, fmt.Errorf("giin-roster %s: status %d", w.CanonicalURL, status)
	}

	utf8Body, err := decodeShiftJIS(body)
	if err != nil {
		return port.FetchResult{}, fmt.Errorf("decode shift_jis: %w", err)
	}
	compressed, err := blob.Compress(utf8Body)
	if err != nil {
		return port.FetchResult{}, err
	}
	return port.FetchResult{
		Present: true,
		Snapshot: &port.Snapshot{
			ContentHash: obs.SumBytes(utf8Body),
			Bytes:       compressed,
			ByteSize:    int64(len(utf8Body)),
			MediaType:   mediaTypeHTML,
		},
		Permalink: w.CanonicalURL,
	}, nil
}

// ParseRoster reads the UTF-8 roster HTML and normalizes each 衆議院議員一覧 row
// (氏名 / ふりがな / 会派 / 選挙区 / 当選回数) into a RosterEntry. A row is a member iff it
// has 5 cells AND its 選挙区 parses — the header ("選挙区") and navigation rows fail that
// test and are skipped, so the domain parser is itself the filter.
func (g *Gateway) ParseRoster(content []byte) ([]leg.RosterEntry, error) {
	var out []leg.RosterEntry
	for _, cells := range tableRows(content) {
		if len(cells) != 5 {
			continue
		}
		if e, ok := leg.NewShugiinRosterEntry(cells[0], cells[1], cells[2], cells[3]); ok {
			out = append(out, e)
		}
	}
	return out, nil
}

// tableRows returns the trimmed text of each <tr>'s <td> cells in document order.
// It uses the TOKENIZER, not html.Parse: the 衆 roster is old Domino-generated HTML
// whose malformed table markup makes the strict HTML5 tree parser foster-parent the
// rows out of the table (losing them). A linear token scan is robust to that.
func tableRows(content []byte) [][]string {
	z := html.NewTokenizer(bytes.NewReader(content))
	var rows [][]string
	var cells []string
	var cur strings.Builder
	inTD, inRow := false, false
	flushRow := func() {
		if inRow && len(cells) > 0 {
			rows = append(rows, cells)
		}
		cells, inRow = nil, false
	}
	for {
		switch z.Next() {
		case html.ErrorToken:
			flushRow()
			return rows
		case html.StartTagToken, html.SelfClosingTagToken:
			switch name, _ := z.TagName(); string(name) {
			case "tr":
				flushRow() // tolerate a missing </tr>
				inRow = true
			case "td", "th":
				inTD = true
				cur.Reset()
			}
		case html.EndTagToken:
			switch name, _ := z.TagName(); string(name) {
			case "td", "th":
				if inTD {
					cells = append(cells, strings.TrimSpace(cur.String()))
					inTD = false
				}
			case "tr":
				flushRow()
			}
		case html.TextToken:
			if inTD {
				cur.Write(z.Text())
			}
		}
	}
}

func decodeShiftJIS(b []byte) ([]byte, error) {
	return io.ReadAll(transform.NewReader(bytes.NewReader(b), japanese.ShiftJIS.NewDecoder()))
}
