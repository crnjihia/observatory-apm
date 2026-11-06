// path: api/middleware/auth.go
package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
)

// AuthMiddleware validates API keys for incoming requests.
type AuthMiddleware struct {
	expectedKey string
	enabled     bool
	exemptPaths map[string]struct{}
}

// NewAuth creates an AuthMiddleware instance.
func NewAuth(expectedKey string, enabled bool, exemptPaths ...string) *AuthMiddleware {
	exemptMap := make(map[string]struct{}, len(exemptPaths)+2)
	exemptMap["/health"] = struct{}{}
	exemptMap["/metrics"] = struct{}{}
	for _, p := range exemptPaths {
		exemptMap[p] = struct{}{}
	}

	return &AuthMiddleware{
		expectedKey: expectedKey,
		enabled:     enabled,
		exemptPaths: exemptMap,
	}
}

// Handler returns an HTTP middleware function.
func (a *AuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Preflight CORS requests are always allowed
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Exempt paths bypass authentication
		if _, exempt := a.exemptPaths[r.URL.Path]; exempt {
			next.ServeHTTP(w, r)
			return
		}

		// If auth is disabled globally
		if !a.enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Extract API key from X-API-Key header or Authorization: Bearer <key>
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				apiKey = strings.TrimSpace(authHeader[7:])
			}
		}

		// Verify API key matches
		if apiKey == "" || apiKey != a.expectedKey {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":   "unauthorized",
				"message": "invalid or missing API key",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

