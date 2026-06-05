// Package egov is the anti-corruption layer for the e-Gov 法令 API v2: it maps
// e-Gov JSON onto the interpretation-plane law domain and produces canonical,
// content-addressed snapshots (Exclusive Canonical XML over the 法令標準XML) for
// the observation plane. It implements the law source ports over an injected
// HTTP getter.
package egov

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ucarion/c14n"

	"s4rciv.org/api/internal/blob"
	leg "s4rciv.org/api/internal/domain/legislative"
	obs "s4rciv.org/api/internal/domain/observation"
	"s4rciv.org/api/internal/port"
)

// SourceName is the adapter/source identifier and stream_id prefix.
const SourceName = "egov-law"

// mediaTypeXML is stamped on the snapshot and passed to the differ.
const mediaTypeXML = "application/xml"

// updateListV1Base is the v1 fallback for the updated-law list when v2 returns 404.
const updateListV1Base = "https://laws.e-gov.go.jp/api/1"

// httpGetter is the read-only HTTP-GET boundary (driver/egovhttp.Client). The
// absURL form lets the gateway reach the v1 updatelawlists endpoint when v2 404s.
type httpGetter interface {
	Get(ctx context.Context, endpoint string, q url.Values) ([]byte, int, error)
	GetAbs(ctx context.Context, rawURL string) ([]byte, int, error)
}

type Gateway struct {
	http httpGetter
}

func New(h httpGetter) *Gateway { return &Gateway{http: h} }

// StreamID is the deterministic stream identity for an e-Gov 法令ID.
func StreamID(lawID string) string { return leg.LawStreamID(lawID) }

// Permalink is the e-Gov reference URL stamped on every record (attribution).
func Permalink(lawID string) string { return "https://laws.e-gov.go.jp/law/" + lawID }

// Fetch GETs the current in-force 法令標準XML for one law, C14N-canonicalizes it
// and content-addresses it. A missing full-text file does NOT by itself mean the
// law vanished: e-Gov flips a law's current-revision pointer in its metadata
// before publishing that revision's 法令標準XML, so law_data 404s (code 404004)
// while the law is still in force. We therefore resolve a missing full text
// against the authoritative existence signal (the /laws metadata endpoint) — the
// law leaving the registry is a genuine ResourceVanished; the full text merely
// lagging is ContentUnavailable (no event, re-poll soon). See ADR-000011.
func (g *Gateway) Fetch(ctx context.Context, w port.Watch) (port.FetchResult, error) {
	q := url.Values{}
	q.Set("law_full_text_format", "xml")
	q.Set("response_format", "json")

	body, status, err := g.http.Get(ctx, "law_data/"+w.SourceLocalKey, q)
	if err != nil {
		return port.FetchResult{}, err
	}
	if status == 404 {
		return g.absenceOrPending(ctx, w.SourceLocalKey)
	}
	if status != 200 {
		return port.FetchResult{}, fmt.Errorf("egov law_data: status %d", status)
	}

	var resp lawDataResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return port.FetchResult{}, fmt.Errorf("decode law_data response: %w", err)
	}
	if resp.LawFullText == "" {
		return g.absenceOrPending(ctx, w.SourceLocalKey)
	}

	rawXML, err := base64.StdEncoding.DecodeString(strings.TrimSpace(resp.LawFullText))
	if err != nil {
		return port.FetchResult{}, fmt.Errorf("decode base64 law_full_text: %w", err)
	}
	canonical, err := canonicalizeXML(rawXML)
	if err != nil {
		return port.FetchResult{}, fmt.Errorf("canonicalize law xml: %w", err)
	}

	compressed, err := blob.Compress(canonical)
	if err != nil {
		return port.FetchResult{}, err
	}
	snap := &port.Snapshot{
		ContentHash: obs.SumBytes(canonical),
		Bytes:       compressed,
		ByteSize:    int64(len(canonical)),
		MediaType:   mediaTypeXML,
		WasOCR:      false,
	}
	return port.FetchResult{
		Present:           true,
		Snapshot:          snap,
		SourcePublishedAt: enforcementDateOf(resp.RevisionInfo.LawRevisionID),
		Permalink:         Permalink(w.SourceLocalKey),
	}, nil
}

