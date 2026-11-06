// path: cmd/loadgen/main.go
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type Config struct {
	Endpoint  string
	Rate      int
	BatchSize int
	Workers   int
	Duration  time.Duration
}

type Stats struct {
	TotalRequests int64
	TotalPoints   int64
	Success200    int64
	Rejected429   int64
	Errors        int64
	LatenciesMu   sync.Mutex
	Latencies     []time.Duration
}

func main() {
	endpoint := flag.String("endpoint", getEnv("TARGET_ENDPOINT", "http://localhost:8080"), "Observatory Ingest API endpoint")
	rate := flag.Int("rate", 10000, "Target points per second")
	batchSize := flag.Int("batch-size", 100, "Number of points per batch request")
	workers := flag.Int("workers", 16, "Number of concurrent sender workers")
	duration := flag.Duration("duration", 10*time.Second, "Test run duration")
	flag.Parse()

	cfg := Config{
		Endpoint:  *endpoint,
		Rate:      *rate,
		BatchSize: *batchSize,
		Workers:   *workers,
		Duration:  *duration,
	}

	fmt.Println("================================================================================")
	fmt.Printf("🚀 Observatory APM High-Throughput OTLP Ingest Benchmark\n")
	fmt.Printf("Target Endpoint : %s\n", cfg.Endpoint)
	fmt.Printf("Target Rate     : %d points/sec\n", cfg.Rate)
	fmt.Printf("Batch Size      : %d points/request\n", cfg.BatchSize)
	fmt.Printf("Sender Workers  : %d\n", cfg.Workers)
	fmt.Printf("Duration        : %v\n", cfg.Duration)
	fmt.Println("================================================================================")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	stats := &Stats{
		Latencies: make([]time.Duration, 0, 50000),
	}

	transport := &http.Transport{
		MaxIdleConns:        500,
		MaxIdleConnsPerHost: 200,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression: true,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}

	// Calculate batch rate (requests per second across all workers)
	batchesPerSec := int(math.Ceil(float64(cfg.Rate) / float64(cfg.BatchSize)))
	if batchesPerSec < 1 {
		batchesPerSec = 1
	}

	batchQueue := make(chan []byte, batchesPerSec*2)
	var wg sync.WaitGroup

	// Generator routine: feeds pre-encoded JSON batches at target rate
	go func() {
		ticker := time.NewTicker(time.Second / time.Duration(batchesPerSec))
		defer ticker.Stop()
		defer close(batchQueue)

		startTime := time.Now()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if time.Since(startTime) >= cfg.Duration {
					return
				}
				batchData := GenerateSyntheticSpans(cfg.BatchSize)
				select {
				case batchQueue <- batchData:
				default:
					// Rate dropped if queue full
				}
			}
		}
	}()

	// Worker routines: transmit batches to endpoint
	benchStart := time.Now()
	for i := 0; i < cfg.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			url := fmt.Sprintf("%s/v1/traces", cfg.Endpoint)
			for data := range batchQueue {
				reqStart := time.Now()
				req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
				if err != nil {
					atomic.AddInt64(&stats.Errors, 1)
					continue
				}
				req.Header.Set("Content-Type", "application/json")

				resp, err := client.Do(req)
				elapsed := time.Since(reqStart)

				atomic.AddInt64(&stats.TotalRequests, 1)
				atomic.AddInt64(&stats.TotalPoints, int64(cfg.BatchSize))

				stats.LatenciesMu.Lock()
				if len(stats.Latencies) < 100000 {
					stats.Latencies = append(stats.Latencies, elapsed)
				}
				stats.LatenciesMu.Unlock()

				if err != nil {
					atomic.AddInt64(&stats.Errors, 1)
					continue
				}
				_ = resp.Body.Close()

				if resp.StatusCode == http.StatusOK {
					atomic.AddInt64(&stats.Success200, 1)
				} else if resp.StatusCode == http.StatusTooManyRequests {
					atomic.AddInt64(&stats.Rejected429, 1)
				} else {
					atomic.AddInt64(&stats.Errors, 1)
				}
			}
		}()
	}

	wg.Wait()
	totalElapsed := time.Since(benchStart)

	printReport(stats, totalElapsed, cfg)
}

