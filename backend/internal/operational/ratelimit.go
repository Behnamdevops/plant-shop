package operational

import (
	"net/http"
	"sync"
	"time"
)

func RateLimit(perMinute int, window time.Duration, next http.Handler) http.Handler {
	type bucket struct {
		tokens  float64
		updated time.Time
	}
	var mutex sync.Mutex
	buckets := map[string]*bucket{}
	fill := float64(perMinute)
	rate := fill / float64(time.Minute)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.RemoteAddr
		now := time.Now()
		mutex.Lock()
		current, ok := buckets[key]
		if !ok {
			if len(buckets) > 10_000 {
				mutex.Unlock()
				http.Error(w, "too many requests", http.StatusTooManyRequests)
				return
			}
			current = &bucket{tokens: fill, updated: now}
			buckets[key] = current
		}
		if ok {
			current.tokens += now.Sub(current.updated).Seconds() * rate
			if current.tokens > fill {
				current.tokens = fill
			}
		}
		current.updated = now
		if current.tokens < 1 {
			mutex.Unlock()
			w.Header().Set("Retry-After", "60")
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		current.tokens--
		mutex.Unlock()
		next.ServeHTTP(w, r)
	})
}
