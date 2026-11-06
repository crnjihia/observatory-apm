// path: packages/agent-node/test/agent.test.ts
import { test } from 'node:test';
import * as assert from 'node:assert';
import { observatory, Span } from '../src/index.js';

test('ObservatoryAgent initialization', () => {
  observatory.init({
    service: 'test-mpesa-service',
    endpoint: 'http://localhost:8080',
    sampleRate: 1.0,
  });

  assert.ok(observatory.logger, 'logger should be initialized');
});

test('startSpan executes callback and captures trace context', async () => {
  observatory.init({
    service: 'daraja-b2c',
    endpoint: 'http://localhost:8080',
  });

  let capturedTraceId = '';
  let capturedSpanId = '';

  await observatory.startSpan('process-b2c-callback', async (ctx) => {
    capturedTraceId = ctx.traceId;
    capturedSpanId = ctx.spanId;
    assert.strictEqual(ctx.traceId.length, 32, 'traceId must be 32 hex chars');
    assert.strictEqual(ctx.spanId.length, 16, 'spanId must be 16 hex chars');
  });

  assert.ok(capturedTraceId, 'trace ID must be captured');
  assert.ok(capturedSpanId, 'span ID must be captured');
});

test('Logger automatically correlates trace_id and span_id into log lines', async () => {
  observatory.init({
    service: 'ussd-menu',
    endpoint: 'http://localhost:8080',
  });

  await observatory.startSpan('render-ussd-menu', async (ctx) => {
    const log = observatory.logger.info('User navigated to balance check', {
      phone: '+254711000001',
      menu: '*384*100#',
    });

    assert.strictEqual(log.trace_id, ctx.traceId, 'log trace_id must match span context');
    assert.strictEqual(log.span_id, ctx.spanId, 'log span_id must match span context');
    assert.strictEqual(log.service, 'ussd-menu');
    assert.strictEqual(log.attrs.phone, '+254711000001');
  });
});

