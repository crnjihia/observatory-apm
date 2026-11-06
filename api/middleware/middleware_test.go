// path: api/middleware/middleware_test.go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddleware(t *testing.T) {
	apiKey := "test_secret_key_123"
	auth := NewAuth(apiKey, true)

	handler := auth.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	t.Run("Exempt health route without key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 on /health, got %d", rec.Code)
		}
	})

	t.Run("Exempt metrics route without key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 on /metrics, got %d", rec.Code)
		}
	})

	t.Run("OPTIONS preflight allowed without key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/v1/traces", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 on OPTIONS, got %d", rec.Code)
		}
	})

	t.Run("Missing key on protected route returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/traces", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("Invalid key on protected route returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/traces", nil)
		req.Header.Set("X-API-Key", "wrong-key")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("Valid X-API-Key returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/traces", nil)
		req.Header.Set("X-API-Key", apiKey)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("Valid Bearer token returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/traces", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})
}

func TestRateLimiter(t *testing.T) {
	// Limiter with low burst for testing: 60 RPM (1 token/sec), burst 2
	limiter := &RateLimiter{
		buckets:     make(map[string]*clientBucket),
		ratePerSec:  1.0,
		burst:       2.0,
		stopCleaner: make(chan struct{}),
	}
	defer limiter.Close()

	handler := limiter.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	// 1st request should succeed (burst starts at 2 - 1 = 1)
	req1 := httptest.NewRequest(http.MethodGet, "/services", nil)
	req1.RemoteAddr = "192.168.1.100:1234"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200 on 1st request, got %d", rec1.Code)
	}

	// 2nd request should succeed (tokens drops from 1 to 0)
	req2 := httptest.NewRequest(http.MethodGet, "/services", nil)
	req2.RemoteAddr = "192.168.1.100:1234"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 on 2nd request, got %d", rec2.Code)
	}

	// 3rd immediate request should be rate-limited (429)
	req3 := httptest.NewRequest(http.MethodGet, "/services", nil)
	req3.RemoteAddr = "192.168.1.100:1234"
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on 3rd request, got %d", rec3.Code)
	}
	if rec3.Header().Get("Retry-After") != "1" {
		t.Fatalf("expected Retry-After header: 1, got %q", rec3.Header().Get("Retry-After"))
	}
}

