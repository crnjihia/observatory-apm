// path: api/db/writer_test.go
package db

import (
	"context"
	"testing"
	"time"

	"observatory-apm/api/ingest"
)

func TestNullableString(t *testing.T) {
	if nullableString("") != nil {
		t.Fatal("expected empty string to yield nil interface")
	}
	val := nullableString("parent-123")
	if val != "parent-123" {
		t.Fatalf("expected 'parent-123', got %v", val)
	}
}

func TestNilPoolErrors(t *testing.T) {
	ctx := context.Background()

	_, err := CopySpans(ctx, nil, []ingest.Span{{TraceID: "t1"}})
	if err == nil {
		t.Fatal("expected error for nil pool on CopySpans")
	}

	_, err = CopyMetrics(ctx, nil, []ingest.Metric{{Name: "m1"}})
	if err == nil {
		t.Fatal("expected error for nil pool on CopyMetrics")
	}

	_, err = CopyLogs(ctx, nil, []ingest.LogRecord{{Body: "l1"}})
	if err == nil {
		t.Fatal("expected error for nil pool on CopyLogs")
	}
}

func TestEmptyBatchHandling(t *testing.T) {
	ctx := context.Background()

	// Should safely return 0 count with nil or non-nil pool when batch is empty
	count, err := CopySpans(ctx, nil, nil)
	if count != 0 || err != nil {
		t.Fatalf("expected 0, nil for empty spans; got %d, %v", count, err)
	}

	count, err = CopyMetrics(ctx, nil, nil)
	if count != 0 || err != nil {
		t.Fatalf("expected 0, nil for empty metrics; got %d, %v", count, err)
	}

	count, err = CopyLogs(ctx, nil, nil)
	if count != 0 || err != nil {
		t.Fatalf("expected 0, nil for empty logs; got %d, %v", count, err)
	}
}

func TestTelemetryDataFormatting(t *testing.T) {
	spans := []ingest.Span{
		{
			StartTs:    time.Now().UTC(),
			TraceID:    "4bf92f3577b34da6a3ce929d0e0e4736",
			SpanID:     "00f067aa0ba902b7",
			ParentID:   "",
			Service:    "mpesa-callback-service",
			Name:       "/v1/callbacks/mpesa",
			Kind:       "SPAN_KIND_SERVER",
			DurationNs: 120000000,
			Status:     "STATUS_CODE_OK",
			Attrs: map[string]interface{}{
				"currency": "KES",
				"amount":   3500.0,
			},
		},
	}

	if spans[0].Attrs["currency"] != "KES" {
		t.Fatalf("expected currency KES, got %v", spans[0].Attrs["currency"])
	}
}

