# Observatory APM Architecture

Observatory APM is a self-hosted unified observability platform engineered for high-throughput Kenyan fintech, digital banking, and telecommunications infrastructures. It captures metrics, distributed traces, and structured logs with automated Safaricom Daraja M-Pesa SLA compliance monitoring, USSD session state tracking, tail-based anomaly sampling, and multi-channel on-call alerting via Africa's Talking SMS and Slack.

---

## 1. High-Level System Architecture

```mermaid
flowchart TD
    subgraph Clients["Telemetry Producers"]
        NodeApp["Node.js Services (@observatory/agent-node)"]
        GoApp["Go Microservices (observatory-agent-go)"]
        Simulator["Fintech Traffic & Fault Simulator (:4000)"]
    end

    subgraph Edge["API Gateway & Security Layer (:8080)"]
        RL["Token-Bucket Rate Limiter\n(10k ingest/min, 1k query/min)"]
        Auth["API Key Auth Middleware\n(X-API-Key / Bearer)"]
    end

    subgraph Ingestion["Ingestion & Sampling Pipeline"]
        HTTPReceiver["OTLP HTTP Receiver\n(/v1/traces, /v1/metrics, /v1/logs)"]
        TailEngine["Tail Sampling Engine\n(3s Sliding Window)"]
        RedisRing["Redis 7 Buffer\n(Sorted Sets & Lists)"]
        MemBuffer["In-Memory Buffer Fallback"]
        TraceBatcher["Bounded Trace Batcher\n(10k cap, 1k batch, 500ms flush)"]
        MetricBatcher["Bounded Metric Batcher\n(10k cap, 1k batch, 500ms flush)"]
        LogBatcher["Bounded Log Batcher\n(10k cap, 1k batch, 500ms flush)"]
    end

    subgraph Storage["Storage & Continuous Aggregation"]
        Postgres["PostgreSQL 16 + TimescaleDB"]
        Hypertables["Hypertables: spans, metrics, logs"]
        Cagg["Continuous Aggregate: endpoint_latency_cagg"]
        Views["Real-time Query Views: endpoint_latency_agg"]
    end

    subgraph Alerting["Alerting & On-Call System"]
        EWMA["Statistical Evaluator (EWMA & Thresholds)"]
        SMS["Africa's Talking SMS (Safaricom / Airtel / Telkom)"]
        Slack["Slack Webhook Notifier"]
        OnCall["On-Call Escalation State Machine"]
    end

    subgraph Frontend["Dashboard UI (:3001)"]
        NextUI["Next.js 14 Dashboard"]
        Heatmap["D3.js 2D Heatmap & Percentile Splines"]
        Waterfall["Gantt Span Waterfall Viewer"]
        LogsViewer["Correlated JSON Log Viewer"]
        OnCallUI["On-Call Rotation & Incident Manager"]
    end

    Clients --> Edge
    Edge --> RL --> Auth --> HTTPReceiver
    HTTPReceiver --> TailEngine
    TailEngine <--> RedisRing
    TailEngine -.-> MemBuffer
    TailEngine --> TraceBatcher
    HTTPReceiver --> MetricBatcher
    HTTPReceiver --> LogBatcher

    TraceBatcher -->|PostgreSQL COPY FROM| Hypertables
    MetricBatcher -->|PostgreSQL COPY FROM| Hypertables
    LogBatcher -->|PostgreSQL COPY FROM| Hypertables

    Hypertables --> Cagg --> Views
    Views --> EWMA
    EWMA --> OnCall
    OnCall --> SMS
    OnCall --> Slack

    Views --> NextUI
    Hypertables --> NextUI
    NextUI --> Heatmap
    NextUI --> Waterfall
    NextUI --> LogsViewer
    NextUI --> OnCallUI
```

---

## 2. Ingestion & Tail Sampling Lifecycle

