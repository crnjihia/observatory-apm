-- path: db/seed.sql

-- ============================================================================
-- OBSERVATORY APM SYNTHETIC SEED FIXTURES (KENYAN FINTECH PIPELINE)
-- Timezone: Africa/Nairobi (UTC+3)
-- Currency: KES
-- Telemetry: Safaricom Daraja M-Pesa Callbacks, USSD Menus, Bank Settlement
-- ============================================================================

-- 1. BASELINE SPANS (Normal operations across the past 30 minutes)
INSERT INTO spans (start_ts, trace_id, span_id, parent_id, service, name, kind, duration_ns, status, attrs)
VALUES
  -- Normal Daraja Callback
  (NOW() - INTERVAL '25 minutes', 'a1b2c3d4e5f60001a1b2c3d4e5f60001', 'span000000000001', NULL,
   'mpesa-callback-service', '/v1/callbacks/mpesa', 'SPAN_KIND_SERVER', 65000000, 'STATUS_CODE_OK',
   '{"currency": "KES", "mpesa.receipt": "QKJ1000001X", "phone": "+254711000001", "amount": 1500.00}'::jsonb),

  (NOW() - INTERVAL '20 minutes', 'a1b2c3d4e5f60002a1b2c3d4e5f60002', 'span000000000002', NULL,
   'mpesa-callback-service', '/v1/callbacks/mpesa', 'SPAN_KIND_SERVER', 82000000, 'STATUS_CODE_OK',
   '{"currency": "KES", "mpesa.receipt": "QKJ1000002X", "phone": "+254711000002", "amount": 5000.00}'::jsonb),

  -- Normal USSD Session
  (NOW() - INTERVAL '18 minutes', 'b2c3d4e5f6a10001b2c3d4e5f6a10001', 'span000000000003', NULL,
   'ussd-menu-session', '/ussd/entry', 'SPAN_KIND_SERVER', 35000000, 'STATUS_CODE_OK',
   '{"ussd.code": "*384*100#", "network": "Safaricom", "phone": "+254722000001"}'::jsonb),

  (NOW() - INTERVAL '15 minutes', 'b2c3d4e5f6a10002b2c3d4e5f6a10002', 'span000000000004', NULL,
   'ussd-menu-session', '/ussd/balance-check', 'SPAN_KIND_SERVER', 48000000, 'STATUS_CODE_OK',
   '{"ussd.code": "*384*100#", "network": "Safaricom", "phone": "+254722000002"}'::jsonb),

  -- Normal B2C Payment
  (NOW() - INTERVAL '12 minutes', 'c3d4e5f6a1b20001c3d4e5f6a1b20001', 'span000000000005', NULL,
   'daraja-b2c-gateway', '/v1/payments/b2c', 'SPAN_KIND_SERVER', 110000000, 'STATUS_CODE_OK',
   '{"currency": "KES", "b2c.command": "BusinessPayment", "amount": 25000.00}'::jsonb);

-- 2. LATENCY SPIKE & INCIDENT CASCADE (Triggered 5 minutes ago)
-- Root cause: Daraja M-Pesa callback gateway timeout leading to queue saturation
INSERT INTO spans (start_ts, trace_id, span_id, parent_id, service, name, kind, duration_ns, status, attrs)
VALUES
  -- Root Span (Timed out HTTP 504)
  (NOW() - INTERVAL '5 minutes', '54eba8423f6d4350ba3291c0bfd51e11', 'root000000000001', NULL,
   'mpesa-callback-service', '/v1/callbacks/mpesa', 'SPAN_KIND_SERVER', 3450000000, 'STATUS_CODE_ERROR',
   '{"currency": "KES", "http.status": 504, "error": "Gateway Timeout", "mpesa.receipt": "QKJ729019X", "phone": "+254711000099", "amount": 3500.00}'::jsonb),

  -- Child Span 1: Signature Verification
  (NOW() - INTERVAL '5 minutes' + INTERVAL '10 milliseconds', '54eba8423f6d4350ba3291c0bfd51e11', 'child00000000001', 'root000000000001',
   'mpesa-callback-service', 'crypto.verify_saf_cert', 'SPAN_KIND_INTERNAL', 12000000, 'STATUS_CODE_OK',
   '{"crypto.algo": "RSA-SHA256"}'::jsonb),

  -- Child Span 2: Ledger Transaction Write (Blocked waiting for DB connection)
  (NOW() - INTERVAL '5 minutes' + INTERVAL '30 milliseconds', '54eba8423f6d4350ba3291c0bfd51e11', 'child00000000002', 'root000000000001',
   'bank-settlement-engine', 'ledger.lock_and_credit', 'SPAN_KIND_CLIENT', 3200000000, 'STATUS_CODE_ERROR',
   '{"db.system": "postgresql", "error.message": "connection pool exhausted waiting for client ack"}'::jsonb),

  -- Secondary failing spans in the same spike window
  (NOW() - INTERVAL '4 minutes', '54eba8423f6d4350ba3291c0bfd51e12', 'root000000000002', NULL,
   'mpesa-callback-service', '/v1/callbacks/mpesa', 'SPAN_KIND_SERVER', 2850000000, 'STATUS_CODE_ERROR',
   '{"currency": "KES", "http.status": 504, "error": "Gateway Timeout", "phone": "+254711000098"}'::jsonb),

  (NOW() - INTERVAL '3 minutes', '54eba8423f6d4350ba3291c0bfd51e13', 'root000000000003', NULL,
   'mpesa-callback-service', '/v1/callbacks/mpesa', 'SPAN_KIND_SERVER', 1980000000, 'STATUS_CODE_ERROR',
   '{"currency": "KES", "http.status": 504, "error": "Gateway Timeout"}'::jsonb);

