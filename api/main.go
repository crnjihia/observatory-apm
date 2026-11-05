// path: C:/Users/crnji/Videos/Projects/Web Development/New/observatory-apm/api/main.go
package main

import (
    "context"
    "log"
    "net/http"
    "strconv"

    "github.com/go-chi/chi/v5"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/prometheus/client_golang/prometheus/promhttp"

    "observatory-apm/api/config"
    "observatory-apm/api/handlers"
    "observatory-apm/api/oncall"
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


    // Router setup.
    r := chi.NewRouter()
    r.Get("/health", healthHandler)
    r.Get("/metrics", promhttp.Handler())
    r.Get("/services", handlers.GetServices(dbPool))
    r.Get("/latency", handlers.GetLatency(dbPool))
    r.Get("/traces", handlers.GetTracesByBucket(dbPool))
    r.Get("/trace/{id}", handlers.GetTraceSpans(dbPool))
    r.Get("/trace/{id}/logs", handlers.GetTraceLogs(dbPool))
    // On‑call routes
    r.Mount("/oncall", oncall.APIHandler(oncallStore))

    // Server configuration.
    srv := &http.Server{
        Addr:         ":8080",
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
