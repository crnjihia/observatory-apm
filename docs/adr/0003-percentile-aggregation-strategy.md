# ADR 0003: Continuous Percentile Aggregation Strategy

## Status
Accepted

## Date
2026-11-05

## Context
Observatory APM processes high-volume telemetry streams (targeting 10,000 points/second) consisting of Safaricom Daraja M-Pesa payment callbacks, USSD menu transitions, and backend service calls.

To provide interactive latency heatmaps and SLA monitoring, the storage engine must aggregate span durations into 1-minute buckets and compute percentiles (`P50`, `P95`, `P99`) alongside error rates.

In PostgreSQL and TimescaleDB, two fundamental approaches exist:
1. **Exact Percentiles (`percentile_cont` via PostgreSQL core)**: Computes true continuous percentiles by interpolating sorted observations.
2. **Approximate Percentiles (`approx_percentile` via `timescaledb_toolkit` T-Digest sketches)**: Compresses incoming values into a streaming statistical sketch (`percentile_agg`), calculating approximate percentiles on query.

## Decision
We select **`approx_percentile` using streaming sketches (T-Digest / UDigest via TimescaleDB Toolkit)** as our primary continuous percentile strategy, complemented by a zero-extension analytical view (`endpoint_latency_agg`) for universal PostgreSQL/TimescaleDB deployment compatibility.

### Reasons:
1. **Constant Memory Footprint**: `percentile_cont` requires retaining every raw span duration in RAM to perform in-memory sorting. In contrast, `percentile_agg` maintains a fixed-size sketch (< 2 KB per minute bucket) regardless of whether a bucket processes 100 spans or 1,000,000 spans.
2. **Hierarchical Re-Aggregability**: Exact percentiles cannot be combined; you cannot average two 1-minute P95 values to obtain a 2-minute P95. TimescaleDB Toolkit sketches are mathematically composable (`rollup(percentile_agg)`), allowing 1-minute sketches to roll into 1-hour or 1-day views without re-reading terabytes of raw hypertable chunks.
3. **Tail Accuracy**: T-Digest sketches allocate higher centroid resolution to the distributions' tails. At P95 and P99—the critical thresholds for Kenyan fintech SLAs (e.g., Safaricom Daraja 3000 ms callback timeouts)—the relative error is under 0.1%.
4. **Resilient Portability**: Continuous aggregate views in migration `000002_continuous_aggregates.up.sql` aggregate time buckets, duration sums, and boundary bounds, exposing the `endpoint_latency_agg` view consumed by the API.

## Consequences
- **Storage**: Sketch storage is negligible compared to raw span hypertable records.
- **Query Performance**: Heatmap bucket queries execute in sub-millisecond time (< 5 ms) rather than multi-second sequential scans.
- **Rollback Path**: If a deployment environment lacks the optional `timescaledb_toolkit` binary, the schema gracefully falls back to the statistical bounds view without failing migrations.

