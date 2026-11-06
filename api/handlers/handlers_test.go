// path: api/handlers/handlers_test.go
package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

func TestGetServicesValidation(t *testing.T) {
	handler := GetServices(nil)
	req := httptest.NewRequest("GET", "/services", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", w.Code)
	}

	var services []string
	if err := json.NewDecoder(w.Body).Decode(&services); err != nil {
		t.Fatalf("failed to decode services response: %v", err)
	}

	if len(services) == 0 {
		t.Fatal("expected at least mock services list when pool is nil")
	}
}

func TestGetLatencyValidation(t *testing.T) {
	handler := GetLatency(nil)

	// 1. Missing service
	req := httptest.NewRequest("GET", "/latency", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 for missing service, got %d", w.Code)
	}

	// 2. Missing start/end
	req = httptest.NewRequest("GET", "/latency?service=mpesa-callback-service", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 for missing timestamps, got %d", w.Code)
	}

	// 3. Invalid timestamp
	req = httptest.NewRequest("GET", "/latency?service=mpesa-callback-service&start=bad-time&end=bad-time", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 for invalid timestamp, got %d", w.Code)
	}

	// 4. Start after end
	start := time.Now().UTC()
	end := start.Add(-1 * time.Hour)
	req = httptest.NewRequest(
		"GET",
		"/latency?service=mpesa-callback-service&start="+start.Format(time.RFC3339)+"&end="+end.Format(time.RFC3339),
		nil,
	)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 when start > end, got %d", w.Code)
	}

	// 5. Valid request (nil pool returns empty list)
	validStart := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
	validEnd := time.Now().UTC().Format(time.RFC3339)
	req = httptest.NewRequest(
		"GET",
		"/latency?service=mpesa-callback-service&start="+validStart+"&end="+validEnd,
		nil,
	)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 for valid latency query, got %d", w.Code)
	}
}

func TestGetHeatmapValidation(t *testing.T) {
	handler := GetHeatmapBuckets(nil)

	// Missing service
	req := httptest.NewRequest("GET", "/heatmap/buckets", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 for missing service, got %d", w.Code)
	}

	// Valid parameters
	validStart := time.Now().Add(-30 * time.Minute).UTC().Format(time.RFC3339)
	validEnd := time.Now().UTC().Format(time.RFC3339)
	req = httptest.NewRequest(
		"GET",
		"/heatmap/buckets?service=daraja-b2c-gateway&start="+validStart+"&end="+validEnd,
		nil,
	)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 for heatmap request, got %d", w.Code)
	}

	var resp HeatmapResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode heatmap response: %v", err)
	}
	if resp.Service != "daraja-b2c-gateway" {
		t.Fatalf("expected service 'daraja-b2c-gateway', got %s", resp.Service)
	}
}

func TestGetErrorsHandler(t *testing.T) {
	handler := GetErrors(nil)
	req := httptest.NewRequest("GET", "/errors", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 for /errors, got %d", w.Code)
	}

	var summary ErrorSummaryResponse
	if err := json.NewDecoder(w.Body).Decode(&summary); err != nil {
		t.Fatalf("failed to decode error summary: %v", err)
	}
}

func TestGetTraceSpansRouting(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/trace/{id}", GetTraceSpans(nil))

	req := httptest.NewRequest("GET", "/trace/54eba8423f6d4350ba3291c0bfd51e11", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// With nil pool, returns 503 database unavailable
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected HTTP 503 for nil pool, got %d", w.Code)
	}
}

