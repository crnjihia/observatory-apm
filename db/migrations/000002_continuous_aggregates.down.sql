-- path: db/migrations/000002_continuous_aggregates.down.sql

DROP VIEW IF EXISTS endpoint_latency_agg CASCADE;
DROP MATERIALIZED VIEW IF EXISTS endpoint_latency_cagg CASCADE;

