# ADR 0002: Tail-Based Sampling with Sliding-Window Redis Buffer

## Status
Accepted

## Date
2026-11-05

## Context
Kenyan fintech and telecommunications infrastructures handle massive, bursty transaction volumes—ranging from Safaricom Daraja M-Pesa STK Push payment callbacks to USSD session state machines and Africa's Talking SMS notifications. During peak Nairobi commercial hours, payment callback surges generate tens of thousands of spans per minute.

Two competing architectural requirements exist:
1. **Cost & Storage Feasibility**: Storing 100% of telemetry traces across millions of daily transactions quickly exhausts TimescaleDB disk I/O, increases index bloat, and inflates cloud storage costs.
2. **Incident Root Cause Visibility**: Critical anomalies (such as Safaricom Daraja `504 Gateway Timeout` callback spikes or telco USSD network resets) often represent less than 0.1% to 1.0% of total volume. Under naive 10% head-based sampling, 90% of those intermittent failure traces are irreversibly dropped before reaching storage, preventing engineering teams from diagnosing root causes.

## Decision
We implement a **Hybrid Sampling Architecture** combining agent-side 10% head-based ratio sampling with an ingest-side **Tail-Based Sampling Engine** governed by a 3-second sliding decision window.

The tail sampler temporarily buffers incoming spans per `trace_id` in Redis 7 (or an in-memory buffer fallback) and applies deterministic retention heuristics before deciding whether to persist or drop the complete distributed trace:
1. **100% Error Trace Capture**: Any trace containing at least one span with status `STATUS_CODE_ERROR`, HTTP status code $\ge 500$, or attribute `error = true` is retained at 100% fidelity.
2. **100% Slow Root Span Capture**: Any trace whose root span duration exceeds the SLA latency threshold (default: $1500\text{ ms}$, representing Daraja callback timeout boundaries) is retained at 100% fidelity.
3. **100% Slow Child Span Capture**: Any trace with individual child bottleneck spans exceeding $1500\text{ ms}$ (e.g. locked database queries or telco webhook delays) is retained.
4. **10% Uniform Baseline Sampling**: Traces that are neither erroneous nor slow are sampled deterministically at 10% (`0.10`) using 64-bit integer hashing on the lower 64 bits of the W3C `trace_id`.
5. **Late-Arriving Span Routing**: Sampling decisions are cached in a TTL map for 60 seconds. Late spans arriving for previously retained traces are forwarded immediately to storage without incurring an additional decision window.

### Storage & Buffer Strategy: Redis Ring Buffer with In-Memory Fallback
- **Distributed Mode**: Uses Redis 7 lists (`obs:spans:<trace_id>`) for payload isolation and sorted sets (`obs:active_traces`) indexed by arrival epoch `UnixNano` for sub-millisecond range queries (`ZRANGEBYSCORE`). This allows multiple stateless `observatory-api` instances behind an NGINX or AWS ALB load balancer to observe spans for the same distributed trace concurrently.
- **Standalone / Fallback Mode**: When `REDIS_URL` is omitted or Redis is unreachable, the engine automatically switches to `MemoryTraceBuffer` protected by `sync.RWMutex`. This guarantees zero-dependency development, unit testing, and isolated single-node deployment.

## Consequences
- **Zero Loss of Anomaly Telemetry**: 100% of customer-impacting payment failures, timeout spikes, and unhandled panics are retained in TimescaleDB with full distributed context.
- **Storage Savings**: Normal, fast transactions ($< 1500\text{ ms}$, status `OK`) are reduced by 90%, reducing hypertable disk consumption and ingestion pressure by approximately 85% overall.
- **Latency**: Ingestion API endpoints respond immediately ($< 2\text{ ms}$) with HTTP 200/202, as buffering occurs asynchronously prior to batch insertion into TimescaleDB.
- **Memory Footprint**: Active trace buffer consumes $< 30\text{ MB}$ of Redis RAM under typical workloads (3-second window at 5,000 spans/sec with average 1 KB span size).

