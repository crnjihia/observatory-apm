// path: demo/src/mpesa.ts
import * as crypto from 'node:crypto';

export interface TelemetrySpan {
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

export interface MpesaSimulationResult {
  traceId: string;
  receiptNumber: string;
  phone: string;
  amount: number;
  durationMs: number;
  success: boolean;
  spans: TelemetrySpan[];
}

const KENYAN_PHONE_PREFIXES = ['+254712', '+254722', '+254790', '+254701', '+254740'];

export function getRandomKenyanPhone(): string {
  const prefix = KENYAN_PHONE_PREFIXES[Math.floor(Math.random() * KENYAN_PHONE_PREFIXES.length)];
  const suffix = Math.floor(100000 + Math.random() * 900000).toString();
  return prefix + suffix;
}

export function generateReceiptNumber(): string {
  const letters = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ';
  let prefix = '';
  for (let i = 0; i < 3; i++) {
    prefix += letters[Math.floor(Math.random() * letters.length)];
  }
  const digits = Math.floor(1000000 + Math.random() * 9000000).toString();
  return prefix + digits;
}

export async function simulateMpesaTransaction(isAnomaly: boolean = false): Promise<MpesaSimulationResult> {
  const traceId = crypto.randomBytes(16).toString('hex');
  const rootSpanId = crypto.randomBytes(8).toString('hex');
  const childSpanId = crypto.randomBytes(8).toString('hex');
  const ledgerSpanId = crypto.randomBytes(8).toString('hex');

  const phone = getRandomKenyanPhone();
  const receipt = generateReceiptNumber();
  const amount = Math.floor(50 + Math.random() * 45000); // KES 50 to 45,000

  const startTime = new Date();
  const startIso = startTime.toISOString();

  let rootDurationMs: number;
  let status: string;
  let errorMsg: string | undefined;

  if (isAnomaly) {
    // Safaricom Daraja 504 Timeout Spike (> 1500ms SLA breach)
    rootDurationMs = 2200 + Math.floor(Math.random() * 950); // 2200ms to 3150ms
    status = 'STATUS_CODE_ERROR';
    errorMsg = 'Safaricom Daraja 504 Gateway Timeout: Callback request timed out after 3000ms';
  } else {
    // Nominal STK callback
    rootDurationMs = 45 + Math.floor(Math.random() * 75); // 45ms to 120ms
    status = 'STATUS_CODE_OK';
  }

  const rootDurationNs = rootDurationMs * 1_000_000;
  const childDurationNs = Math.floor(rootDurationNs * 0.7);
  const ledgerDurationNs = Math.floor(rootDurationNs * 0.25);

  const spans: TelemetrySpan[] = [
    // 1. Root Span: HTTP POST /v1/callbacks/mpesa
    {
      start_ts: startIso,
      trace_id: traceId,
      span_id: rootSpanId,
      service: 'mpesa-payment-gateway',
      name: 'HTTP POST /v1/callbacks/mpesa',
      kind: 'SPAN_KIND_SERVER',
      duration_ns: rootDurationNs,
      status: status,
      attrs: {
        'http.method': 'POST',
        'http.url': 'https://api.safaricom.co.ke/mpesa/c2b/v1/simulate',
        'http.status_code': isAnomaly ? 504 : 200,
        'mpesa.phone': phone,
        'mpesa.receipt': receipt,
        'mpesa.amount_kes': amount,
        'mpesa.currency': 'KES',
        'mpesa.paybill': '174379',
        ...(errorMsg ? { 'error': true, 'error.message': errorMsg } : {}),
      },
    },
    // 2. Child Span: Daraja.VerifyCallbackSignature
    {
      start_ts: new Date(startTime.getTime() + 5).toISOString(),
      trace_id: traceId,
      span_id: childSpanId,
      parent_id: rootSpanId,
      service: 'daraja-client',
      name: 'Daraja.VerifyCallbackSignature',
      kind: 'SPAN_KIND_INTERNAL',
      duration_ns: childDurationNs,
      status: status,
      attrs: {
        'daraja.merchant_id': 'sandbox_merchant_001',
        'daraja.security_credential': 'sha256_verified',
        ...(isAnomaly ? { 'error.reason': 'Gateway timeout waiting for Safaricom KMS cert signature' } : {}),
      },
    },
    // 3. Child Span: Ledger.CreditFloatAccount
    {
      start_ts: new Date(startTime.getTime() + 15).toISOString(),
      trace_id: traceId,
      span_id: ledgerSpanId,
      parent_id: rootSpanId,
      service: 'ledger-service',
      name: 'Ledger.CreditFloatAccount',
      kind: 'SPAN_KIND_CLIENT',
      duration_ns: ledgerDurationNs,
      status: isAnomaly ? 'STATUS_CODE_ERROR' : 'STATUS_CODE_OK',
      attrs: {
        'db.system': 'postgresql',
        'db.table': 'customer_float_wallets',
        'account.phone': phone,
        'account.amount_kes': amount,
        'db.statement': 'UPDATE customer_float_wallets SET balance = balance + $1 WHERE phone = $2',
      },
    },
  ];

  return {
    traceId,
    receiptNumber: receipt,
    phone,
    amount,
    durationMs: rootDurationMs,
    success: !isAnomaly,
    spans,
  };
}

