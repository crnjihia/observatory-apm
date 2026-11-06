// path: packages/agent-node/src/logger.ts
import { AsyncLocalStorage } from 'node:async_hooks';

export interface SpanContext {
  traceId: string;
  spanId: string;
}

export const traceStorage = new AsyncLocalStorage<SpanContext>();

export interface LogPayload {
  ts: string;
  service: string;
  severity: string;
  body: string;
  trace_id?: string;
  span_id?: string;
  attrs: Record<string, unknown>;
}

export type LogExporter = (log: LogPayload) => void;

export class Logger {
  private service: string;
  private exporter?: LogExporter;

  constructor(service: string, exporter?: LogExporter) {
    this.service = service;
    this.exporter = exporter;
  }

  private log(severity: string, message: string, attrs: Record<string, unknown> = {}): LogPayload {
    const context = traceStorage.getStore();
    const payload: LogPayload = {
      ts: new Date().toISOString(),
      service: this.service,
      severity,
      body: message,
      trace_id: context?.traceId,
      span_id: context?.spanId,
      attrs: {
        ...attrs,
        pid: process.pid,
      },
    };

    // Print JSON log to stdout
    const serialized = JSON.stringify(payload);
    if (severity === 'ERROR' || severity === 'FATAL') {
      process.stderr.write(serialized + '\n');
    } else {
      process.stdout.write(serialized + '\n');
    }

    if (this.exporter) {
      this.exporter(payload);
    }

    return payload;
  }

  public info(message: string, attrs?: Record<string, unknown>): LogPayload {
    return this.log('INFO', message, attrs);
  }

  public warn(message: string, attrs?: Record<string, unknown>): LogPayload {
    return this.log('WARN', message, attrs);
  }

  public error(message: string, attrs?: Record<string, unknown>): LogPayload {
    return this.log('ERROR', message, attrs);
  }

  public debug(message: string, attrs?: Record<string, unknown>): LogPayload {
    return this.log('DEBUG', message, attrs);
  }
}