// absenceOrPending resolves a missing law_data full text into either a genuine
// absence (the law is gone from e-Gov's registry → ResourceVanished) or a
// content-publishing lag (the law still exists in metadata but its current
// revision's 法令標準XML is not published yet → ContentUnavailable, no event).
// A vanish requires POSITIVE confirmation that the law left the registry; when
// existence cannot be confirmed we error rather than fabricate an absence.
func (g *Gateway) absenceOrPending(ctx context.Context, lawID string) (port.FetchResult, error) {
	exists, err := g.lawExists(ctx, lawID)
	if err != nil {
		return port.FetchResult{}, fmt.Errorf("confirm law existence %s: %w", lawID, err)
	}
	if exists {
		return port.FetchResult{ContentUnavailable: true}, nil
	}
	return port.FetchResult{Present: false}, nil
}

// lawExists reports whether the law is still listed in e-Gov's law registry (the
// /laws metadata endpoint), independent of whether its full-text file is
// retrievable. e-Gov returns 200 with total_count 0 for an unknown law_id, so a
// non-200 status is treated as "cannot confirm" (error) rather than absence.
func (g *Gateway) lawExists(ctx context.Context, lawID string) (bool, error) {
	q := url.Values{}
	q.Set("law_id", lawID)
	q.Set("response_format", "json")

	body, status, err := g.http.Get(ctx, "laws", q)
	if err != nil {
		return false, err
	}
	if status != 200 {
		return false, fmt.Errorf("egov laws lookup: status %d", status)
	}
	var resp lawsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return false, fmt.Errorf("decode laws lookup: %w", err)
	}
	return resp.TotalCount > 0 || len(resp.Laws) > 0, nil
}

// ParseLaw decodes canonical snapshot bytes into the normalized law domain.
func (g *Gateway) ParseLaw(content []byte) (leg.LawContent, error) {
	c, err := leg.ParseLawXML(content)
	if err != nil {
		return leg.LawContent{}, fmt.Errorf("parse law snapshot: %w", err)
	}
	return c, nil
}

// ListLaws traverses /laws, paging via next_offset, returning stream refs to add
// to the watch list. lawType "" lists all law types.
func (g *Gateway) ListLaws(ctx context.Context, scope port.ListScope, lawType string) ([]port.LawRef, error) {
	var refs []port.LawRef
	offset := 0
	const pageSize = 100
	for {
		q := url.Values{}
		q.Set("response_format", "json")
		q.Set("limit", strconv.Itoa(pageSize))
		q.Set("offset", strconv.Itoa(offset))
		if lawType != "" {
			q.Set("law_type", lawType)
		}

		body, status, err := g.http.Get(ctx, "laws", q)
		if err != nil {
			return nil, err
		}
		if status != 200 {
			return nil, fmt.Errorf("egov laws: status %d", status)
		}
		var resp lawsResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("decode laws: %w", err)
		}
		if len(resp.Laws) == 0 {
			return refs, nil
		}
		for _, e := range resp.Laws {
			id := e.LawInfo.LawID
			if id == "" {
				continue
			}
			refs = append(refs, port.LawRef{
				StreamID: StreamID(id), SourceLocalKey: id, CanonicalURL: Permalink(id),
			})
			if scope.Max > 0 && len(refs) >= scope.Max {
				return refs, nil
			}
		}
		if resp.NextOffset <= offset {
			return refs, nil
		}
		offset = resp.NextOffset
	}
}

// ListUpdated iterates each date in the scope window and collects the LawIds that
// were updated and are in force (EnforcementFlg "0"), returning stream refs.
func (g *Gateway) ListUpdated(ctx context.Context, scope port.ListScope) ([]port.LawRef, error) {
	from, until, err := scopeDates(scope)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	var refs []port.LawRef
	for d := from; !d.After(until); d = d.AddDate(0, 0, 1) {
		ids, err := g.updatedOn(ctx, d)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			refs = append(refs, port.LawRef{
				StreamID: StreamID(id), SourceLocalKey: id, CanonicalURL: Permalink(id),
			})
			if scope.Max > 0 && len(refs) >= scope.Max {
				return refs, nil
			}
		}
	}
	return refs, nil
}

