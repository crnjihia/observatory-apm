// path: packages/agent-go/observatory_test.go
package observatory

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInitValidation(t *testing.T) {
	_, err := Init(Config{})
	if err == nil {
		t.Fatal("expected error when initializing agent without service name")
	}

	client, err := Init(Config{
		Service:  "daraja-b2c-gateway",
		Endpoint: "http://localhost:8080",
	})
	if err != nil {
		t.Fatalf("failed to initialize client: %v", err)
	}
	defer client.Close()

	if client.cfg.Service != "daraja-b2c-gateway" {
		t.Fatalf("expected service 'daraja-b2c-gateway', got %s", client.cfg.Service)
	}
}

func TestStartSpanContextPropagation(t *testing.T) {
	client, err := Init(Config{
		Service: "mpesa-callback-service",
	})
	if err != nil {
		t.Fatalf("unexpected init error: %v", err)
	}
	defer client.Close()

	// Root span
	rootCtx, rootSpan := client.StartSpan(context.Background(), "HTTP POST /v1/callbacks/mpesa")
	if rootSpan.TraceID == "" || rootSpan.SpanID == "" {
		t.Fatal("expected non-empty trace and span IDs")
	}
	if rootSpan.ParentID != "" {
		t.Fatalf("root span parent ID must be empty, got %s", rootSpan.ParentID)
	}

	// Child span
	_, childSpan := client.StartSpan(rootCtx, "crypto.verify_signature")
	if childSpan.TraceID != rootSpan.TraceID {
		t.Fatalf("child span must inherit root trace ID %s, got %s", rootSpan.TraceID, childSpan.TraceID)
	}
	if childSpan.ParentID != rootSpan.SpanID {
		t.Fatalf("child span parent ID must equal root span ID %s, got %s", rootSpan.SpanID, childSpan.ParentID)
	}
}

func TestHTTPMiddleware(t *testing.T) {
	client, err := Init(Config{
		Service: "ussd-menu-session",
	})
	if err != nil {
		t.Fatalf("unexpected init error: %v", err)
	}
	defer client.Close()

	handler := client.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tc, ok := FromContext(r.Context())
		if !ok || tc.TraceID == "" {
			t.Fatal("expected valid trace context in HTTP request context")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("CON Welcome to M-Pesa USSD"))
	}))

	req := httptest.NewRequest("POST", "/ussd/entry", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	traceparent := w.Header().Get("traceparent")
	if traceparent == "" {
		t.Fatal("expected traceparent header in HTTP response")
	}
}

func TestSlogTraceCorrelation(t *testing.T) {
	buf := new(bytes.Buffer)
	jsonHandler := slog.NewJSONHandler(buf, nil)
	logger := NewCorrelatedLogger(jsonHandler)

	ctx := context.Background()
	traceID := "54eba8423f6d4350ba3291c0bfd51e11"
	spanID := "91c0bfd51e110001"
	ctx = WithContext(ctx, TraceContext{TraceID: traceID, SpanID: spanID})

	logger.InfoContext(ctx, "M-Pesa callback received", "phone", "+254711000001", "amount", 1500)

	var logged map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logged); err != nil {
		t.Fatalf("failed to parse structured log json: %v", err)
	}

	if logged["trace_id"] != traceID {
		t.Fatalf("expected injected trace_id %s, got %v", traceID, logged["trace_id"])
	}
	if logged["span_id"] != spanID {
		t.Fatalf("expected injected span_id %s, got %v", spanID, logged["span_id"])
	}
	if logged["msg"] != "M-Pesa callback received" {
		t.Fatalf("expected msg 'M-Pesa callback received', got %v", logged["msg"])
	}
}

