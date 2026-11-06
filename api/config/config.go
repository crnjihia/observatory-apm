// path: api/config/config.go
package config

import (
	"os"
	"strconv"
	"time"
)

// Config encapsulates environment configuration for the Observatory APM API.
type Config struct {
	DBDSN                   string
	Port                    string
	RedisURL                string
	SamplingDecisionWindow  time.Duration
	SamplingSlowThresholdMs int64
	SamplingHeadRate        float64
	IngestAPIKey            string
	AuthEnabled             bool
	RateLimitIngestRPM      int
	RateLimitQueryRPM       int
}

// Load populates the configuration from environment variables with production defaults.
func Load() (*Config, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://observatory:observatory_secret@localhost:5432/observatory?sslmode=disable"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	redisURL := os.Getenv("REDIS_URL")

	windowSec := 3
	if val := os.Getenv("TAIL_DECISION_WINDOW_SEC"); val != "" {
		if s, err := strconv.Atoi(val); err == nil && s > 0 {
			windowSec = s
		}
	} else if val := os.Getenv("SAMPLING_DECISION_WINDOW_SEC"); val != "" {
		if s, err := strconv.Atoi(val); err == nil && s > 0 {
			windowSec = s
		}
	}

	slowMs := int64(1500)
	if val := os.Getenv("TAIL_SLOW_TRACE_THRESHOLD_MS"); val != "" {
		if ms, err := strconv.ParseInt(val, 10, 64); err == nil && ms > 0 {
			slowMs = ms
		}
	} else if val := os.Getenv("SAMPLING_SLOW_THRESHOLD_MS"); val != "" {
		if ms, err := strconv.ParseInt(val, 10, 64); err == nil && ms > 0 {
			slowMs = ms
		}
	}

	headRate := 0.10
	if val := os.Getenv("SAMPLING_HEAD_RATE"); val != "" {
		if r, err := strconv.ParseFloat(val, 64); err == nil && r >= 0.0 && r <= 1.0 {
			headRate = r
		}
	}

	apiKey := os.Getenv("INGEST_API_KEY")
	if apiKey == "" {
		apiKey = "obs_sandbox_sec_key_nairobi_2026"
	}

	authEnabled := true
	if val := os.Getenv("AUTH_ENABLED"); val != "" {
		authEnabled = (val == "true" || val == "1" || val == "yes")
	}

	rateLimitIngestRPM := 10000
	if val := os.Getenv("RATE_LIMIT_INGEST_RPM"); val != "" {
		if rpm, err := strconv.Atoi(val); err == nil && rpm > 0 {
			rateLimitIngestRPM = rpm
		}
	}

	rateLimitQueryRPM := 1000
	if val := os.Getenv("RATE_LIMIT_QUERY_RPM"); val != "" {
		if rpm, err := strconv.Atoi(val); err == nil && rpm > 0 {
			rateLimitQueryRPM = rpm
		}
	}

	return &Config{
		DBDSN:                   dsn,
		Port:                    port,
		RedisURL:                redisURL,
		SamplingDecisionWindow:  time.Duration(windowSec) * time.Second,
		SamplingSlowThresholdMs: slowMs,
		SamplingHeadRate:        headRate,
		IngestAPIKey:            apiKey,
		AuthEnabled:             authEnabled,
		RateLimitIngestRPM:      rateLimitIngestRPM,
		RateLimitQueryRPM:       rateLimitQueryRPM,
	}, nil
}
