-- path: db/migrations/000001_init_schema.up.sql

CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;

-- ============================================================================
-- 1. SPANS HYPERTABLE
-- ============================================================================
CREATE TABLE IF NOT EXISTS spans (
    start_ts TIMESTAMPTZ NOT NULL,
    trace_id VARCHAR(64) NOT NULL,
    span_id VARCHAR(32) NOT NULL,
    parent_id VARCHAR(32),
    service VARCHAR(128) NOT NULL,
    name VARCHAR(255) NOT NULL,
    kind VARCHAR(32) NOT NULL DEFAULT 'SPAN_KIND_INTERNAL',
    duration_ns BIGINT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'STATUS_CODE_UNSET',
    attrs JSONB NOT NULL DEFAULT '{}'::jsonb
);

SELECT create_hypertable('spans', 'start_ts', if_not_exists => TRUE, chunk_time_interval => INTERVAL '1 day');

CREATE INDEX IF NOT EXISTS idx_spans_trace_id ON spans (trace_id);
CREATE INDEX IF NOT EXISTS idx_spans_service_start_ts ON spans (service, start_ts DESC);
CREATE INDEX IF NOT EXISTS idx_spans_service_name_start_ts ON spans (service, name, start_ts DESC);
CREATE INDEX IF NOT EXISTS idx_spans_duration ON spans (service, duration_ns DESC, start_ts DESC);

-- Compression & Retention Policies for Spans
ALTER TABLE spans SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'service, name',
    timescaledb.compress_orderby = 'start_ts DESC'
);
SELECT add_compression_policy('spans', INTERVAL '7 days', if_not_exists => TRUE);
SELECT add_retention_policy('spans', INTERVAL '30 days', if_not_exists => TRUE);

-- ============================================================================
-- 2. METRICS HYPERTABLE
-- ============================================================================
CREATE TABLE IF NOT EXISTS metrics (
    ts TIMESTAMPTZ NOT NULL,
    service VARCHAR(128) NOT NULL,
    name VARCHAR(255) NOT NULL,
    labels JSONB NOT NULL DEFAULT '{}'::jsonb,
    value DOUBLE PRECISION NOT NULL
);

SELECT create_hypertable('metrics', 'ts', if_not_exists => TRUE, chunk_time_interval => INTERVAL '1 day');

CREATE INDEX IF NOT EXISTS idx_metrics_service_name_ts ON metrics (service, name, ts DESC);

-- Compression & Retention Policies for Metrics
ALTER TABLE metrics SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'service, name',
    timescaledb.compress_orderby = 'ts DESC'
);
SELECT add_compression_policy('metrics', INTERVAL '7 days', if_not_exists => TRUE);
SELECT add_retention_policy('metrics', INTERVAL '30 days', if_not_exists => TRUE);

-- ============================================================================
-- 3. LOGS HYPERTABLE
-- ============================================================================
CREATE TABLE IF NOT EXISTS logs (
    ts TIMESTAMPTZ NOT NULL,
    service VARCHAR(128) NOT NULL,
    severity VARCHAR(32) NOT NULL DEFAULT 'INFO',
    body TEXT NOT NULL,
    trace_id VARCHAR(64),
    span_id VARCHAR(32),
    attrs JSONB NOT NULL DEFAULT '{}'::jsonb
);

SELECT create_hypertable('logs', 'ts', if_not_exists => TRUE, chunk_time_interval => INTERVAL '1 day');

CREATE INDEX IF NOT EXISTS idx_logs_trace_id ON logs (trace_id) WHERE trace_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_logs_service_severity_ts ON logs (service, severity, ts DESC);

-- Compression & Retention Policies for Logs
ALTER TABLE logs SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'service, severity',
    timescaledb.compress_orderby = 'ts DESC'
);
SELECT add_compression_policy('logs', INTERVAL '7 days', if_not_exists => TRUE);
SELECT add_retention_policy('logs', INTERVAL '30 days', if_not_exists => TRUE);
