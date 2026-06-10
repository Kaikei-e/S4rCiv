package kokkaihttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// newTestServer serves robots.txt as 404 (allow all) plus the given handlers, and
// records the arrival time of every request so interval gating can be asserted.
func newTestServer(t *testing.T, handlers map[string]http.HandlerFunc) (*httptest.Server, func() []time.Time) {
	t.Helper()
	var mu sync.Mutex
	var times []time.Time
	mux := http.NewServeMux()
	for pattern, h := range handlers {
		mux.HandleFunc(pattern, h)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		times = append(times, time.Now())
		mu.Unlock()
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, func() []time.Time {
		mu.Lock()
		defer mu.Unlock()
		return append([]time.Time(nil), times...)
	}
}

// A redirect within the source host is followed and the final body returned.
func TestGetFollowsRedirectWithinSourceHost(t *testing.T) {
	srv, _ := newTestServer(t, map[string]http.HandlerFunc{
		"/hop": func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/target", http.StatusFound)
		},
		"/target": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("ok"))
		},
	})
	c, err := New(srv.URL, "test-agent", 0)
	if err != nil {
		t.Fatal(err)
	}
	body, status, err := c.Get(context.Background(), "hop", nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if status != 200 || string(body) != "ok" {
		t.Fatalf("status=%d body=%q, want 200 \"ok\"", status, body)
	}
}

// A redirect off the source host must be refused: the SSRF allowlist applies to
// every hop, not just the initial URL. The off-host target is never dialed
// (example.invalid would not resolve anyway).
func TestGetRefusesRedirectOffSourceHost(t *testing.T) {
	srv, _ := newTestServer(t, map[string]http.HandlerFunc{
		"/hop": func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://attacker.example.invalid/exfil", http.StatusFound)
		},
	})
	c, err := New(srv.URL, "test-agent", 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = c.Get(context.Background(), "hop", nil)
	if err == nil || !strings.Contains(err.Error(), "off the source host") {
		t.Fatalf("redirect off the source host must be refused, got err=%v", err)
	}
}

// A redirect carrying userinfo must be refused even on the source host.
func TestGetRefusesRedirectWithUserinfo(t *testing.T) {
	var hopTo string
	srv, _ := newTestServer(t, map[string]http.HandlerFunc{
		"/hop": func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, hopTo, http.StatusFound)
		},
	})
	hopTo = "http://user:secret@" + strings.TrimPrefix(srv.URL, "http://") + "/target"
	c, err := New(srv.URL, "test-agent", 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = c.Get(context.Background(), "hop", nil)
	if err == nil || !strings.Contains(err.Error(), "userinfo") {
		t.Fatalf("redirect with userinfo must be refused, got err=%v", err)
	}
}

// The robots.txt fetch consumes an interval slot: first contact must not send the
// robots GET and the payload GET back-to-back (DISCIPLINE §1 serial access).
func TestRobotsFetchConsumesIntervalSlot(t *testing.T) {
	srv, requestTimes := newTestServer(t, map[string]http.HandlerFunc{
		"/target": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("ok"))
		},
	})
	const interval = 150 * time.Millisecond
	c, err := New(srv.URL, "test-agent", interval)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.Get(context.Background(), "target", nil); err != nil {
		t.Fatalf("Get: %v", err)
	}
	times := requestTimes()
	if len(times) != 2 {
		t.Fatalf("requests = %d, want 2 (robots.txt + target)", len(times))
	}
	if gap := times[1].Sub(times[0]); gap < interval {
		t.Fatalf("gap between robots.txt and target = %v, want >= %v", gap, interval)
	}
}
