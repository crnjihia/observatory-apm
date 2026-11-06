// path: demo/src/ussd.ts
import * as crypto from 'node:crypto';
import { TelemetrySpan, getRandomKenyanPhone } from './mpesa.js';

export interface UssdSimulationResult {
  traceId: string;
  sessionId: string;
  phone: string;
  shortCode: string;
  menu: string;
  durationMs: number;
  success: boolean;
  spans: TelemetrySpan[];
}

const SHORTCODES = ['*384*100#', '*144#', '*544#', '*334#'];
const MENUS = [
  '1. Account Balance Check',
  '2. Buy Airtime for My Phone',
  '3. Send Money to Other Network',
  '4. Mini-Statement (Last 5 Txns)',
];

export async function simulateUssdSession(isAnomaly: boolean = false): Promise<UssdSimulationResult> {
  const traceId = crypto.randomBytes(16).toString('hex');
  const rootSpanId = crypto.randomBytes(8).toString('hex');
  const sessionDbSpanId = crypto.randomBytes(8).toString('hex');

  const sessionId = 'USSD_SES_' + crypto.randomBytes(6).toString('hex').toUpperCase();
  const phone = getRandomKenyanPhone();
  const shortCode = SHORTCODES[Math.floor(Math.random() * SHORTCODES.length)];
  const menu = MENUS[Math.floor(Math.random() * MENUS.length)];

  const startTime = new Date();
  const startIso = startTime.toISOString();

  let rootDurationMs: number;
  let status: string;
  let errorMsg: string | undefined;

  if (isAnomaly) {
    // USSD Telco Session Disconnect
    rootDurationMs = 750 + Math.floor(Math.random() * 650); // 750ms to 1400ms
    status = 'STATUS_CODE_ERROR';
    errorMsg = 'USSD session dropped by telco: Network channel disconnect before state completion';
  } else {
    // Nominal USSD render
    rootDurationMs = 35 + Math.floor(Math.random() * 55); // 35ms to 90ms
    status = 'STATUS_CODE_OK';
  }

  const rootDurationNs = rootDurationMs * 1_000_000;
  const dbDurationNs = Math.floor(rootDurationNs * 0.4);

  const spans: TelemetrySpan[] = [
    // 1. Root Span: HTTP POST /ussd/v2/menu
    {
      start_ts: startIso,
      trace_id: traceId,
      span_id: rootSpanId,
      service: 'ussd-banking-session',
      name: 'HTTP POST /ussd/v2/menu',
      kind: 'SPAN_KIND_SERVER',
      duration_ns: rootDurationNs,
      status: status,
      attrs: {
        'http.method': 'POST',
        'http.status_code': isAnomaly ? 500 : 200,
        'ussd.session_id': sessionId,
        'ussd.phone': phone,
        'ussd.shortcode': shortCode,
        'ussd.menu_state': 'CON',
        'ussd.active_selection': menu,
        ...(errorMsg ? { 'error': true, 'error.message': errorMsg } : {}),
      },
    },
    // 2. Child Span: Redis.FetchSessionState
    {
      start_ts: new Date(startTime.getTime() + 5).toISOString(),
      trace_id: traceId,
      span_id: sessionDbSpanId,
      parent_id: rootSpanId,
      service: 'ussd-session-store',
      name: 'Redis.FetchSessionState',
      kind: 'SPAN_KIND_CLIENT',
      duration_ns: dbDurationNs,
      status: 'STATUS_CODE_OK',
      attrs: {
        'db.system': 'redis',
        'db.operation': 'HGETALL',
        'redis.key': `session:${sessionId}`,
      },
    },
  ];

  return {
    traceId,
    sessionId,
    phone,
    shortCode,
    menu,
    durationMs: rootDurationMs,
    success: !isAnomaly,
    spans,
  };
}

