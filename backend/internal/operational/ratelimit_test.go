package operational

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRateLimit(t *testing.T) {
	request := func(handler http.Handler, remote string) *httptest.ResponseRecorder {
		request := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader("{}"))
		request.RemoteAddr = remote
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, request)
		return w
	}
	hit := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	limited := RateLimit(1, time.Minute, hit)
	if w := request(limited, "192.0.2.1:1"); w.Code != 200 || w.Header().Get("Retry-After") != "" {
		t.Fatal(w)
	}
	if w := request(limited, "192.0.2.1:1"); w.Code != 429 || w.Header().Get("Retry-After") != "60" {
		t.Fatal(w)
	}
	if w := request(limited, "192.0.2.2:1"); w.Code != 200 {
		t.Fatal("remote clients share a bucket")
	}
	time.Sleep(1100 * time.Millisecond)
	if w := request(RateLimit(60, time.Minute, hit), "192.0.2.3:1"); w.Code != 200 {
		t.Fatal("refill not time based")
	}
	var group sync.WaitGroup
	concurrent := RateLimit(50, time.Minute, hit)
	for i := 0; i < 100; i++ {
		group.Add(1)
		go func() { defer group.Done(); request(concurrent, "192.0.2.4:1") }()
	}
	group.Wait()
	allowed, blocked := 0, 0
	for i := 0; i < 100; i++ {
		if code := request(concurrent, "192.0.2.5:1").Code; code == 200 {
			allowed++
		} else if code == 429 {
			blocked++
		}
	}
	if allowed != 50 || blocked != 50 {
		t.Fatal("bucket accounting")
	}
}