The telemetry ingestion pipeline implements a hybrid sampling strategy:
1. **Agent Head Sampling**: Non-critical client traces are pre-sampled at 10% using 64-bit deterministic hash evaluation on the trace ID.
2. **Ingestion Tail Sampling**: All traces arriving at the API are buffered for 3 seconds in Redis (or an in-memory ring buffer).
3. **Anomaly Retention Heuristics**:
   - **100% Errors**: Traces with HTTP 5xx or `STATUS_CODE_ERROR` are retained.
   - **100% Slow Root Spans**: Traces exceeding the Daraja M-Pesa 1500ms SLA threshold are retained.
   - **10% Baseline**: Fast, nominal traces are sampled uniformly at 10%.
   - **Decision Cache**: Retained trace IDs are cached for 60 seconds so late-arriving spans bypass buffering.

```mermaid
sequenceDiagram
    autonumber
    participant App as Producer (Node.js / Go)
    participant GW as Rate Limiter & Auth (:8080)
    participant Sampler as Tail Sampling Engine
    participant Redis as Redis Ring Buffer
    participant Batcher as Bounded Batcher
    participant DB as TimescaleDB (spans)

    App->>GW: POST /v1/traces (Payload with Trace Spans)
    GW->>GW: Check Token Bucket (< 10,000 req/min)
    GW->>GW: Validate X-API-Key / Bearer Token
    GW->>Sampler: Submit Spans
    Sampler->>Sampler: Check 60s Decision Cache
    alt Decision Already Made: Keep
        Sampler->>Batcher: Forward Directly
    else Decision Pending
        Sampler->>Redis: RPUSH obs:spans:<trace_id>, ZADD obs:active_traces
        Note over Sampler,Redis: Buffer for 3-Second Window
        Sampler->>Sampler: Evaluate Heuristics (Error or Latency > 1500ms)
        alt Error or Slow Trace (> 1500ms)
            Sampler->>Batcher: Flush 100% of Buffered Spans
            Sampler->>Sampler: Cache Keep Decision (TTL 60s)
        else Nominal Fast Trace
            Sampler->>Sampler: Evaluate Lower 64-bit Trace Hash (10% Ratio)
            alt Sampled (10%)
                Sampler->>Batcher: Flush Spans to Batcher
            else Dropped (90%)
                Sampler->>Redis: DEL obs:spans:<trace_id>
            end
        end
    end
    Batcher->>DB: PostgreSQL COPY FROM (Binary Stream)
```

---

## 3. Storage & Continuous Aggregation Engine

To guarantee sub-5ms query performance for D3 interactive heatmaps and latency distribution charts, Observatory APM avoids full hypertable scans over millions of spans by leveraging TimescaleDB Continuous Aggregates:

```mermaid
flowchart LR
    subgraph RawData["Raw Telemetry"]
        Spans["spans (Hypertable)"]
    end

    subgraph AggEngine["Continuous Materialization"]
        CAGG["endpoint_latency_cagg\n(1-minute materialized buckets)"]
        RealtimeView["endpoint_latency_agg\n(Union with open-interval spans)"]
    end

    subgraph QueryLayer["Query Endpoints"]
        HeatmapAPI["/heatmap/buckets"]
        LatencyAPI["/latency"]
        PercentilesAPI["P50, P95, P99 Percentiles"]
    end

    Spans -->|Background Rollup| CAGG
    CAGG --> RealtimeView
    Spans -.->|Open Chunks| RealtimeView
    RealtimeView --> HeatmapAPI
    RealtimeView --> LatencyAPI
    RealtimeView --> PercentilesAPI
```

Continuous aggregate highlights:
- Materializes 1-minute bucket summaries: span count, error count, min duration, max duration, and percentile estimations using `percentile_cont(0.50)`, `percentile_cont(0.95)`, and `percentile_cont(0.99)`.
- Continuous aggregate compression policy compresses chunks older than 2 hours.
- Raw spans hypertable applies 7-day chunking with TimescaleDB segment-by-service compression.

---

