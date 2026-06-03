// Package kokkaihttp is the read-only HTTP-GET boundary to the kokkai
// (国会会議録検索API). It enforces the DISCIPLINE §1 obligations in one place:
// serial access with a per-source interval, an identifying User-Agent, and
// robots.txt compliance. It only ever issues GET (DISCIPLINE §2).
package kokkaihttp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/temoto/robotstxt"
)

// Client serializes and spaces requests to one source. Construct one per source.
type Client struct {
	base     *url.URL
	ua       string
	interval time.Duration
	http     *http.Client

	mu   sync.Mutex // serializes requests (no parallel/burst access)
	next time.Time  // earliest time the next request may go out

	robotsOnce sync.Once
	robotsErr  error
	robots     *robotstxt.Group
}

func New(baseURL, userAgent string, interval time.Duration) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}
	return &Client{
		base:     u,
		ua:       userAgent,
		interval: interval,
		http:     &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Get fetches base + "/" + endpoint with the given query. It blocks until the
// per-source interval has elapsed since the previous request. Returns the body
// and HTTP status; a 404 is returned to the caller (not an error) so a vanished
// resource can be recorded.
func (c *Client) Get(ctx context.Context, endpoint string, q url.Values) ([]byte, int, error) {
	target := *c.base
	target.Path = singleJoin(c.base.Path, endpoint)
	target.RawQuery = q.Encode()

	if err := c.checkRobots(ctx, target.Path); err != nil {
		return nil, 0, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if wait := time.Until(c.next); wait > 0 {
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case <-time.After(wait):
		}
	}

	body, status, err := c.do(ctx, target.String())
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

// checkRobots lazily fetches and caches robots.txt for the host and rejects a
// disallowed path. A missing robots.txt is treated as "allow all".
func (c *Client) checkRobots(ctx context.Context, path string) error {
	c.robotsOnce.Do(func() {
		robotsURL := c.base.Scheme + "://" + c.base.Host + "/robots.txt"
		body, status, err := c.do(ctx, robotsURL)
		if err != nil {
			c.robotsErr = fmt.Errorf("fetch robots.txt: %w", err)
			return
		}
		if status == http.StatusNotFound {
			return // no robots.txt => allow all
		}
		data, err := robotstxt.FromBytes(body)
		if err != nil {
			c.robotsErr = fmt.Errorf("parse robots.txt: %w", err)
			return
		}
		c.robots = data.FindGroup(c.ua)
	})
	if c.robotsErr != nil {
		return c.robotsErr
	}
	if c.robots != nil && !c.robots.Test(path) {
		return fmt.Errorf("robots.txt disallows %s", path)
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
