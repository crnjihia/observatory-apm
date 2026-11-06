// path: api/middleware/ratelimit.go
package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type clientBucket struct {
	tokens     float64
	lastRefill time.Time
	lastAccess time.Time
}

// RateLimiter implements a thread-safe token bucket rate limiter per client.
type RateLimiter struct {
	mu           sync.Mutex
	buckets      map[string]*clientBucket
	ratePerSec   float64
	burst        float64
	stopCleaner  chan struct{}
}

// NewRateLimiter creates a RateLimiter instance configured for requests-per-minute (RPM).
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	if requestsPerMinute <= 0 {
		requestsPerMinute = 1000
	}

	ratePerSec := float64(requestsPerMinute) / 60.0
	burst := float64(requestsPerMinute) / 10.0 // allow 6s burst or minimum 10
	if burst < 10 {
		burst = float64(requestsPerMinute)
	}

	limiter := &RateLimiter{
		buckets:     make(map[string]*clientBucket),
		ratePerSec:  ratePerSec,
		burst:       burst,
		stopCleaner: make(chan struct{}),
	}

	go limiter.cleanupLoop()

	return limiter
}

// Allow checks if a request from the specified client is permitted under rate limit.
func (rl *RateLimiter) Allow(clientKey string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	bucket, exists := rl.buckets[clientKey]
	if !exists {
		rl.buckets[clientKey] = &clientBucket{
			tokens:     rl.burst - 1.0,
			lastRefill: now,
			lastAccess: now,
		}
		return true
	}

	// Refill tokens based on elapsed duration
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	bucket.tokens += elapsed * rl.ratePerSec
	if bucket.tokens > rl.burst {
		bucket.tokens = rl.burst
	}
	bucket.lastRefill = now
	bucket.lastAccess = now

	if bucket.tokens >= 1.0 {
		bucket.tokens -= 1.0
		return true
	}

	return false
}

// Handler returns an HTTP middleware enforcing the rate limit.
func (rl *RateLimiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		clientKey := extractClientKey(r)
		if !rl.Allow(clientKey) {
			w.Header().Set("Retry-After", "1")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":   "rate_limit_exceeded",
				"message": "rate limit exceeded, please retry later",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Close stops the background cleaner goroutine.
func (rl *RateLimiter) Close() {
	select {
	case <-rl.stopCleaner:
	default:
		close(rl.stopCleaner)
	}
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-rl.stopCleaner:
			return
		case <-ticker.C:
			rl.mu.Lock()
			cutoff := time.Now().Add(-5 * time.Minute)
			for key, b := range rl.buckets {
				if b.lastAccess.Before(cutoff) {
					delete(rl.buckets, key)
				}
			}
			rl.mu.Unlock()
		}
	}
}

func extractClientKey(r *http.Request) string {
	// Prioritize API key if provided
	if key := r.Header.Get("X-API-Key"); key != "" {
		return "key:" + key
	}

	// Check X-Forwarded-For
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return "ip:" + ip
			}
		}
	}

	// Check X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return "ip:" + strings.TrimSpace(xri)
	}

	// Fallback to RemoteAddr host
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return "ip:" + host
	}

	return "ip:" + r.RemoteAddr
}

