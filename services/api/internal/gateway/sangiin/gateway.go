// Package sangiin is the anti-corruption layer for the 参議院本会議投票結果
// (touhyoulist) — per-member named votes that the district vote map needs (ADR-000010,
// because the kokkai 会議録 records 衆 記名投票 as counts only). It GETs the public
// vote-result pages over a read-only HTTP boundary and parses each member's 賛否.
package sangiin

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"golang.org/x/net/html"

	"s4rciv.org/api/internal/blob"
	leg "s4rciv.org/api/internal/domain/legislative"
	obs "s4rciv.org/api/internal/domain/observation"
	"s4rciv.org/api/internal/port"
)

// SourceName is the adapter/source identifier and stream_id prefix.
const SourceName = "sangiin-vote"

const mediaTypeHTML = "text/html; charset=utf-8"

// httpGetter is the read-only HTTP-GET boundary. The vote pages live at absolute
// URLs on www.sangiin.go.jp.
type httpGetter interface {
	GetAbs(ctx context.Context, rawURL string) ([]byte, int, error)
}

type Gateway struct {
	http httpGetter
}

func New(h httpGetter) *Gateway { return &Gateway{http: h} }

// StreamID is the deterministic stream identity for one vote-result page, keyed by
// the page slug "{session}-{MMDD}-v{NNN}" (e.g. "221-0407-v001").
func StreamID(slug string) string { return "sangiin-vote:" + slug }

// Fetch GETs one vote-result page (UTF-8, unlike the Shift_JIS 衆 roster) and
// content-addresses it. Absence (404) is reported as not-present (ResourceVanished).
func (g *Gateway) Fetch(ctx context.Context, w port.Watch) (port.FetchResult, error) {
	body, status, err := g.http.GetAbs(ctx, w.CanonicalURL)
	if err != nil {
		return port.FetchResult{}, err
	}
	if status == 404 {
		return port.FetchResult{Present: false}, nil
	}
	if status != 200 {
		return port.FetchResult{}, fmt.Errorf("sangiin-vote %s: status %d", w.CanonicalURL, status)
	}
	compressed, err := blob.Compress(body)
	if err != nil {
		return port.FetchResult{}, err
	}
	return port.FetchResult{
		Present: true,
		Snapshot: &port.Snapshot{
			ContentHash: obs.SumBytes(body),
			Bytes:       compressed,
			ByteSize:    int64(len(body)),
			MediaType:   mediaTypeHTML,
		},
		Permalink: w.CanonicalURL,
	}, nil
}

// ParseVotePage parses a 参議院 roll-call page into a per-member named vote set. The
// page is well-formed UTF-8 HTML5, so an html.Parse tree walk navigates it cleanly:
// title → 議案件名, h2.kaiji_nichiji → 回次/日付, h3.tohyosousu → 賛否票数, h4.party →
// the current 会派, and each li.giin holds <span class="pros">/<span class="cons">/
// <span class="names"> for one member.
func (g *Gateway) ParseVotePage(content []byte) (leg.SangiinVotePage, error) {
	doc, err := html.Parse(bytes.NewReader(content))
	if err != nil {
		return leg.SangiinVotePage{}, fmt.Errorf("parse sangiin vote html: %w", err)
	}
	var page leg.SangiinVotePage
	group := ""
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			class := attr(n, "class")
			switch {
			case n.Data == "title" && page.Motion == "":
				page.Motion = motionFromTitle(textOf(n))
			case n.Data == "h2" && strings.Contains(class, "kaiji_nichiji"):
				page.Session, page.Date = parseSessionDate(textOf(n))
			case n.Data == "h3" && strings.Contains(class, "tohyosousu"):
				page.YesCount = leg.ParseSangiinHeadingNumber(between(textOf(n), "賛成票", "反対票"))
				page.NoCount = leg.ParseSangiinHeadingNumber(after(textOf(n), "反対票"))
			case n.Data == "h4" && strings.Contains(class, "party"):
				group = leg.CleanParliamentaryGroup(textOf(n))
			case n.Data == "li" && strings.Contains(class, "giin"):
				page.Votes = append(page.Votes, giinVote(n, group))
				return // a member li's spans are leaves; don't descend
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return page, nil
}

// giinVote reads one <li class="giin"> — its pros/cons/names spans (in that order).
func giinVote(li *html.Node, group string) leg.SangiinNamedVote {
	var pros, cons, name string
	for c := li.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || c.Data != "span" {
			continue
		}
		switch cl := attr(c, "class"); {
		case strings.Contains(cl, "pros"):
			pros = textOf(c)
		case strings.Contains(cl, "cons"):
			cons = textOf(c)
		case strings.Contains(cl, "names"):
			name = cleanName(textOf(c))
		}
	}
	return leg.SangiinNamedVote{Name: name, Option: leg.SangiinOption(pros, cons), Group: group}
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func textOf(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(m *html.Node) {
		if m.Type == html.TextNode {
			b.WriteString(m.Data)
		}
		for c := m.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(b.String())
}

// cleanName collapses the full-width spaces the roll-call pads names with
// ("青木　　一彦" → "青木 一彦").
func cleanName(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "　", " ")), " ")
}

// motionFromTitle takes the 議案件名 from "<件名>：本会議投票結果：参議院".
func motionFromTitle(title string) string {
	if i := strings.IndexAny(title, "：:"); i > 0 {
		return strings.TrimSpace(title[:i])
	}
	return strings.TrimSpace(title)
}

// parseSessionDate reads "第221回国会2026年 4月 7日投票結果" → (221, "2026-04-07").
// The date is parsed from AFTER "国会" so the session digits (221) don't bleed into
// the year. The page prints the Gregorian year (西暦), so no 和暦 conversion is needed.
func parseSessionDate(s string) (session int, date string) {
	session = leg.ParseSangiinHeadingNumber(between(s, "第", "回"))
	rest := after(s, "国会")
	if rest == "" {
		rest = s
	}
	y := leg.ParseSangiinHeadingNumber(between(rest, "", "年"))
	m := leg.ParseSangiinHeadingNumber(between(rest, "年", "月"))
	d := leg.ParseSangiinHeadingNumber(between(rest, "月", "日"))
	if y > 0 && m > 0 && d > 0 {
		date = fmt.Sprintf("%04d-%02d-%02d", y, m, d)
	}
	return session, date
}

// between returns the substring of s strictly between the first occurrence of lo and
// the first occurrence of hi after it. lo "" means start-of-string.
func between(s, lo, hi string) string {
	start := 0
	if lo != "" {
		i := strings.Index(s, lo)
		if i < 0 {
			return ""
		}
		start = i + len(lo)
	}
	rest := s[start:]
	if hi == "" {
		return rest
	}
	if j := strings.Index(rest, hi); j >= 0 {
		return rest[:j]
	}
	return rest
}

func after(s, marker string) string {
	if i := strings.Index(s, marker); i >= 0 {
		return s[i+len(marker):]
	}
	return ""
}
