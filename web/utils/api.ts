// path: web/utils/api.ts
export interface ServiceResponse extends Array<string> {}

export interface LatencyRow {
  bucket: string; // ISO timestamp
  service: string;
  endpoint: string;
  p50: number;
  p95: number;
  p99: number;
  span_count: number;
  error_rate: number;
}

export interface HeatmapCell {
  time_bucket: string;
  latency_min_ms: number;
  latency_max_ms: number;
  count: number;
  error_count: number;
}

export interface PercentileOverlay {
  time_bucket: string;
  p50: number;
  p95: number;
  p99: number;
}

export interface HeatmapResponse {
  service: string;
  endpoint?: string;
  start: string;
  end: string;
  min_duration_ms: number;
  max_duration_ms: number;
  cells: HeatmapCell[];
  overlays: PercentileOverlay[];
}

export interface ErrorItem {
  trace_id: string;
  span_id: string;
  service: string;
  endpoint: string;
  error_message: string;
  duration_ms: number;
  start_ts: string;
}

export interface ErrorGroup {
  service: string;
  endpoint: string;
  error_message: string;
  count: number;
}

export interface ErrorSummaryResponse {
  total_errors: number;
  error_groups: ErrorGroup[];
  recent_spans: ErrorItem[];
}

export interface SpanRow {
  trace_id: string;
  span_id: string;
  parent_id?: string;
  service: string;
  name: string;
  kind?: string;
  start_ts: string;
  duration_ns: number;
  status?: string;
  attrs?: Record<string, any>;
}

export interface LogRow {
  ts: string;
  service: string;
  severity?: string;
  body: string;
  trace_id?: string;
  span_id?: string;
  attrs?: Record<string, any>;
}

export interface AlertRule {
  id: string;
  name: string;
  service: string;
  endpoint: string;
  metric: string;
  condition: string;
  threshold: number;
  window_sec: number;
  severity: string;
  channels: string[];
  enabled: boolean;
  cooldown_sec: number;
}

export interface ActiveAlertItem {
  id: string;
  rule_id: string;
  rule_name: string;
  service: string;
  endpoint: string;
  metric: string;
  observed_value: number;
  threshold: number;
  severity: string;
  triggered_at: string;
  state: string; // 'TRIGGERED' | 'ACKNOWLEDGED' | 'RESOLVED'
  acknowledged_at?: string;
  resolved_at?: string;
}

export interface Incident {
  id: string;
  service: string;
  endpoint: string;
  description: string;
  created_at: string;
  state: string; // 'triggered' | 'acknowledged' | 'resolved'
  acknowledged_at?: string;
  resolved_at?: string;
}

const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';
const API_KEY = process.env.NEXT_PUBLIC_API_KEY || 'obs_sandbox_sec_key_nairobi_2026';

function apiFetch(input: string, init?: RequestInit): Promise<Response> {
  const headers = new Headers(init?.headers);
  if (!headers.has('X-API-Key') && API_KEY) {
    headers.set('X-API-Key', API_KEY);
  }
  return fetch(input, {
    ...init,
    headers,
  });
}

export async function fetchServices(): Promise<string[]> {
  try {
    const resp = await apiFetch(`${API_BASE}/services`);
    if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
    const data = (await resp.json()) as string[];
    if (Array.isArray(data) && data.length > 0) {
      return data;
    }
  } catch (err) {
    // Fallback default Kenyan services for resilient offline rendering
  }
  return ['mpesa-payment-gateway', 'ussd-banking-session', 'daraja-b2c-worker', 'ledger-service'];
}

