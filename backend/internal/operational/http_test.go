package operational

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHealth(t *testing.T) {
	w := httptest.NewRecorder()
	Health(w, httptest.NewRequest("GET", "/healthz", nil))
	if w.Code != 200 || w.Body.String() != `{"status":"ok"}` || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w)
	}
}

func TestReadiness(t *testing.T) {
	for _, fail := range []bool{false, true} {
		w := httptest.NewRecorder()
		Ready(func(ctx context.Context) error {
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) > time.Second {
				t.Fatal("unbounded readiness")
			}
			if fail {
				return errors.New("secret database URL")
			}
			return nil
		})(w, httptest.NewRequest("GET", "/readyz", nil))
		want := 200
		if fail {
			want = 503
		}
		if w.Code != want || strings.Contains(w.Body.String(), "secret") {
			t.Fatal(w)
		}
	}
	w := httptest.NewRecorder()
	start := time.Now()
	Ready(func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() })(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Code != 503 || time.Since(start) > 2*time.Second {
		t.Fatal("readiness timeout")
	}
}

func TestLogging(t *testing.T) {
	for _, id := range []string{"", "untrusted-secret", "0123456789abcdef0123456789abcdef"} {
		var output bytes.Buffer
		mux := http.NewServeMux()
		mux.HandleFunc("POST /api/{id}", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(201)
			w.WriteHeader(500)
			w.Write([]byte("ok"))
		})
		request := httptest.NewRequest("POST", "/api/customer-secret?Authority=payment-secret", strings.NewReader("password-secret"))
		request.Header.Set("X-Request-ID", id)
		request.Header.Set("Authorization", "Bearer auth-secret")
		request.Header.Set("Cookie", "session=cookie-secret")
		request.Header.Set("X-Forwarded-For", "spoofed-secret")
		request.RemoteAddr = "192.0.2.3:4567"
		w := httptest.NewRecorder()
		Logging(slog.New(slog.NewJSONHandler(&output, nil)), mux).ServeHTTP(w, request)
		var event map[string]any
		if err := json.Unmarshal(output.Bytes(), &event); err != nil {
			t.Fatal(err)
		}
		if event["path"] != "/api/{id}" || event["status"] != float64(201) || event["peer_ip"] != "192.0.2.3" || event["method"] != "POST" {
			t.Fatal(event)
		}
		if strings.Contains(output.String(), "secret") {
			t.Fatal("sensitive information logged")
		}
		if !safeID.MatchString(w.Header().Get("X-Request-ID")) || event["request_id"] != w.Header().Get("X-Request-ID") {
			t.Fatal("missing request ID")
		}
		if safeID.MatchString(id) && id != w.Header().Get("X-Request-ID") {
			t.Fatal("safe ID not propagated")
		}
	}
}

func TestLoggingHealthAndPanic(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	for _, path := range []string{"/health", "/healthz", "/readyz"} {
		Logging(logger, http.HandlerFunc(Health)).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", path, nil))
	}
	if output.Len() != 0 {
		t.Fatal("noisy health logging")
	}
	w := httptest.NewRecorder()
	Logging(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { panic("secret panic") })).ServeHTTP(w, httptest.NewRequest("GET", "/unknown-secret", nil))
	if w.Code != 500 || strings.Contains(output.String(), "secret") || strings.Contains(w.Body.String(), "secret") {
		t.Fatal("unsafe panic logging")
	}
}
