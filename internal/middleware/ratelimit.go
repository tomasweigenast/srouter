package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type ipBucket struct {
	count int
	reset time.Time
}

type ipRateLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	m      map[string]*ipBucket
}

// LoginLimiter returns a middleware that allows at most limit POST requests per
// window per source IP. Excess requests receive 429. Intended for the login
// route to mitigate credential brute-force.
func LoginLimiter(limit int, window time.Duration) func(http.Handler) http.Handler {
	rl := &ipRateLimiter{
		limit:  limit,
		window: window,
		m:      make(map[string]*ipBucket),
	}
	return rl.middleware
}

func (rl *ipRateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if !rl.allow(ip) {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (rl *ipRateLimiter) allow(ip string) bool {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.m[ip]
	if !ok || now.After(b.reset) {
		rl.m[ip] = &ipBucket{count: 1, reset: now.Add(rl.window)}
		return true
	}
	if b.count >= rl.limit {
		return false
	}
	b.count++
	return true
}