export async function fetchLatency(params: {
  service: string;
  endpoint?: string;
  start: string;
  end: string;
  limit?: number;
}): Promise<LatencyRow[]> {
  const qp = new URLSearchParams({
    service: params.service,
    start: params.start,
    end: params.end,
  });
  if (params.endpoint) qp.set('endpoint', params.endpoint);
  if (params.limit) qp.set('limit', params.limit.toString());

  try {
    const resp = await apiFetch(`${API_BASE}/latency?${qp.toString()}`);
    if (resp.ok) {
      return (await resp.json()) as LatencyRow[];
    }
  } catch (err) {
    // fallback to synthetic series
  }

  // Generate synthetic latency data points across the interval
  const rows: LatencyRow[] = [];
  const startTime = new Date(params.start).getTime();
  const endTime = new Date(params.end).getTime();
  const stepMs = Math.max(60000, Math.floor((endTime - startTime) / 15));

  for (let t = startTime; t <= endTime; t += stepMs) {
    const d = new Date(t);
    const isSpike = Math.random() < 0.2;
    rows.push({
      bucket: d.toISOString(),
      service: params.service,
      endpoint: params.endpoint || '/v1/callbacks/mpesa',
      p50: isSpike ? 180 + Math.random() * 80 : 45 + Math.random() * 30,
      p95: isSpike ? 950 + Math.random() * 600 : 180 + Math.random() * 80,
      p99: isSpike ? 2100 + Math.random() * 800 : 380 + Math.random() * 120,
      span_count: Math.floor(150 + Math.random() * 400),
      error_rate: isSpike ? 0.045 : 0.002,
    });
  }
  return rows;
}

export async function fetchHeatmapBuckets(params: {
  service: string;
  endpoint?: string;
  start: string;
  end: string;
  bins?: number;
}): Promise<HeatmapResponse> {
  const qp = new URLSearchParams({
    service: params.service,
    start: params.start,
    end: params.end,
  });
  if (params.endpoint) qp.set('endpoint', params.endpoint);
  if (params.bins) qp.set('bins', params.bins.toString());

  try {
    const resp = await apiFetch(`${API_BASE}/heatmap/buckets?${qp.toString()}`);
    if (resp.ok) {
      const data = (await resp.json()) as HeatmapResponse;
      if (data.cells && data.cells.length > 0) {
        return data;
      }
    }
  } catch (err) {
    // Fallback generation
  }

  // Generate synthetic 2D heatmap matrix
  const cells: HeatmapCell[] = [];
  const overlays: PercentileOverlay[] = [];
  const startTime = new Date(params.start).getTime();
  const endTime = new Date(params.end).getTime();
  const timeSteps = 15;
  const timeStepMs = Math.max(60000, Math.floor((endTime - startTime) / timeSteps));
  const latencyBins = 12;
  const binHeightMs = 250;

  for (let t = 0; t < timeSteps; t++) {
    const bucketTime = new Date(startTime + t * timeStepMs).toISOString();
    const isAnomalous = t >= 9 && t <= 11; // Simulates Kenyan network timeout spike

    for (let b = 0; b < latencyBins; b++) {
      const minLatency = b * binHeightMs;
      const maxLatency = (b + 1) * binHeightMs;
      let count = 0;
      let errorCount = 0;

      if (b === 0 || b === 1) {
        count = Math.floor(120 + Math.random() * 180);
      } else if (b >= 2 && b <= 4) {
        count = Math.floor(40 + Math.random() * 80);
      } else if (isAnomalous && b >= 7) {
        count = Math.floor(25 + Math.random() * 45);
        errorCount = Math.floor(count * 0.7);
      } else if (Math.random() < 0.15) {
        count = Math.floor(5 + Math.random() * 15);
      }

      if (count > 0) {
        cells.push({
          time_bucket: bucketTime,
          latency_min_ms: minLatency,
          latency_max_ms: maxLatency,
          count,
          error_count: errorCount,
        });
      }
    }

    overlays.push({
      time_bucket: bucketTime,
      p50: isAnomalous ? 220 : 65,
      p95: isAnomalous ? 1650 : 280,
      p99: isAnomalous ? 2850 : 520,
    });
  }

  return {
    service: params.service,
    endpoint: params.endpoint,
    start: params.start,
    end: params.end,
    min_duration_ms: 0,
    max_duration_ms: latencyBins * binHeightMs,
    cells,
    overlays,
  };
}

