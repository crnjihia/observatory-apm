// path: C:/Users/crnji/Videos/Projects/Web Development/New/observatory-apm/api/main.go
package main

import (
    "context"
    "log"
    "net/http"
    "strconv"

    "github.com/go-chi/chi/v5"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/prometheus/client_golang/prometheus/promhttp"

    "observatory-apm/api/alerts"
    "observatory-apm/api/config"
    "observatory-apm/api/handlers"
    "observatory-apm/api/ingest"
    "observatory-apm/api/middleware"
    "observatory-apm/api/oncall"
    "observatory-apm/api/sampling"
    "os"
    "strings"
    "time"
)

func main() {
    // Load configuration (environment variables).
    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("config error: %v", err)
    }

    // Initialise PostgreSQL connection pool.
    dbPool, err := pgxpool.New(context.Background(), cfg.DBDSN)
    if err != nil {
        log.Fatalf("failed to connect to DB: %v", err)
    }
    defer dbPool.Close()

    // Initialise on‑call store from environment variables.
    // Expected env vars:
    //   ONCALL_ROTATION   – comma‑separated phone numbers (e.g. +254712345678,+254712345679)
    //   ONCALL_ROTATION_PERIOD – seconds per person (default 86400 = 24h)
    //   ONCALL_ESCALATION – comma‑separated phone numbers for escalation chain
    //   ONCALL_ESCALATION_TIMEOUT – seconds before escalating (default 300)
    rotationList := strings.Split(os.Getenv("ONCALL_ROTATION"), ",")
    periodSec, _ := strconv.Atoi(os.Getenv("ONCALL_ROTATION_PERIOD"))
    if periodSec <= 0 {
        periodSec = 86400 // default 24h
    }
    escalationList := strings.Split(os.Getenv("ONCALL_ESCALATION"), ",")
    escTimeoutSec, _ := strconv.Atoi(os.Getenv("ONCALL_ESCALATION_TIMEOUT"))
    if escTimeoutSec <= 0 {
        escTimeoutSec = 300 // default 5 min
    }
    oncallStore := oncall.NewStore(
        oncall.OnCallSchedule{Users: rotationList, RotationPeriod: time.Duration(periodSec) * time.Second},
        oncall.EscalationChain{Users: escalationList, Timeout: time.Duration(escTimeoutSec) * time.Second},
    )
    // Load persisted incidents if any.
    _ = oncallStore.Load("/tmp/oncall_incidents.json")
    // Register on‑call metrics.
    oncall.Register()

    // Initialize Alerting Engine (statistical evaluator, rules, SMS & Slack dispatchers).
    ruleStore := alerts.NewRuleStore()
    atNotifier := alerts.NewAfricasTalkingNotifier()
    slackNotifier := alerts.NewSlackNotifier()
    channelNotifier := alerts.NewChannelNotifier(atNotifier, slackNotifier, oncallStore)
    alertEvaluator := alerts.NewEvaluator(ruleStore, channelNotifier)

    // Initialize bounded batchers for OTLP ingestion.
    traceBatcher := ingest.NewBatcher[ingest.Span](
        "trace",
        10000,
        1000,
        500*time.Millisecond,
        func(ctx context.Context, batch []ingest.Span) error {
            if dbPool != nil {
                rows := make([][]interface{}, len(batch))
                for i, s := range batch {
                    rows[i] = []interface{}{
                        s.StartTs, s.TraceID, s.SpanID, s.ParentID, s.Service, s.Name, s.Kind, s.DurationNs, s.Status, s.Attrs,
                    }
                }
                _, err := dbPool.CopyFrom(
                    ctx,
                    pgx.Identifier{"spans"},
                    []string{"start_ts", "trace_id", "span_id", "parent_id", "service", "name", "kind", "duration_ns", "status", "attrs"},
                    pgx.CopyFromRows(rows),
                )
                return err
            }
            return nil
        },
    )
    defer traceBatcher.Stop()

    metricBatcher := ingest.NewBatcher[ingest.Metric](
        "metric",
        10000,
        1000,
        500*time.Millisecond,
        func(ctx context.Context, batch []ingest.Metric) error {
            if dbPool != nil {
                rows := make([][]interface{}, len(batch))
                for i, m := range batch {
                    rows[i] = []interface{}{
                        m.Ts, m.Service, m.Name, m.Labels, m.Value,
                    }
                }
                _, err := dbPool.CopyFrom(
                    ctx,
                    pgx.Identifier{"metrics"},
                    []string{"ts", "service", "name", "labels", "value"},
                    pgx.CopyFromRows(rows),
                )
                return err
            }
            return nil
        },
    )
    defer metricBatcher.Stop()

    logBatcher := ingest.NewBatcher[ingest.LogRecord](
        "log",
        10000,
        1000,
        500*time.Millisecond,
        func(ctx context.Context, batch []ingest.LogRecord) error {
            if dbPool != nil {
                rows := make([][]interface{}, len(batch))
                for i, l := range batch {
                    rows[i] = []interface{}{
                        l.Ts, l.Service, l.Severity, l.Body, l.TraceID, l.SpanID, l.Attrs,
                    }
                }
                _, err := dbPool.CopyFrom(
                    ctx,
                    pgx.Identifier{"logs"},
                    []string{"ts", "service", "severity", "body", "trace_id", "span_id", "attrs"},
                    pgx.CopyFromRows(rows),
                )
                return err
            }
            return nil
        },
    )
    defer logBatcher.Stop()

    // Initialize tail-based sampling engine (Redis ring buffer or in-memory fallback).
    tailBuffer, err := sampling.NewTraceBuffer(cfg.RedisURL)
    if err != nil {
        log.Fatalf("failed to initialize tail sampling buffer: %v", err)
    }
    tailSampler := sampling.NewTailSampler(
        sampling.TailSamplerConfig{
            DecisionWindow:  cfg.SamplingDecisionWindow,
            SlowThresholdMs: cfg.SamplingSlowThresholdMs,
            HeadSampleRate:  cfg.SamplingHeadRate,
            CacheTTL:        60 * time.Second,
        },
        tailBuffer,
        traceBatcher,
    )
    tailSampler.Start()
    defer tailSampler.Stop()

    ingestReceiver := ingest.NewHTTPReceiver(traceBatcher, metricBatcher, logBatcher, tailSampler)

    // Initialize Auth & Rate Limiter middlewares.
    authMiddleware := middleware.NewAuth(cfg.IngestAPIKey, cfg.AuthEnabled, "/health", "/metrics")
    ingestRateLimiter := middleware.NewRateLimiter(cfg.RateLimitIngestRPM)
    defer ingestRateLimiter.Close()
    queryRateLimiter := middleware.NewRateLimiter(cfg.RateLimitQueryRPM)
    defer queryRateLimiter.Close()

    // Router setup.
    r := chi.NewRouter()
    r.Get("/health", healthHandler)
    r.Handle("/metrics", promhttp.Handler())

    // Ingestion routes with Ingest Rate Limiter (default 10,000 req/min) & API Key Auth
    r.Group(func(ingestRouter chi.Router) {
        ingestRouter.Use(ingestRateLimiter.Handler)
        ingestRouter.Use(authMiddleware.Handler)
        ingestRouter.Mount("/", ingestReceiver.Routes())
    })

    // Query & Alerting routes with Query Rate Limiter (default 1,000 req/min)
    r.Group(func(queryRouter chi.Router) {
        queryRouter.Use(queryRateLimiter.Handler)
        queryRouter.Get("/services", handlers.GetServices(dbPool))
        queryRouter.Get("/latency", handlers.GetLatency(dbPool))
        queryRouter.Get("/heatmap/buckets", handlers.GetHeatmapBuckets(dbPool))
        queryRouter.Get("/traces", handlers.GetTracesByBucket(dbPool))
        queryRouter.Get("/trace/{id}", handlers.GetTraceSpans(dbPool))
        queryRouter.Get("/trace/{id}/logs", handlers.GetTraceLogs(dbPool))
        queryRouter.Get("/logs", handlers.GetLogs(dbPool))
        queryRouter.Get("/errors", handlers.GetErrors(dbPool))
        // On‑call & Alerting routes
        queryRouter.Mount("/oncall", oncall.APIHandler(oncallStore))
        queryRouter.Mount("/alerts", alerts.APIHandler(ruleStore, alertEvaluator))
    })

    // Server configuration.
    srv := &http.Server{
        Addr:         ":" + cfg.Port,
        Handler:      r,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 10 * time.Second,
        IdleTimeout:  120 * time.Second,
    }
    log.Printf("Observatory APM API listening on %s", srv.Addr)
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatalf("server error: %v", err)
    }
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
    _, _ = w.Write([]byte(`{"status":"ok"}`))
}
