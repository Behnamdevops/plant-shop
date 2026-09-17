package operational

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func hitLimit(handler http.Handler, remote, forwarded string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	r.RemoteAddr = remote
	r.Header.Set("X-Forwarded-For", forwarded)
	r.Header.Set("X-Real-IP", forwarded)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestRateLimitIdentity(t *testing.T) {
	now := time.Unix(0, 0)
	trusted := func(_ context.Context, peer netip.Addr) bool { return peer.String() == "192.0.2.10" }
	h := rateLimit(2, time.Minute, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), trusted, func() time.Time { return now })
	for _, tc := range []struct {
		remote, forwarded string
		want              int
	}{
		{"192.0.2.1:1", "203.0.113.1", 200},
		{"192.0.2.1:2", "203.0.113.2", 200},
		{"192.0.2.1:3", "203.0.113.3", 429},
		{"[::ffff:192.0.2.1]:4", "", 429},
		{"192.0.2.2:1", "", 200},
		{"192.0.2.10:1", "203.0.113.1", 200},
		{"192.0.2.10:2", "203.0.113.1", 200},
		{"192.0.2.10:3", "203.0.113.1", 429},
		{"192.0.2.10:4", "203.0.113.2", 200},
		{"192.0.2.10:5", "invalid", 200},
		{"192.0.2.10:6", "203.0.113.1, 203.0.113.2", 200},
		{"192.0.2.10:7", "", 429},
	} {
		if w := hitLimit(h, tc.remote, tc.forwarded); w.Code != tc.want {
			t.Fatalf("remote=%s forwarded=%s status=%d want=%d", tc.remote, tc.forwarded, w.Code, tc.want)
		}
	}
}

func TestRateLimitDefaultsDoNotTrustHeaders(t *testing.T) {
	h := RateLimit(1, time.Minute, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	if hitLimit(h, "192.0.2.1:1", "203.0.113.1").Code != 200 || hitLimit(h, "192.0.2.1:2", "203.0.113.2").Code != 429 {
		t.Fatal("untrusted headers changed identity")
	}
}

func TestTrustedProxy(t *testing.T) {
	peer := netip.MustParseAddr("192.0.2.10")
	if TrustedProxy("")(t.Context(), peer) || TrustedProxy("192.0.2.11")(t.Context(), peer) {
		t.Fatal("unconfigured peer trusted")
	}
	if !TrustedProxy("192.0.2.10")(t.Context(), peer) {
		t.Fatal("explicit proxy not trusted")
	}
}

func TestRateLimitRefill(t *testing.T) {
	now := time.Unix(0, 0)
	h := rateLimit(2, 10*time.Second, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), nil, func() time.Time { return now })
	for range 2 {
		if hitLimit(h, "192.0.2.1:1", "").Code != 200 {
			t.Fatal("initial burst rejected")
		}
	}
	if w := hitLimit(h, "192.0.2.1:2", ""); w.Code != 429 || w.Header().Get("Retry-After") != "5" {
		t.Fatal("incorrect exhausted response", w)
	}
	now = now.Add(4500 * time.Millisecond)
	if w := hitLimit(h, "192.0.2.1:3", ""); w.Code != 429 || w.Header().Get("Retry-After") != "1" {
		t.Fatal("fractional token accepted", w)
	}
	now = now.Add(500 * time.Millisecond)
	if hitLimit(h, "192.0.2.1:4", "").Code != 200 || hitLimit(h, "192.0.2.1:5", "").Code != 429 {
		t.Fatal("one-token refill incorrect")
	}
	now = now.Add(time.Hour)
	for range 2 {
		if hitLimit(h, "192.0.2.1:6", "").Code != 200 {
			t.Fatal("idle bucket not restored")
		}
	}
	if hitLimit(h, "192.0.2.1:7", "").Code != 429 {
		t.Fatal("refill exceeded capacity")
	}
}

func TestRateLimitConcurrentAndBrowsing(t *testing.T) {
	now := time.Unix(0, 0)
	hit := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/auth/login", rateLimit(30, time.Minute, hit, nil, func() time.Time { return now }))
	mux.Handle("GET /api/v1/products", hit)
	var group sync.WaitGroup
	var allowed, blocked atomic.Int32
	for i := range 64 {
		group.Add(1)
		go func() {
			defer group.Done()
			switch hitLimit(mux, fmt.Sprintf("192.0.2.1:%d", i+1), "").Code {
			case 200:
				allowed.Add(1)
			case 429:
				blocked.Add(1)
			}
		}()
	}
	group.Wait()
	if allowed.Load() != 30 || blocked.Load() != 34 {
		t.Fatalf("allowed=%d blocked=%d", allowed.Load(), blocked.Load())
	}
	r := httptest.NewRequest("GET", "/api/v1/products", nil)
	r.RemoteAddr = "192.0.2.1:100"
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal("browsing throttled by login bucket")
	}
}
