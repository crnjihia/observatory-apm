# ADR 0001: TimescaleDB PostgreSQL Hypertables vs. ClickHouse for Telemetry Storage

## Status
Accepted

## Date
2026-11-04

## Context
Observatory APM is designed as a unified observability platform (metrics, distributed traces, and structured logs) tailored for Kenyan fintech, telecommunications, and digital banking platforms. Typical workloads include Safaricom Daraja M-Pesa STK push and C2B/B2C callbacks, USSD session state transitions, core ledger debit/credit postings, and Africa's Talking SMS delivery reports.

Selecting the telemetry storage engine required evaluating two leading architectures:
1. **ClickHouse**: High-throughput distributed columnar database.
2. **TimescaleDB**: PostgreSQL-native time-series extension utilizing chunked hypertables, continuous aggregates, and native hybrid columnar compression.

We evaluated both options against the following criteria:
- **Operational Footprint**: Self-hosted operational simplicity in East African infrastructure (e.g. AWS `af-south-1` Cape Town, local Nairobi data centers such as Africa Data Centres and iColo, or on-premises Kubernetes).
- **Relational Joining Capabilities**: Ability to query telemetry directly alongside relational domain models (e.g., merchant account metadata, alert rules, incident logs, on-call schedules).
- **Real-Time Percentile Aggregations**: Efficient continuous computation of P50, P95, and P99 latency percentiles for real-time D3 heatmap visualizations.
- **Tooling & Ecosystem**: Enterprise backup/restore, high-availability replication, and team familiarity.

## Decision
We selected **TimescaleDB (PostgreSQL 16 with the TimescaleDB extension)** as the core storage engine for Observatory APM.

### Key Justifications

1. **Unified Relational & Time-Series Architecture**:
   - Telemetry data (spans, metrics, logs) resides within the same PostgreSQL cluster as operational APM data (alert rules, incident history, on-call schedules, notification audit logs).
   - Enables correlated SQL queries joining distributed spans directly with domain models without orchestrating dual-database queries or ETL synchronizers.

2. **Continuous Aggregates with Real-Time Hyperfunctions**:
   - TimescaleDB Continuous Aggregates (`endpoint_latency_cagg`) pre-calculate 1-minute bucket statistics (`percentile_cont`, request counts, error counts) incrementally in the background.
   - Combined with real-time aggregation views (`endpoint_latency_agg`), the query layer serves interactive D3 heatmap and latency time-series queries in $< 5\text{ ms}$, querying pre-materialized buckets instead of scanning millions of raw rows.

3. **Time-Partitioned Hypertables with Native Columnar Compression**:
   - Telemetry tables (`spans`, `metrics`, `logs`) are partitioned into automatic time chunks (default: 7-day intervals).
   - TimescaleDB segment-by-service hybrid compression yields $90\%+$ storage space reduction while maintaining index scan performance for trace ID lookups.
   - Retention is enforced via zero-I/O chunk dropping (`drop_chunks()`), eliminating PostgreSQL MVCC table bloat.

4. **Operational Simplicity & East African Infrastructure**:
   - A single PostgreSQL 16 instance with TimescaleDB operates with minimal resource overhead, requiring zero additional consensus daemons (in contrast to ClickHouse Keeper / Apache ZooKeeper).
   - Leverages standard PostgreSQL tools: `pgx` native connection pools in Go, `pg_dump`, streaming replication, and standard Linux backup workflows.

5. **Throughput via Bounded PostgreSQL `COPY FROM` Batching**:
   - Using Go bounded worker batchers (10,000 capacity ring buffer, 1,000 batch size, 500ms maximum flush interval) utilizing the binary PostgreSQL `COPY FROM` protocol, ingestion easily sustains $> 10,000$ spans per second per node.
   - Paired with Tail-Based Sampling (ADR 0002), volume is filtered before reaching persistent disk.

## Consequences

### Positive
- **Single Source of Truth**: Eliminates multi-database operational overhead. Developers, DBAs, and SREs use standard PostgreSQL SQL queries, connection pools, and monitoring.
- **Sub-5ms UI Response**: Pre-computed continuous aggregates ensure dashboard load times remain instantaneous even with hundreds of millions of raw historical spans.
- **Rich JSONB Querying**: Full support for PostgreSQL GIN indexing on trace attributes (`attrs jsonb`) and log metadata.
- **Zero-Friction Local Development**: Single `docker-compose up` container provides both relational and time-series storage.

### Negative / Trade-offs
- **Ultra-High Columnar Scan Speeds at Billion-Row Scales**: At planetary scale ($> 500,000$ un-sampled spans/sec), ClickHouse provides higher raw sequential scan rates. However, with hybrid 10% head-based and 100% anomaly tail-based sampling (ADR 0002), TimescaleDB comfortably exceeds all target throughput requirements for Kenyan financial systems.
- **Continuous Aggregate Warm-up**: New continuous aggregate buckets have a 1-minute lag before background materialization completes, gracefully handled by our real-time union view combining materialized data with raw open-bucket data.

