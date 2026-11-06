// path: packages/agent-node/src/index.ts
import * as crypto from 'node:crypto';
import * as http from 'node:http';
import { Logger, LogPayload, SpanContext, traceStorage } from './logger.js';

export interface InitOptions {
  service: string;
  endpoint?: string;
  apiKey?: string;
  sampleRate?: number;
  flushIntervalMs?: number;
  batchSize?: number;
}

export interface Span {
  start_ts: string;
  trace_id: string;
  span_id: string;
  parent_id?: string;
  service: string;
  name: string;
  kind: string;
  duration_ns: number;
  status: string;
  attrs: Record<string, unknown>;
}

export class ObservatoryAgent {
  private service: string = 'unknown-service';
  private endpoint: string = 'http://localhost:8080';
  private apiKey?: string;
  private sampleRate: number = 0.1; // Default to 10% head-based ratio sampling
  private flushIntervalMs: number = 500;
  private batchSize: number = 100;

  private spanBuffer: Span[] = [];
  private logBuffer: LogPayload[] = [];
  private flushTimer?: NodeJS.Timeout;

  public logger!: Logger;

  public init(options: InitOptions): this {
    this.service = options.service;
    this.endpoint = (options.endpoint || 'http://localhost:8080').replace(/\/$/, '');
    this.apiKey = options.apiKey;
    this.sampleRate = options.sampleRate ?? 0.1;
    this.flushIntervalMs = options.flushIntervalMs ?? 500;
    this.batchSize = options.batchSize ?? 100;

    this.logger = new Logger(this.service, (log) => {
      this.logBuffer.push(log);
      if (this.logBuffer.length >= this.batchSize) {
        this.flushLogs();
      }
    });

    if (this.flushTimer) {
      clearInterval(this.flushTimer);
    }
    this.flushTimer = setInterval(() => {
      this.flushSpans();
      this.flushLogs();
    }, this.flushIntervalMs);
    this.flushTimer.unref();

    return this;
  }

  private shouldSampleTrace(traceId: string): boolean {
    if (this.sampleRate >= 1.0) return true;
    if (this.sampleRate <= 0.0) return false;
    try {
      const clean = traceId.replace(/[^0-9a-fA-F]/g, '');
      if (clean.length >= 16) {
        const val = BigInt('0x' + clean.slice(-16));
        const threshold = BigInt(Math.floor(this.sampleRate * 18446744073709551615));
        return val < threshold;
      }
    } catch {}
    return Math.random() < this.sampleRate;
  }

  public recordSpan(span: Span): void {
    // 100% capture of error spans; deterministic 64-bit head-based ratio sampling for others
    if (span.status !== 'STATUS_CODE_ERROR' && !this.shouldSampleTrace(span.trace_id)) {
      return;
    }

    this.spanBuffer.push(span);
    if (this.spanBuffer.length >= this.batchSize) {
      this.flushSpans();
    }
  }

  public async startSpan<T>(name: string, fn: (context: SpanContext) => Promise<T> | T): Promise<T> {
    const parentContext = traceStorage.getStore();
    const traceId = parentContext?.traceId || crypto.randomBytes(16).toString('hex');
    const spanId = crypto.randomBytes(8).toString('hex');
    const startNanos = process.hrtime.bigint();
    const startIso = new Date().toISOString();

    const context: SpanContext = { traceId, spanId };

    let status = 'STATUS_CODE_OK';
    let caughtError: unknown;

    try {
      return await traceStorage.run(context, async () => {
        return await fn(context);
      });
    } catch (err) {
      status = 'STATUS_CODE_ERROR';
      caughtError = err;
      throw err;
    } finally {
      const endNanos = process.hrtime.bigint();
      const durationNs = Number(endNanos - startNanos);

      const span: Span = {
        start_ts: startIso,
        trace_id: traceId,
        span_id: spanId,
        parent_id: parentContext?.spanId,
        service: this.service,
        name,
        kind: parentContext ? 'SPAN_KIND_INTERNAL' : 'SPAN_KIND_SERVER',
        duration_ns: durationNs,
        status,
        attrs: {
          'telemetry.sdk.language': 'nodejs',
          'telemetry.sdk.name': 'observatory-agent-node',
          ...(caughtError ? { 'error.message': String(caughtError) } : {}),
        },
      };

      this.recordSpan(span);
    }
  }

  // Express / Node HTTP Middleware for Auto-Instrumentation
  public middleware() {
    return (req: http.IncomingMessage & { path?: string; originalUrl?: string }, res: http.ServerResponse, next: () => void) => {
      const headerTraceParent = req.headers['traceparent'] as string | undefined;
      let traceId = crypto.randomBytes(16).toString('hex');
      let parentSpanId: string | undefined;

      if (headerTraceParent && headerTraceParent.startsWith('00-')) {
        const parts = headerTraceParent.split('-');
        if (parts.length >= 3 && parts[1].length === 32) {
          traceId = parts[1];
          parentSpanId = parts[2];
        }
      }

      const spanId = crypto.randomBytes(8).toString('hex');
      const startNanos = process.hrtime.bigint();
      const startIso = new Date().toISOString();
      const endpoint = req.originalUrl || req.url || '/';

      const context: SpanContext = { traceId, spanId };

      res.setHeader('traceparent', `00-${traceId}-${spanId}-01`);

      res.on('finish', () => {
        const endNanos = process.hrtime.bigint();
        const durationNs = Number(endNanos - startNanos);
        const statusCode = res.statusCode || 200;
        const status = statusCode >= 500 ? 'STATUS_CODE_ERROR' : 'STATUS_CODE_OK';

        this.recordSpan({
          start_ts: startIso,
          trace_id: traceId,
          span_id: spanId,
          parent_id: parentSpanId,
          service: this.service,
          name: endpoint,
          kind: 'SPAN_KIND_SERVER',
          duration_ns: durationNs,
          status,
          attrs: {
            'http.status_code': statusCode,
            'http.method': req.method || 'GET',
            'telemetry.sdk.name': 'observatory-agent-node',
          },
        });
      });

      traceStorage.run(context, () => {
        next();
      });
    };
  }

  public flushSpans(): void {
    if (this.spanBuffer.length === 0) return;
    const batch = this.spanBuffer.splice(0, this.batchSize);
    this.sendPost('/v1/traces', batch);
  }

  public flushLogs(): void {
    if (this.logBuffer.length === 0) return;
    const batch = this.logBuffer.splice(0, this.batchSize);
    this.sendPost('/v1/logs', batch);
  }

  private sendPost(path: string, payload: unknown): void {
    try {
      const data = JSON.stringify(payload);
      const url = new URL(this.endpoint + path);

      const req = http.request(
        url,
        {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Content-Length': Buffer.byteLength(data),
            ...(this.apiKey ? { 'X-API-Key': this.apiKey } : {}),
          },
          timeout: 2000,
        },
        (res) => {
          res.resume(); // discard body
        }
      );

      req.on('error', () => {
        // Silently drop errors to prevent APM failures from impacting app runtime
      });

      req.write(data);
      req.end();
    } catch {
      // Non-fatal network exception
    }
  }
}

export const observatory = new ObservatoryAgent();
export { Logger } from './logger.js';