// updatedOn fetches the updated-law list for one date, falling back to the
// v1-documented path when v2 returns 404.
func (g *Gateway) updatedOn(ctx context.Context, d time.Time) ([]string, error) {
	ymd := d.Format("20060102")
	body, status, err := g.http.Get(ctx, "updatelawlists/"+ymd, nil)
	if err != nil {
		return nil, err
	}
	if status == 404 {
		body, status, err = g.http.GetAbs(ctx, updateListV1Base+"/updatelawlists/"+ymd)
		if err != nil {
			return nil, err
		}
	}
	if status == 404 {
		return nil, nil // no updates published for that date
	}
	if status != 200 {
		return nil, fmt.Errorf("egov updatelawlists %s: status %d", ymd, status)
	}
	entries, err := decodeUpdateList(body)
	if err != nil {
		return nil, fmt.Errorf("decode updatelawlists %s: %w", ymd, err)
	}
	var ids []string
	for _, e := range entries {
		if e.LawID != "" && (e.EnforcementFlg == "" || e.EnforcementFlg == "0") {
			ids = append(ids, e.LawID)
		}
	}
	return ids, nil
}

// decodeUpdateList tolerates the formats e-Gov actually serves for the updated-law
// list. The v2 host has no updatelawlists endpoint (404), so the live path is the
// v1 fallback, which serves text/xml: DataRoot > ApplData > LawNameListInfo[]. JSON
// shapes are kept for forward-compat if v2 ever ships a JSON updated-law list.
func decodeUpdateList(body []byte) ([]updateLawEntry, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, nil
	}
	if trimmed[0] == '<' { // v1 XML (the live path)
		var doc struct {
			Entries []struct {
				LawID          string `xml:"LawId"`
				EnforcementFlg string `xml:"EnforcementFlg"`
				AuthFlg        string `xml:"AuthFlg"`
			} `xml:"ApplData>LawNameListInfo"`
		}
		if err := xml.Unmarshal(trimmed, &doc); err != nil {
			return nil, err
		}
		out := make([]updateLawEntry, 0, len(doc.Entries))
		for _, e := range doc.Entries {
			out = append(out, updateLawEntry{LawID: e.LawID, EnforcementFlg: e.EnforcementFlg, AuthFlg: e.AuthFlg})
		}
		return out, nil
	}
	if trimmed[0] == '[' {
		var arr []updateLawEntry
		if err := json.Unmarshal(trimmed, &arr); err != nil {
			return nil, err
		}
		return arr, nil
	}
	var wrap struct {
		Result struct {
			Laws []updateLawEntry `json:"LawList"`
		} `json:"result"`
		ApplData struct {
			Laws []updateLawEntry `json:"LawList"`
		} `json:"ApplData"`
		Laws []updateLawEntry `json:"laws"`
	}
	if err := json.Unmarshal(trimmed, &wrap); err != nil {
		return nil, err
	}
	switch {
	case len(wrap.Result.Laws) > 0:
		return wrap.Result.Laws, nil
	case len(wrap.ApplData.Laws) > 0:
		return wrap.ApplData.Laws, nil
	default:
		return wrap.Laws, nil
	}
}

// canonicalizeXML produces Exclusive Canonical XML (W3C exc-c14n) over the
// 法令標準XML so identical content yields identical content_hash across fetches.
func canonicalizeXML(raw []byte) ([]byte, error) {
	return c14n.Canonicalize(xml.NewDecoder(bytes.NewReader(raw)))
}

// enforcementDateOf parses the 施行日 (yyyymmdd) out of a law_revision_id of the
// form {law_id}_{施行日}_{改正法令ID}; nil when it cannot be read.
func enforcementDateOf(revisionID string) *time.Time {
	parts := strings.Split(revisionID, "_")
	if len(parts) < 2 {
		return nil
	}
	t, err := time.Parse("20060102", parts[1])
	if err != nil {
		return nil
	}
	return &t
}

func scopeDates(scope port.ListScope) (from, until time.Time, err error) {
	if scope.From == "" || scope.Until == "" {
		return from, until, fmt.Errorf("update scope requires from and until")
	}
	from, err = time.Parse("2006-01-02", scope.From)
	if err != nil {
		return from, until, fmt.Errorf("parse from: %w", err)
	}
	until, err = time.Parse("2006-01-02", scope.Until)
	if err != nil {
		return from, until, fmt.Errorf("parse until: %w", err)
	}
	return from, until, nil
}