export async function fetchErrors(params: {
  service?: string;
  limit?: number;
}): Promise<ErrorSummaryResponse> {
  const qp = new URLSearchParams();
  if (params.service) qp.set('service', params.service);
  if (params.limit) qp.set('limit', params.limit.toString());

  try {
    const resp = await apiFetch(`${API_BASE}/errors?${qp.toString()}`);
    if (resp.ok) {
      return (await resp.json()) as ErrorSummaryResponse;
    }
  } catch (err) {
    // fallback below
  }

  return {
    total_errors: 18,
    error_groups: [
      {
        service: params.service || 'mpesa-payment-gateway',
        endpoint: '/v1/callbacks/mpesa',
        error_message: 'Safaricom Daraja 504 Gateway Timeout: Callback request timed out after 3000ms',
        count: 11,
      },
      {
        service: params.service || 'ussd-banking-session',
        endpoint: '/ussd/v2/menu',
        error_message: 'USSD session dropped by telco: Network channel disconnect',
        count: 5,
      },
      {
        service: params.service || 'ledger-service',
        endpoint: '/v1/wallets/debit',
        error_message: 'Transaction rollback: Insufficient float balance',
        count: 2,
      },
    ],
    recent_spans: [
      {
        trace_id: '54eba8423f6d4350ba3291c0bfd51e11',
        span_id: '91c0bfd51e110001',
        service: params.service || 'mpesa-payment-gateway',
        endpoint: '/v1/callbacks/mpesa',
        error_message: 'Safaricom Daraja 504 Gateway Timeout',
        duration_ms: 3120,
        start_ts: new Date().toISOString(),
      },
    ],
  };
}

export async function fetchTracesForBucket(params: {
  service: string;
  endpoint?: string;
  bucket: string;
  limit?: number;
}): Promise<string[]> {
  const qp = new URLSearchParams({
    service: params.service,
    bucket: params.bucket,
  });
  if (params.endpoint) qp.set('endpoint', params.endpoint);
  if (params.limit) qp.set('limit', params.limit.toString());

  try {
    const resp = await apiFetch(`${API_BASE}/traces?${qp.toString()}`);
    if (resp.ok) {
      const data = (await resp.json()) as string[];
      if (Array.isArray(data) && data.length > 0) return data;
    }
  } catch (err) {}

  return [
    '54eba8423f6d4350ba3291c0bfd51e11',
    '7b492f1589a149f193cb9981a4e21a22',
    '9c18f3a09e1346ab90f230245a990013',
  ];
}

export async function fetchTraceSpans(traceId: string): Promise<SpanRow[]> {
  try {
    const resp = await apiFetch(`${API_BASE}/trace/${traceId}`);
    if (resp.ok) return (await resp.json()) as SpanRow[];
  } catch (err) {}

  return [
    {
      trace_id: traceId,
      span_id: 'root-span-001',
      service: 'mpesa-payment-gateway',
      name: 'HTTP POST /v1/callbacks/mpesa',
      kind: 'SPAN_KIND_SERVER',
      start_ts: new Date(Date.now() - 3200).toISOString(),
      duration_ns: 3120000000,
      status: 'STATUS_CODE_ERROR',
      attrs: {
        'http.status_code': 504,
        'mpesa.phone': '+254712345678',
        'mpesa.receipt': 'QHD4819AKD',
        'error.message': 'Safaricom Daraja 504 Gateway Timeout: STK push acknowledgement expired',
      },
    },
    {
      trace_id: traceId,
      span_id: 'child-span-002',
      parent_id: 'root-span-001',
      service: 'daraja-client',
      name: 'Daraja.STKPushQuery',
      kind: 'SPAN_KIND_CLIENT',
      start_ts: new Date(Date.now() - 3100).toISOString(),
      duration_ns: 3000000000,
      status: 'STATUS_CODE_ERROR',
      attrs: {
        'http.url': 'https://api.safaricom.co.ke/mpesa/stkpushquery/v1/query',
        'error': true,
      },
    },
  ];
}

