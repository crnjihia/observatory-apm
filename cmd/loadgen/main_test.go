// path: cmd/loadgen/main_test.go
package main

import (
	"encoding/json"
	"testing"
)

func TestGenerateSyntheticSpans(t *testing.T) {
	count := 50
	data := GenerateSyntheticSpans(count)
	if len(data) == 0 {
		t.Fatal("expected non-empty json payload")
	}

	var parsed []SyntheticSpan
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal generated json: %v", err)
	}

	if len(parsed) != count {
		t.Fatalf("expected %d spans, got %d", count, len(parsed))
	}

	for i, s := range parsed {
		if len(s.TraceID) != 32 {
			t.Fatalf("span %d has invalid trace ID length: %d", i, len(s.TraceID))
		}
		if len(s.SpanID) != 16 {
			t.Fatalf("span %d has invalid span ID length: %d", i, len(s.SpanID))
		}
		if s.Service == "" || s.Name == "" {
			t.Fatalf("span %d missing service or endpoint name", i)
		}
		if s.DurationNs <= 0 {
			t.Fatalf("span %d has invalid duration: %d", i, s.DurationNs)
		}
	}
}

