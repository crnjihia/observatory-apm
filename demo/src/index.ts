// path: demo/src/index.ts
import express, { Request, Response } from 'express';
import cors from 'cors';
import { simulateMpesaTransaction } from './mpesa.js';
import { simulateUssdSession } from './ussd.js';

const app = express();
const port = parseInt(process.env.PORT || '4000', 10);
const ingestUrl = process.env.OBSERVATORY_INGEST_URL || 'http://localhost:8080/v1/traces';

app.use(cors());
app.use(express.json());

// In-memory traffic metrics
let totalSimulatedTransactions = 0;
let totalErrorsInjected = 0;

// Helper to push spans to Observatory APM API
async function exportSpansToAPM(spans: any[]): Promise<boolean> {
  try {
    const res = await fetch(ingestUrl, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-API-Key': process.env.OBSERVATORY_API_KEY || 'obs_sandbox_sec_key_nairobi_2026',
      },
      body: JSON.stringify(spans),
    });
    return res.ok;
  } catch {
    // If local ingest API isn't running yet, continue smoothly
    return false;
  }
}

app.get('/health', (_req: Request, res: Response) => {
  res.json({
    status: 'ok',
    service: 'observatory-traffic-simulator',
    ingestUrl,
    stats: {
      totalSimulatedTransactions,
      totalErrorsInjected,
    },
  });
});

app.get('/stats', (_req: Request, res: Response) => {
  res.json({
    totalSimulatedTransactions,
    totalErrorsInjected,
    uptime_sec: Math.floor(process.uptime()),
  });
});

// Trigger M-Pesa transaction
app.post('/simulate/mpesa', async (req: Request, res: Response) => {
  const isAnomaly = req.query.anomaly === 'true' || req.body?.anomaly === true;
  const result = await simulateMpesaTransaction(isAnomaly);

  totalSimulatedTransactions++;
  if (isAnomaly) totalErrorsInjected++;

  const forwarded = await exportSpansToAPM(result.spans);
  res.json({
    ...result,
    forwardedToAPM: forwarded,
  });
});

// Trigger USSD session
app.post('/simulate/ussd', async (req: Request, res: Response) => {
  const isAnomaly = req.query.anomaly === 'true' || req.body?.anomaly === true;
  const result = await simulateUssdSession(isAnomaly);

  totalSimulatedTransactions++;
  if (isAnomaly) totalErrorsInjected++;

  const forwarded = await exportSpansToAPM(result.spans);
  res.json({
    ...result,
    forwardedToAPM: forwarded,
  });
});

// Inject Named Incident Fault
app.post('/simulate/fault', async (req: Request, res: Response) => {
  const faultType = req.body?.fault || req.query.fault || 'daraja_504';
  let result: any;

  switch (faultType) {
    case 'ussd_disconnect':
      result = await simulateUssdSession(true);
      break;
    case 'daraja_504':
    default:
      result = await simulateMpesaTransaction(true);
      break;
  }

  totalSimulatedTransactions++;
  totalErrorsInjected++;

  const forwarded = await exportSpansToAPM(result.spans);
  res.status(200).json({
    fault: faultType,
    message: `Incident fault '${faultType}' successfully injected into telemetry stream`,
    traceId: result.traceId,
    durationMs: result.durationMs,
    forwardedToAPM: forwarded,
    spansGenerated: result.spans.length,
  });
});

app.listen(port, () => {
  console.log(`[Observatory Demo Simulator] Running on http://localhost:${port}`);
  console.log(`[Observatory Demo Simulator] Telemetry target: ${ingestUrl}`);
});
