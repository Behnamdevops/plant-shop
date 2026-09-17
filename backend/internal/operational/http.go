package operational

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
)

func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte(`{"status":"ok"}`))
}

func Ready(ping func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if err := ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"unavailable"}`))
			return
		}
		w.Write([]byte(`{"status":"ok"}`))
	}
}

type response struct {
	http.ResponseWriter
	status int
}

func (w *response) WriteHeader(status int) {
	if status >= 100 && status < 200 {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *response) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}
func (w *response) Unwrap() http.ResponseWriter { return w.ResponseWriter }

var safeID = regexp.MustCompile(`^[a-fA-F0-9]{32}$`)

func Logging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		id := r.Header.Get("X-Request-ID")
		if !safeID.MatchString(id) {
			var bytes [16]byte
			if _, err := rand.Read(bytes[:]); err != nil {
				http.Error(w, "unavailable", http.StatusServiceUnavailable)
				return
			}
			id = hex.EncodeToString(bytes[:])
		}
		w.Header().Set("X-Request-ID", id)
		recorded := &response{ResponseWriter: w}
		defer func() {
			panicValue := recover()
			if panicValue != nil {
				if recorded.status == 0 {
					http.Error(recorded, "internal server error", http.StatusInternalServerError)
				}
			}
			if recorded.status == 0 {
				recorded.status = http.StatusOK
			}
			if r.URL.Path == "/health" || r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
				if panicValue == nil {
					return
				}
			}
			path := r.Pattern
			if _, matched, ok := strings.Cut(path, " "); ok {
				path = matched
			}
			if path == "" {
				path = "unmatched"
			}
			peer, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				peer = "unknown"
			}
			logger.Info("http_request", "method", r.Method, "path", path, "status", recorded.status, "duration_ms", time.Since(started).Milliseconds(), "request_id", id, "peer_ip", peer, "panic", panicValue != nil)
		}()
		next.ServeHTTP(recorded, r)
	})
}
