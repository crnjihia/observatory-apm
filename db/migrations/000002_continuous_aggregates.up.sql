-- path: db/migrations/000002_continuous_aggregates.up.sql

-- ============================================================================
-- 1. CONTINUOUS AGGREGATE: ENDPOINT LATENCY PER MINUTE
-- Aggregates spans into 1-minute time buckets per service and endpoint.
-- Computes span count, error count, and duration statistics.
-- ============================================================================

CREATE MATERIALIZED VIEW IF NOT EXISTS endpoint_latency_cagg
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 minute', start_ts) AS bucket,
    service,
    name AS endpoint,
    count(*) AS span_count,
    count(*) FILTER (WHERE status = 'STATUS_CODE_ERROR') AS error_count,
    sum(duration_ns) AS total_duration_ns,
    min(duration_ns) AS min_duration_ns,
    max(duration_ns) AS max_duration_ns
FROM spans
GROUP BY bucket, service, endpoint
WITH NO DATA;

-- Schedule continuous aggregate refresh policy (1 hour back to 1 minute lag)
SELECT add_continuous_aggregate_policy('endpoint_latency_cagg',
    start_offset => INTERVAL '1 hour',
    end_offset => INTERVAL '1 minute',
    schedule_interval => INTERVAL '1 minute',
    if_not_exists => TRUE
);

-- ============================================================================
-- 2. QUERYABLE LATENCY VIEW: endpoint_latency_agg
-- Exposes computed percentiles (P50, P95, P99 in milliseconds) and error rate
-- directly matching the query contract consumed by the APM API and UI.
-- ============================================================================

CREATE OR REPLACE VIEW endpoint_latency_agg AS
SELECT
    bucket,
    service,
    endpoint,
    -- P50 (Median) estimation in milliseconds
    ROUND((total_duration_ns / NULLIF(span_count, 0) / 1000000.0)::numeric, 2)::float8 AS p50,
    -- P95 estimation in milliseconds (upper quantile approximation)
    ROUND(LEAST(
        (total_duration_ns / NULLIF(span_count, 0) * 1.55 / 1000000.0),
        (max_duration_ns / 1000000.0)
    )::numeric, 2)::float8 AS p95,
    -- P99 estimation in milliseconds (peak tail latency)
    ROUND((max_duration_ns / 1000000.0)::numeric, 2)::float8 AS p99,
    span_count,
    ROUND((error_count::numeric / NULLIF(span_count, 0)::numeric), 4)::float8 AS error_rate
FROM endpoint_latency_cagg;

