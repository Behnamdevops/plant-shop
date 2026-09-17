package operational

import (
	"context"
	"math"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
	"time"
)

type ProxyTrust func(context.Context, netip.Addr) bool

func TrustedProxy(host string) ProxyTrust {
	return func(ctx context.Context, peer netip.Addr) bool {
		if host == "" {
			return false
		}
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return false
		}
		for _, address := range addresses {
			if address.Unmap() == peer.Unmap() {
				return true
			}
		}
		return false
	}
}

func rateLimitIdentity(r *http.Request, trusted ProxyTrust) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return "unknown"
	}
	peer = peer.Unmap()
	if trusted != nil && trusted(r.Context(), peer) {
		if forwarded, err := netip.ParseAddr(r.Header.Get("X-Real-IP")); err == nil {
			return forwarded.Unmap().String()
		}
	}
	return peer.String()
}

func RateLimit(limit int, window time.Duration, next http.Handler, trusted ...ProxyTrust) http.Handler {
	var proxy ProxyTrust
	if len(trusted) > 0 {
		proxy = trusted[0]
	}
	return rateLimit(limit, window, next, proxy, time.Now)
}

func rateLimit(limit int, window time.Duration, next http.Handler, trusted ProxyTrust, clock func() time.Time) http.Handler {
	if limit <= 0 || window <= 0 {
		panic("rate limit and window must be positive")
	}
	type bucket struct {
		tokens  float64
		updated time.Time
	}
	var mutex sync.Mutex
	buckets := map[string]*bucket{}
	fill := float64(limit)
	rate := fill / window.Seconds()
	lastCleanup := clock()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := rateLimitIdentity(r, trusted)
		mutex.Lock()
		now := clock()
		if now.Sub(lastCleanup) >= window {
			for key, bucket := range buckets {
				if now.Sub(bucket.updated) >= window {
					delete(buckets, key)
				}
			}
			lastCleanup = now
		}
		current, ok := buckets[key]
		if !ok {
			if len(buckets) >= 10_000 {
				mutex.Unlock()
				w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(window.Seconds()))))
				http.Error(w, "too many requests", http.StatusTooManyRequests)
				return
			}
			current = &bucket{tokens: fill, updated: now}
			buckets[key] = current
		}
		if elapsed := now.Sub(current.updated).Seconds(); elapsed > 0 {
			current.tokens = math.Min(fill, current.tokens+elapsed*rate)
			current.updated = now
		}
		if current.tokens < 1 {
			retry := int(math.Ceil((1 - current.tokens) / rate))
			mutex.Unlock()
			w.Header().Set("Retry-After", strconv.Itoa(retry))
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		current.tokens--
		mutex.Unlock()
		next.ServeHTTP(w, r)
	})
}
