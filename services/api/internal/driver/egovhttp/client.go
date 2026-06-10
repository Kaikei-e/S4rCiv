// Package egovhttp is the read-only HTTP-GET boundary to the e-Gov 法令 API v2.
// It enforces the DISCIPLINE §1 obligations in one place: serial access with a
// per-source interval, an identifying User-Agent, and robots.txt compliance. It
// only ever issues GET (DISCIPLINE §2). It mirrors driver/kokkaihttp; the small
// duplication is accepted to keep each source's boundary independent, and it adds
// GetAbs for the v1 updatelawlists fallback host and the cross-host roster pages.
//
// GetAbs is the one place an absolute, content-derived URL is fetched, so it is the
// SSRF chokepoint (CWE-918): the scheme must be http(s), the URL must carry no
// userinfo, and the host must be on the per-client allowlist (the base host plus
// any extra hosts passed to New). robots.txt is evaluated against the ACTUAL target
// host, not the base host, so the §7 compliance check matches the request.
package egovhttp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/temoto/robotstxt"
)

// maxRedirects caps redirect chains; every hop is re-validated by checkRedirect.
const maxRedirects = 10

// Client serializes and spaces requests to one source. Construct one per source.
type Client struct {
	base     *url.URL
	ua       string
	interval time.Duration
	http     *http.Client
	allowed  map[string]struct{} // hostnames this client may reach (base + extras)

	mu   sync.Mutex // serializes requests (no parallel/burst access)
	next time.Time  // earliest time the next request may go out

	robots sync.Map // host(string) -> *robotsResult, fetched at most once per host
}

// robotsResult memoizes one host's robots.txt group (or the error fetching it).
// done is set after the once completes so redirect hops can read the cached group
// race-free without going through (and possibly blocking on) the once.
type robotsResult struct {
	once  sync.Once
	done  atomic.Bool
	group *robotstxt.Group
	err   error
}

// New builds a client anchored on baseURL. The base host is always reachable;
// allowedHosts widens the GetAbs allowlist for sources that legitimately span more
// than one host (e.g. the 両院 roster on www.shugiin.go.jp + www.sangiin.go.jp).
func New(baseURL, userAgent string, interval time.Duration, allowedHosts ...string) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}
	allowed := map[string]struct{}{strings.ToLower(u.Hostname()): {}}
	for _, h := range allowedHosts {
		if h != "" {
			allowed[strings.ToLower(h)] = struct{}{}
		}
	}
	c := &Client{
		base:     u,
		ua:       userAgent,
		interval: interval,
		allowed:  allowed,
	}
	c.http = &http.Client{Timeout: 30 * time.Second, CheckRedirect: c.checkRedirect}
	return c, nil
}

// checkRedirect re-applies the GetAbs validation to every redirect hop, so a
// redirecting upstream cannot steer the collector off the allowlist (SSRF,
// CWE-918): only http(s), no userinfo, and an allowlisted host. When the hop
// host's robots.txt is already cached the hop path is re-tested against it;
// fetching it here would bypass the serial interval gate, so an uncached (but
// allowlisted) host is covered by the host allowlist alone.
func (c *Client) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	}
	u := req.URL
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("refusing redirect to non-http(s) scheme %q", u.Scheme)
	}
	if u.User != nil {
		return fmt.Errorf("refusing redirect with userinfo: %q", u.Redacted())
	}
	host := strings.ToLower(u.Hostname())
	if _, ok := c.allowed[host]; !ok {
		return fmt.Errorf("refusing redirect to off-allowlist host %q (SSRF guard)", host)
	}
	if g := c.cachedRobots(u.Host); g != nil && !g.Test(u.Path) {
		return fmt.Errorf("robots.txt disallows redirect target %q on %q", u.Path, host)
	}
	return nil
}

