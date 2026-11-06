// path: api/sampling/head.go
package sampling

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"math"
	"strings"
)

// Decision defines the outcome of a sampling evaluation.
type Decision int

const (
	// DecisionDrop indicates the span/trace should be discarded.
	DecisionDrop Decision = iota
	// DecisionRecordOnly indicates the span is recorded for metrics but not forwarded for distributed trace tracing.
	DecisionRecordOnly
	// DecisionRecordAndSample indicates the span/trace should be sampled and retained.
	DecisionRecordAndSample
)

func (d Decision) String() string {
	switch d {
	case DecisionDrop:
		return "DROP"
	case DecisionRecordOnly:
		return "RECORD_ONLY"
	case DecisionRecordAndSample:
		return "RECORD_AND_SAMPLE"
	default:
		return fmt.Sprintf("Decision(%d)", d)
	}
}

// HeadSampler implements OpenTelemetry-compliant head-based ratio sampling
// and parent-based propagation (ParentBased(TraceIdRatioBased(ratio))).
type HeadSampler struct {
	ratio     float64
	threshold uint64
}

// NewHeadSampler constructs a HeadSampler with the given sampling ratio in [0.0, 1.0].
// A ratio of 0.1 represents 10% head-based sampling.
func NewHeadSampler(ratio float64) *HeadSampler {
	if ratio < 0.0 {
		ratio = 0.0
	}
	if ratio > 1.0 {
		ratio = 1.0
	}

	var threshold uint64
	if ratio >= 1.0 {
		threshold = math.MaxUint64
	} else if ratio <= 0.0 {
		threshold = 0
	} else {
		threshold = uint64(ratio * float64(math.MaxUint64))
	}

	return &HeadSampler{
		ratio:     ratio,
		threshold: threshold,
	}
}

// Ratio returns the configured sampling ratio.
func (s *HeadSampler) Ratio() float64 {
	return s.ratio
}

// TraceIdRatioDecision evaluates deterministic ratio sampling for a 128-bit trace ID.
// Following the OpenTelemetry specification, the lowest 64 bits (last 16 hex chars)
// are interpreted as a big-endian uint64 and compared against the threshold.
func (s *HeadSampler) TraceIdRatioDecision(traceID string) Decision {
	if s.ratio >= 1.0 {
		return DecisionRecordAndSample
	}
	if s.ratio <= 0.0 {
		return DecisionDrop
	}

	val := extractTraceIDUint64(traceID)
	if val < s.threshold {
		return DecisionRecordAndSample
	}
	return DecisionDrop
}

// ShouldSample implements ParentBased(TraceIdRatioBased) semantics.
// - If no parent (root span): evaluates TraceIdRatioBased.
// - If has parent and parent was sampled: returns DecisionRecordAndSample.
// - If has parent and parent was not sampled: returns DecisionDrop.
func (s *HeadSampler) ShouldSample(traceID string, hasParent bool, parentSampled bool) Decision {
	if !hasParent {
		return s.TraceIdRatioDecision(traceID)
	}

	if parentSampled {
		return DecisionRecordAndSample
	}
	return DecisionDrop
}

// extractTraceIDUint64 extracts the lower 64 bits of a trace ID as an unsigned integer.
// If the string is a valid 32-character hex string, the last 16 characters are decoded.
// If parsing fails or the string has a non-standard length, FNV-1a 64-bit hash is used as fallback.
func extractTraceIDUint64(traceID string) uint64 {
	clean := strings.TrimPrefix(traceID, "0x")
	clean = strings.ReplaceAll(clean, "-", "")

	if len(clean) >= 16 {
		// Take the last 16 hex characters (64 bits)
		last16 := clean[len(clean)-16:]
		b, err := hex.DecodeString(last16)
		if err == nil && len(b) == 8 {
			return binary.BigEndian.Uint64(b)
		}
	}

	// Fallback to FNV-1a 64-bit hash
	h := fnv.New64a()
	_, _ = h.Write([]byte(traceID))
	return h.Sum64()
}

// ParseTraceparent parses a W3C traceparent header:
// e.g., "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
// Returns traceID, spanID, sampled boolean, and valid boolean.
func ParseTraceparent(header string) (traceID string, spanID string, sampled bool, ok bool) {
	parts := strings.Split(strings.TrimSpace(header), "-")
	if len(parts) != 4 {
		return "", "", false, false
	}

	version := parts[0]
	traceID = parts[1]
	spanID = parts[2]
	flagsHex := parts[3]

	if version != "00" || len(traceID) != 32 || len(spanID) != 16 || len(flagsHex) != 2 {
		return "", "", false, false
	}

	flags, err := hex.DecodeString(flagsHex)
	if err != nil || len(flags) != 1 {
		return "", "", false, false
	}

	// Bit 0 of flags byte indicates sampled (0x01)
	sampled = (flags[0] & 0x01) != 0
	return traceID, spanID, sampled, true
}