## 4. Alerting & On-Call Escalation Architecture

```mermaid
flowchart TD
    subgraph Stream["Continuous Metrics Stream"]
        Views["TimescaleDB endpoint_latency_agg"]
    end

    subgraph Evaluator["Alert Evaluator (60s loop)"]
        EWMA["Exponential Weighted Moving Average (alpha = 0.2)"]
        ZScore["Statistical Z-Score Detector"]
        Thresholds["SLA Rules (e.g. Daraja P99 >= 1500ms)"]
        Suppression["300s Alert Cooldown State"]
    end

    subgraph Dispatcher["Channel Dispatcher"]
        AT["Africa's Talking SMS Adapter"]
        SL["Slack Incoming Webhook Adapter"]
    end

    subgraph Recipients["Paging Recipients"]
        Primary["Primary On-Call (+254...)"]
        Escalation["Escalation Secondary (+254...)"]
        SlackChannel["#incident-alerts Slack Channel"]
    end

    Views --> Evaluator
    EWMA --> Thresholds
    ZScore --> Thresholds
    Thresholds --> Suppression
    Suppression --> Dispatcher
    Dispatcher --> AT
    Dispatcher --> SL
    AT -->|E.164 Normalization| Primary
    Primary -.->|Timeout 300s Unacknowledged| Escalation
    SL --> SlackChannel
```

Key alerting features:
- **Kenyan Phone Number Formatting**: Normalizes `07...`, `01...`, `254...`, `+254...` into valid E.164 format for Safaricom, Airtel Kenya, and Telkom Kenya.
- **Statistical Filtering**: EWMA smoothing prevents noisy false alarms during brief, transient network blips.
- **Escalation Timers**: Incidents left unacknowledged escalate after 300 seconds to secondary on-call engineers.

---

## 5. Monorepo Structure

```
observatory-apm/
├── api/                     # Go 1.22 Ingestion & Query REST API
│   ├── alerts/              # Statistical EWMA evaluator, SMS & Slack dispatchers
│   ├── config/              # Environment configuration loader
│   ├── handlers/            # HTTP handlers for services, latency, heatmap, traces, logs
│   ├── ingest/              # OTLP receivers and bounded worker batchers
│   ├── middleware/          # API Key auth & token-bucket rate limiter
│   ├── oncall/              # In-memory incident and escalation store
│   ├── sampling/            # Tail-based 3s sliding-window Redis ring buffer engine
│   └── main.go              # Service entry point & Chi HTTP router
├── db/                      # Database schemas & migrations
│   ├── schema.sql           # TimescaleDB hypertables, indexes, and continuous aggregates
│   └── seed.sql             # Realistic Kenyan fintech & USSD seed fixtures
├── demo/                    # Traffic & Fault Simulator (:4000)
│   ├── src/mpesa.ts         # Safaricom Daraja STK Push & B2C payment scenarios
│   ├── src/ussd.ts          # Telco USSD banking session state machines
│   └── src/index.ts         # Express server with simulated fault injections
├── docs/                    # Architecture and Decision Records
│   ├── adr/                 # Architecture Decision Records (0001, 0002, 0003)
│   └── architecture.md      # Detailed platform design specifications
├── e2e/                     # End-to-End Playwright test suite
│   ├── tests/drilldown.spec.ts  # Heatmap, waterfall, logs, on-call & simulator tests
│   └── playwright.config.ts # Playwright browser runner configuration
├── packages/                # Telemetry SDK Agents
│   ├── agent-node/          # Node.js / Express / Fastify instrumentation agent
│   └── agent-go/            # Go HTTP middleware & span tracking client
└── web/                     # Next.js 14 APM Dashboard (:3001)
    ├── components/          # D3 Heatmap, Gantt Waterfall, Log Viewer, Simulator
    ├── pages/               # Dashboard, Alerts & Rules, On-Call rotations
    └── utils/               # Authenticated API client with offline fallback support
```

