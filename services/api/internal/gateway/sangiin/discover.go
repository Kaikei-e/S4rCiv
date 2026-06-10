package sangiin

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"s4rciv.org/api/internal/port"
)

const siteBase = "https://www.sangiin.go.jp"

// isRootedPath reports whether p is a safe same-site relative path: it must start
// with a single "/" and carry no scheme or userinfo, so siteBase+p can never be
// re-anchored to another host (SSRF guard, CWE-918). Rejects "//host", "://" and "@".
func isRootedPath(p string) bool {
	return strings.HasPrefix(p, "/") &&
		!strings.HasPrefix(p, "//") &&
		!strings.Contains(p, "://") &&
		!strings.Contains(p, "@")
}

var (
	reRedirect      = regexp.MustCompile(`location\.replace\("([^"]+)"\)`)
	reRosterSession = regexp.MustCompile(`/giin/([0-9]+)/giin\.htm`)
	reSessionIndex  = regexp.MustCompile(`/touhyoulist/([0-9]+)/vote_ind\.htm`)
	reVoteSlug      = regexp.MustCompile(`([0-9]+-[0-9]{4}-v[0-9]+)\.htm`)
)

// DiscoverRoster follows the /current/ JavaScript redirect to the current-session
// 議員一覧 page and returns its watch ref.
func (g *Gateway) DiscoverRoster(ctx context.Context) (port.MeetingRef, error) {
	body, status, err := g.http.GetAbs(ctx, siteBase+"/japanese/joho1/kousei/giin/current/giin.htm")
	if err != nil {
		return port.MeetingRef{}, err
	}
	if status != 200 {
		return port.MeetingRef{}, fmt.Errorf("sangiin roster current: status %d", status)
	}
	m := reRedirect.FindSubmatch(body)
	if m == nil {
		return port.MeetingRef{}, fmt.Errorf("sangiin roster: redirect target not found")
	}
	path := string(m[1])
	// The redirect target is page-derived, so guard it before it is concatenated to
	// siteBase: only a rooted same-site relative path is accepted, so a poisoned
	// source can never re-anchor CanonicalURL onto another host (SSRF, CWE-918).
	if !isRootedPath(path) {
		return port.MeetingRef{}, fmt.Errorf("sangiin roster: refusing non-rooted redirect target %q", path)
	}
	session := ""
	if sm := reRosterSession.FindStringSubmatch(path); sm != nil {
		session = sm[1]
	}
	return port.MeetingRef{
		StreamID: RosterStreamID(session), SourceLocalKey: session, CanonicalURL: siteBase + path,
	}, nil
}

// DiscoverVotes lists the latest session's 記名投票 result pages (HTML only; PDF votes
// such as the 首相指名 are 単記 and skipped). One ref per 議案.
func (g *Gateway) DiscoverVotes(ctx context.Context) ([]port.MeetingRef, error) {
	idx, status, err := g.http.GetAbs(ctx, siteBase+"/japanese/touhyoulist/touhyoulist.html")
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("touhyoulist index: status %d", status)
	}
	sm := reSessionIndex.FindSubmatch(idx) // newest session is listed first
	if sm == nil {
		return nil, fmt.Errorf("touhyoulist: no session index found")
	}
	session := string(sm[1])

	vidx, status, err := g.http.GetAbs(ctx, fmt.Sprintf("%s/japanese/touhyoulist/%s/vote_ind.htm", siteBase, session))
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("vote_ind %q: status %d", session, status)
	}
	seen := map[string]bool{}
	var refs []port.MeetingRef
	for _, m := range reVoteSlug.FindAllSubmatch(vidx, -1) {
		slug := string(m[1])
		if seen[slug] {
			continue
		}
		seen[slug] = true
		refs = append(refs, port.MeetingRef{
			StreamID:       StreamID(slug),
			SourceLocalKey: slug,
			CanonicalURL:   fmt.Sprintf("%s/japanese/touhyoulist/%s/%s.htm", siteBase, session, slug),
		})
	}
	return refs, nil
}