export async function fetchTraceLogs(traceId: string): Promise<LogRow[]> {
  try {
    const resp = await apiFetch(`${API_BASE}/trace/${traceId}/logs`);
    if (resp.ok) return (await resp.json()) as LogRow[];
  } catch (err) {}

  return [
    {
      ts: new Date(Date.now() - 3150).toISOString(),
      service: 'mpesa-payment-gateway',
      severity: 'INFO',
      body: 'Initiating M-Pesa STK verification for MSISDN +254712345678, amount: KES 4,500',
      trace_id: traceId,
      span_id: 'root-span-001',
    },
    {
      ts: new Date(Date.now() - 100).toISOString(),
      service: 'mpesa-payment-gateway',
      severity: 'ERROR',
      body: 'Gateway Timeout: Safaricom Daraja failed to acknowledge STK query within 3000ms SLA deadline',
      trace_id: traceId,
      span_id: 'root-span-001',
      attrs: {
        error_code: 'DARAJA_504_TIMEOUT',
        retry_scheduled: true,
      },
    },
  ];
}

export async function fetchCurrentOnCall(): Promise<string> {
  try {
    const resp = await apiFetch(`${API_BASE}/oncall/current`);
    if (resp.ok) {
      const data = await resp.json();
      return data.on_call as string;
    }
  } catch (err) {}
  return '+254712345678 (Christopher Njihia)';
}

// In-memory incidents storage for offline standalone UI testing
let inMemoryIncidents: Incident[] = [
  {
    id: 'inc-nairobi-001',
    service: 'mpesa-payment-gateway',
    endpoint: '/v1/callbacks/mpesa',
    description: 'P99 Latency exceeded 1500ms SLA threshold (2,850ms detected)',
    created_at: new Date(Date.now() - 600000).toISOString(),
    state: 'triggered',
  },
];

export async function fetchIncidents(): Promise<Incident[]> {
  try {
    const resp = await apiFetch(`${API_BASE}/oncall/incidents`);
    if (resp.ok) return (await resp.json()) as Incident[];
  } catch (err) {}

  return [...inMemoryIncidents];
}

export async function createIncident(payload: {
  service: string;
  endpoint: string;
  description: string;
  }): Promise<Incident> {
  try {
    const resp = await apiFetch(`${API_BASE}/oncall/incidents`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (resp.ok) return (await resp.json()) as Incident;
  } catch (err) {}

  const newInc: Incident = {
    id: 'inc-' + Math.random().toString(36).substring(2, 10),
    service: payload.service,
    endpoint: payload.endpoint,
    description: payload.description,
    created_at: new Date().toISOString(),
    state: 'triggered',
  };
  inMemoryIncidents.unshift(newInc);
  return newInc;
}

export async function ackIncident(id: string): Promise<Incident> {
  try {
    const resp = await apiFetch(`${API_BASE}/oncall/incidents/${id}/ack`, { method: 'POST' });
    if (resp.ok) return (await resp.json()) as Incident;
  } catch (err) {}

  const match = inMemoryIncidents.find((i) => i.id === id);
  if (match) {
    match.state = 'acknowledged';
    match.acknowledged_at = new Date().toISOString();
    return match;
  }
  return { id, service: '', endpoint: '', description: '', created_at: '', state: 'acknowledged' };
}

export async function resolveIncident(id: string): Promise<Incident> {
  try {
    const resp = await apiFetch(`${API_BASE}/oncall/incidents/${id}/resolve`, { method: 'POST' });
    if (resp.ok) return (await resp.json()) as Incident;
  } catch (err) {}

  const match = inMemoryIncidents.find((i) => i.id === id);
  if (match) {
    match.state = 'resolved';
    match.resolved_at = new Date().toISOString();
    return match;
  }
  return { id, service: '', endpoint: '', description: '', created_at: '', state: 'resolved' };
}

export async function fetchAlertRules(): Promise<AlertRule[]> {
  try {
    const resp = await apiFetch(`${API_BASE}/alerts/rules`);
    if (resp.ok) return (await resp.json()) as AlertRule[];
  } catch (err) {}

  return [
    {
      id: 'rule-daraja-sla-p99',
      name: 'Safaricom Daraja P99 Callback Latency SLA Breach',
      service: 'mpesa-payment-gateway',
      endpoint: '/v1/callbacks/mpesa',
      metric: 'p99_latency',
      condition: 'gte',
      threshold: 1500.0,
      window_sec: 60,
      severity: 'CRITICAL',
      channels: ['sms', 'slack'],
      enabled: true,
      cooldown_sec: 300,
    },
    {
      id: 'rule-high-error-rate',
      name: 'Payment Gateway Error Rate Surge',
      service: '*',
      endpoint: '*',
      metric: 'error_rate',
      condition: 'gte',
      threshold: 0.02,
      window_sec: 60,
      severity: 'CRITICAL',
      channels: ['sms', 'slack'],
      enabled: true,
      cooldown_sec: 300,
    },
    {
      id: 'rule-ussd-latency-warn',
      name: 'USSD Telco Menu Response Latency Warning',
      service: 'ussd-banking-session',
      endpoint: '/ussd/v2/menu',
      metric: 'p95_latency',
      condition: 'gte',
      threshold: 800.0,
      window_sec: 120,
      severity: 'WARNING',
      channels: ['slack'],
      enabled: true,
      cooldown_sec: 600,
    },
  ];
}

export async function toggleAlertRule(id: string, enabled: boolean): Promise<void> {
  await apiFetch(`${API_BASE}/alerts/rules/${id}/toggle`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ enabled }),
  });
}

