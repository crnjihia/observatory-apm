// path: C:/Users/crnji/Videos/Projects/Web Development/New/observatory-apm/web/utils/api.ts
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

const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

export async function fetchServices(): Promise<string[]> {
  const resp = await fetch(`${API_BASE}/services`);
  if (!resp.ok) throw new Error('Failed to fetch services');
  const data = (await resp.json()) as string[];
  return data;
}

export async function fetchTracesForBucket(params: {
  service: string;
  endpoint?: string;
  bucket: string; // RFC3339 minute timestamp
  limit?: number;
}): Promise<string[]> {
  const qp = new URLSearchParams({
    service: params.service,
    bucket: params.bucket,
  });
  if (params.endpoint) qp.set('endpoint', params.endpoint);
  if (params.limit) qp.set('limit', params.limit.toString());
  const resp = await fetch(`${API_BASE}/traces?${qp.toString()}`);
  if (!resp.ok) throw new Error('Failed to fetch traces for bucket');
  const data = (await resp.json()) as string[];
  return data;
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

export async function fetchTraceSpans(traceId: string): Promise<SpanRow[]> {
  const resp = await fetch(`${API_BASE}/trace/${traceId}`);
  if (!resp.ok) throw new Error('Failed to fetch trace spans');
  const data = (await resp.json()) as SpanRow[];
  return data;
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

export interface Incident {
  id: string;
  service: string;
  endpoint: string;
  description: string;
  created_at: string; // RFC3339
  state: string; // 'triggered' | 'acknowledged' | 'resolved'
  acknowledged_at?: string;
  resolved_at?: string;
}

export async function fetchCurrentOnCall(): Promise<string> {
  const resp = await fetch(`${API_BASE}/oncall/current`);
  if (!resp.ok) throw new Error('Failed to fetch current on‑call');
  const data = await resp.json();
  return data.on_call as string;
}

export async function fetchIncidents(): Promise<Incident[]> {
  const resp = await fetch(`${API_BASE}/oncall/incidents`);
  if (!resp.ok) throw new Error('Failed to fetch incidents');
  const data = (await resp.json()) as Incident[];
  return data;
}

export async function createIncident(payload: {service: string; endpoint: string; description: string;}): Promise<Incident> {
  const resp = await fetch(`${API_BASE}/oncall/incidents`, {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify(payload),
  });
  if (!resp.ok) throw new Error('Failed to create incident');
  const data = (await resp.json()) as Incident;
  return data;
}

export async function ackIncident(id: string): Promise<Incident> {
  const resp = await fetch(`${API_BASE}/oncall/incidents/${id}/ack`, {method: 'POST'});
  if (!resp.ok) throw new Error('Failed to acknowledge incident');
  const data = (await resp.json()) as Incident;
  return data;
}

export async function resolveIncident(id: string): Promise<Incident> {
  const resp = await fetch(`${API_BASE}/oncall/incidents/${id}/resolve`, {method: 'POST'});
  if (!resp.ok) throw new Error('Failed to resolve incident');
  const data = (await resp.json()) as Incident;
  return data;
}