-- 3. METRIC DATA POINTS
INSERT INTO metrics (ts, service, name, labels, value)
VALUES
  (NOW() - INTERVAL '10 minutes', 'mpesa-callback-service', 'mpesa_callback_latency_p95', '{"env":"production","region":"nairobi"}'::jsonb, 78.5),
  (NOW() - INTERVAL '5 minutes',  'mpesa-callback-service', 'mpesa_callback_latency_p95', '{"env":"production","region":"nairobi"}'::jsonb, 3450.0),
  (NOW() - INTERVAL '10 minutes', 'mpesa-callback-service', 'mpesa_transaction_volume_kes', '{"env":"production"}'::jsonb, 154200.00),
  (NOW() - INTERVAL '5 minutes',  'mpesa-callback-service', 'mpesa_transaction_volume_kes', '{"env":"production"}'::jsonb, 24000.00),
  (NOW() - INTERVAL '10 minutes', 'ussd-menu-session', 'ussd_active_sessions_count', '{"network":"safaricom"}'::jsonb, 420.0),
  (NOW() - INTERVAL '5 minutes',  'ussd-menu-session', 'ussd_active_sessions_count', '{"network":"safaricom"}'::jsonb, 395.0);

-- 4. CORRELATED LOG ENTRIES
INSERT INTO logs (ts, service, severity, body, trace_id, span_id, attrs)
VALUES
  (NOW() - INTERVAL '5 minutes', 'mpesa-callback-service', 'INFO',
   'Received M-Pesa STK Push callback from Safaricom Daraja IP 196.201.214.200 for receipt QKJ729019X, amount KES 3,500.00',
   '54eba8423f6d4350ba3291c0bfd51e11', 'root000000000001', '{"phone": "+254711000099"}'::jsonb),

  (NOW() - INTERVAL '5 minutes' + INTERVAL '12 milliseconds', 'mpesa-callback-service', 'INFO',
   'Validated Daraja X.509 signature against Safaricom public certificate',
   '54eba8423f6d4350ba3291c0bfd51e11', 'child00000000001', '{}'::jsonb),

  (NOW() - INTERVAL '5 minutes' + INTERVAL '3210 milliseconds', 'bank-settlement-engine', 'ERROR',
   'Timeout executing ledger.lock_and_credit: DB pool exhausted after 3200ms. Aborting transaction.',
   '54eba8423f6d4350ba3291c0bfd51e11', 'child00000000002', '{"db.error": "context deadline exceeded"}'::jsonb),

  (NOW() - INTERVAL '5 minutes' + INTERVAL '3450 milliseconds', 'mpesa-callback-service', 'ERROR',
   'HTTP 504 Gateway Timeout: failed to acknowledge Safaricom Daraja callback in 3000ms SLA window',
   '54eba8423f6d4350ba3291c0bfd51e11', 'root000000000001', '{"sla.limit_ms": 3000, "actual_ms": 3450}'::jsonb);

-- 5. REFRESH CONTINUOUS AGGREGATES
CALL refresh_continuous_aggregate('endpoint_latency_cagg', NOW() - INTERVAL '1 hour', NOW());