// cachedRobots returns the host's robots group when (and only when) its fetch has
// already completed; nil otherwise. Never fetches. Keyed like checkRobots (the
// URL's Host, which may carry a port).
func (c *Client) cachedRobots(host string) *robotstxt.Group {
	v, ok := c.robots.Load(strings.ToLower(host))
	if !ok {
		return nil
	}
	r := v.(*robotsResult)
	if !r.done.Load() {
		return nil
	}
	return r.group
}

// Get fetches base + "/" + endpoint with the given query, spacing requests by the
// per-source interval. A 404 is returned to the caller (not an error).
func (c *Client) Get(ctx context.Context, endpoint string, q url.Values) ([]byte, int, error) {
	target := *c.base
	target.Path = singleJoin(c.base.Path, endpoint)
	if q != nil {
		target.RawQuery = q.Encode()
	}
	// The target is built from the base, so its host is the (always-allowed) base host.
	return c.fetch(ctx, target.String(), c.base.Scheme, c.base.Host, target.Path)
}

// GetAbs fetches an absolute URL (the v1 updatelawlists fallback and the roster
// pages). It is the SSRF chokepoint: only http(s), no userinfo, and an
// allowlisted host may be fetched. It is subject to the same serial interval and
// robots policy as Get.
func (c *Client) GetAbs(ctx context.Context, rawURL string) ([]byte, int, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, 0, fmt.Errorf("parse url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, 0, fmt.Errorf("refusing non-http(s) url scheme %q", u.Scheme)
	}
	if u.User != nil {
		return nil, 0, fmt.Errorf("refusing url with userinfo: %s", u.Redacted())
	}
	host := strings.ToLower(u.Hostname())
	if _, ok := c.allowed[host]; !ok {
		return nil, 0, fmt.Errorf("refusing off-allowlist host %q (SSRF guard)", host)
	}
	return c.fetch(ctx, u.String(), u.Scheme, u.Host, u.Path)
}

func (c *Client) fetch(ctx context.Context, rawURL, scheme, host, path string) ([]byte, int, error) {
	if err := c.checkRobots(ctx, scheme, host, path); err != nil {
		return nil, 0, err
	}
	return c.gatedDo(ctx, rawURL)
}

// gatedDo serializes the request behind the per-source mutex and waits out the
// interval since the previous request. Every outbound request — including the
// robots.txt fetches — goes through here, so first contact never bursts.
func (c *Client) gatedDo(ctx context.Context, rawURL string) ([]byte, int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if wait := time.Until(c.next); wait > 0 {
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case <-time.After(wait):
		}
	}
	body, status, err := c.do(ctx, rawURL)
	c.next = time.Now().Add(c.interval)
	return body, status, err
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

// checkRobots lazily fetches and caches robots.txt for the TARGET host (not the
// base host) and rejects a disallowed path. A missing robots.txt is "allow all".
// The fetch consumes an interval slot (gatedDo) like any other request, so first
// contact with a host does not send two back-to-back requests.
func (c *Client) checkRobots(ctx context.Context, scheme, host, path string) error {
	v, _ := c.robots.LoadOrStore(strings.ToLower(host), &robotsResult{})
	r := v.(*robotsResult)
	r.once.Do(func() {
		defer r.done.Store(true)
		body, status, err := c.gatedDo(ctx, scheme+"://"+host+"/robots.txt")
		if err != nil {
			r.err = fmt.Errorf("fetch robots.txt for %q: %w", host, err)
			return
		}
		if status == http.StatusNotFound {
			return // no robots.txt => allow all
		}
		data, err := robotstxt.FromBytes(body)
		if err != nil {
			r.err = fmt.Errorf("parse robots.txt for %q: %w", host, err)
			return
		}
		r.group = data.FindGroup(c.ua)
	})
	if r.err != nil {
		return r.err
	}
	if r.group != nil && !r.group.Test(path) {
		return fmt.Errorf("robots.txt disallows %q on %q", path, host)
	}
	return nil
}

func singleJoin(a, b string) string {
	switch {
	case a == "":
		return "/" + b
	case a[len(a)-1] == '/':
		return a + b
	default:
		return a + "/" + b
	}
}
