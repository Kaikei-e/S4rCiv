package sangiin

import (
	"bytes"
	"fmt"

	"golang.org/x/net/html"

	leg "s4rciv.org/api/internal/domain/legislative"
)

// RosterSourceName is the source id / stream_id prefix for the 参議院議員名簿. The vote
// pages give 議員名 + 賛否 but no 選挙区 (ADR-000010), so the map joins votes to this
// roster (議員名 → 都道府県/比例) by normalized name.
const RosterSourceName = "sangiin-roster"

// RosterStreamID is the deterministic stream identity for one 参 roster page (keyed by
// the session, e.g. "221").
func RosterStreamID(session string) string { return "sangiin-roster:" + session }

// ParseRoster reads the 参議院 議員一覧 table (well-formed UTF-8) into roster entries.
// Each member <tr> has 6 <td> cells (議員氏名 / 読み方 / 会派 / 選挙区 / 任期満了 / blank); a
// 五十音 group's first row also carries a <th> header, which is ignored (td-only). The
// header row (all <th>) yields no td cells and is skipped, and a row whose 選挙区 does
// not resolve is dropped — so the domain parser is itself the filter.
func (g *Gateway) ParseRoster(content []byte) ([]leg.RosterEntry, error) {
	doc, err := html.Parse(bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("parse sangiin roster html: %w", err)
	}
	var out []leg.RosterEntry
	for _, cells := range tdRows(doc) {
		if len(cells) < 4 {
			continue
		}
		if e, ok := leg.NewSangiinRosterEntry(cells[0], cells[1], cells[2], cells[3]); ok {
			out = append(out, e)
		}
	}
	return out, nil
}

// tdRows returns each <tr>'s <td> cell texts (ignoring <th>) in document order.
func tdRows(root *html.Node) [][]string {
	var rows [][]string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			var cells []string
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.Data == "td" {
					cells = append(cells, textOf(c))
				}
			}
			if len(cells) > 0 {
				rows = append(rows, cells)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return rows
}