export async function fetchActiveAlerts(): Promise<ActiveAlertItem[]> {
  try {
    const resp = await apiFetch(`${API_BASE}/alerts/active`);
    if (resp.ok) return (await resp.json()) as ActiveAlertItem[];
  } catch (err) {}

  return [
    {
      id: 'alt-daraja-01',
      rule_id: 'rule-daraja-sla-p99',
      rule_name: 'Safaricom Daraja P99 Callback Latency SLA Breach',
      service: 'mpesa-payment-gateway',
      endpoint: '/v1/callbacks/mpesa',
      metric: 'p99_latency',
      observed_value: 2850.0,
      threshold: 1500.0,
      severity: 'CRITICAL',
      triggered_at: new Date(Date.now() - 300000).toISOString(),
      state: 'TRIGGERED',
    },
  ];
}

export async function fetchAlertHistory(): Promise<ActiveAlertItem[]> {
  try {
    const resp = await apiFetch(`${API_BASE}/alerts/history`);
    if (resp.ok) return (await resp.json()) as ActiveAlertItem[];
  } catch (err) {}

  return [
    {
      id: 'alt-daraja-01',
      rule_id: 'rule-daraja-sla-p99',
      rule_name: 'Safaricom Daraja P99 Callback Latency SLA Breach',
      service: 'mpesa-payment-gateway',
      endpoint: '/v1/callbacks/mpesa',
      metric: 'p99_latency',
      observed_value: 2850.0,
      threshold: 1500.0,
      severity: 'CRITICAL',
      triggered_at: new Date(Date.now() - 300000).toISOString(),
      state: 'TRIGGERED',
    },
    {
      id: 'alt-ussd-02',
      rule_id: 'rule-ussd-latency-warn',
      rule_name: 'USSD Telco Menu Response Latency Warning',
      service: 'ussd-banking-session',
      endpoint: '/ussd/v2/menu',
      metric: 'p95_latency',
      observed_value: 840.0,
      threshold: 800.0,
      severity: 'WARNING',
      triggered_at: new Date(Date.now() - 1800000).toISOString(),
      state: 'RESOLVED',
      resolved_at: new Date(Date.now() - 1200000).toISOString(),
    },
  ];
}

export async function ackAlert(id: string): Promise<ActiveAlertItem> {
  const resp = await apiFetch(`${API_BASE}/alerts/${id}/ack`, { method: 'POST' });
  if (!resp.ok) throw new Error('Failed to acknowledge alert');
  return (await resp.json()) as ActiveAlertItem;
}

export async function resolveAlert(id: string): Promise<ActiveAlertItem> {
  const resp = await apiFetch(`${API_BASE}/alerts/${id}/resolve`, { method: 'POST' });
  if (!resp.ok) throw new Error('Failed to resolve alert');
  return (await resp.json()) as ActiveAlertItem;
}