func printReport(s *Stats, elapsed time.Duration, cfg Config) {
	effectivePointsPerSec := float64(s.TotalPoints) / elapsed.Seconds()
	effectiveSuccessPointsPerSec := float64(s.Success200*int64(cfg.BatchSize)) / elapsed.Seconds()

	s.LatenciesMu.Lock()
	sort.Slice(s.Latencies, func(i, j int) bool {
		return s.Latencies[i] < s.Latencies[j]
	})
	var p50, p90, p99 time.Duration
	n := len(s.Latencies)
	if n > 0 {
		p50 = s.Latencies[int(float64(n)*0.50)]
		p90 = s.Latencies[int(float64(n)*0.90)]
		p99 = s.Latencies[int(float64(n)*0.99)]
	}
	s.LatenciesMu.Unlock()

	fmt.Println("\n============================ BENCHMARK RESULTS ============================")
	fmt.Printf("Elapsed Time         : %v\n", elapsed.Round(time.Millisecond))
	fmt.Printf("Total Requests Sent  : %d\n", s.TotalRequests)
	fmt.Printf("Total Points Offered : %d\n", s.TotalPoints)
	fmt.Printf("Accepted (200 OK)    : %d requests (%d points)\n", s.Success200, s.Success200*int64(cfg.BatchSize))
	fmt.Printf("Backpressure (429)   : %d requests\n", s.Rejected429)
	fmt.Printf("Errors / Failures    : %d requests\n", s.Errors)
	fmt.Printf("Offered Throughput   : %.2f points/sec\n", effectivePointsPerSec)
	fmt.Printf("Committed Throughput : %.2f points/sec\n", effectiveSuccessPointsPerSec)
	fmt.Println("----------------------------- HTTP LATENCY -----------------------------")
	fmt.Printf("P50 Latency          : %v\n", p50)
	fmt.Printf("P90 Latency          : %v\n", p90)
	fmt.Printf("P99 Latency          : %v\n", p99)
	fmt.Println("==========================================================================")
}

type SyntheticSpan struct {
	StartTs    time.Time              `json:"start_ts"`
	TraceID    string                 `json:"trace_id"`
	SpanID     string                 `json:"span_id"`
	Service    string                 `json:"service"`
	Name       string                 `json:"name"`
	Kind       string                 `json:"kind"`
	DurationNs int64                  `json:"duration_ns"`
	Status     string                 `json:"status"`
	Attrs      map[string]interface{} `json:"attrs"`
}

var sampleServices = []string{
	"daraja-b2c-gateway",
	"daraja-c2b-callback",
	"ussd-menu-session",
	"bank-settlement-engine",
}

var sampleEndpoints = []string{
	"/v1/payments/b2c",
	"/v1/callbacks/mpesa",
	"/ussd/entry",
	"/ussd/balance-check",
	"/settlement/reconcile",
}

func GenerateSyntheticSpans(count int) []byte {
	now := time.Now().UTC()
	spans := make([]SyntheticSpan, 0, count)

	for i := 0; i < count; i++ {
		traceHex := randomHex(16)
		spanHex := randomHex(8)
		service := sampleServices[i%len(sampleServices)]
		endpoint := sampleEndpoints[i%len(sampleEndpoints)]

		durationMs := 20 + (i*17)%250
		status := "STATUS_CODE_OK"
		if i%50 == 0 {
			status = "STATUS_CODE_ERROR"
			durationMs += 800
		}

		spans = append(spans, SyntheticSpan{
			StartTs:    now.Add(-time.Duration(i) * time.Millisecond),
			TraceID:    traceHex,
			SpanID:     spanHex,
			Service:    service,
			Name:       endpoint,
			Kind:       "SPAN_KIND_SERVER",
			DurationNs: int64(durationMs) * 1000000,
			Status:     status,
			Attrs: map[string]interface{}{
				"currency":      "KES",
				"http.status":   200,
				"kenya.carrier": "Safaricom",
			},
		})
	}

	data, _ := json.Marshal(spans)
	return data
}

func randomHex(bytesLen int) string {
	b := make([]byte, bytesLen)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

