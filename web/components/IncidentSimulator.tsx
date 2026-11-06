// path: web/components/IncidentSimulator.tsx
import React, { useState } from 'react';
import { createIncident } from '../utils/api';

interface IncidentSimulatorProps {
  onIncidentInjected?: () => void;
}

interface IncidentScenario {
  id: string;
  title: string;
  service: string;
  endpoint: string;
  severity: 'CRITICAL' | 'WARNING';
  channels: string[];
  description: string;
  expectedSlaViolation: string;
}

export const IncidentSimulator: React.FC<IncidentSimulatorProps> = ({ onIncidentInjected }) => {
  const [loadingId, setLoadingId] = useState<string | null>(null);
  const [lastInjected, setLastInjected] = useState<{
    title: string;
    incidentId: string;
    timestamp: string;
  } | null>(null);
  const [error, setError] = useState<string>('');

  const scenarios: IncidentScenario[] = [
    {
      id: 'daraja_504',
      title: 'Safaricom Daraja 504 Gateway Timeout Spike',
      service: 'mpesa-payment-gateway',
      endpoint: '/v1/callbacks/mpesa',
      severity: 'CRITICAL',
      channels: ['SMS (Africa\'s Talking)', 'Slack'],
      description: 'Forces M-Pesa callback latencies to spike to ~2,850ms, breaching 1500ms SLA deadline.',
      expectedSlaViolation: 'P99 Latency > 1500ms SLA Breach',
    },
    {
      id: 'ussd_disconnect',
      title: 'USSD Telco Channel Network Reset',
      service: 'ussd-banking-session',
      endpoint: '/ussd/v2/menu',
      severity: 'CRITICAL',
      channels: ['SMS (Africa\'s Talking)', 'Slack'],
      description: 'Simulates telco network disconnects aborting active banking sessions (*384*100#).',
      expectedSlaViolation: 'Error Rate > 2.0% Anomaly',
    },
    {
      id: 'b2c_float_depletion',
      title: 'M-Pesa B2C Float Depletion',
      service: 'mpesa-payment-gateway',
      endpoint: '/v1/disbursements/b2c',
      severity: 'CRITICAL',
      channels: ['SMS (Africa\'s Talking)', 'Slack'],
      description: 'Simulates float balance exhaustion causing rollback of customer withdrawal disbursements.',
      expectedSlaViolation: 'Float Account Balance Zero',
    },
    {
      id: 'db_deadlock',
      title: 'PostgreSQL Ledger Lock Deadlock',
      service: 'ledger-service',
      endpoint: '/v1/wallets/debit',
      severity: 'WARNING',
      channels: ['Slack'],
      description: 'Simulates database lock contention on wallet rows under high-volume concurrency.',
      expectedSlaViolation: 'P95 Latency > 800ms Degradation',
    },
  ];

  const handleInjectFault = async (scenario: IncidentScenario) => {
    setLoadingId(scenario.id);
    setError('');

    try {
      // 1. Trigger incident in Observatory On-Call Store
      const inc = await createIncident({
        service: scenario.service,
        endpoint: scenario.endpoint,
        description: `${scenario.title}: ${scenario.description}`,
      });

      // 2. Also notify demo simulator if reachable
      try {
        await fetch('http://localhost:4000/simulate/fault', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ fault: scenario.id }),
        });
      } catch {
        // demo simulator is optional; on-call store handles the state
      }

      setLastInjected({
        title: scenario.title,
        incidentId: inc.id,
        timestamp: new Date().toLocaleTimeString('en-GB') + ' EAT',
      });

      if (onIncidentInjected) {
        onIncidentInjected();
      }
    } catch (err: any) {
      setError(`Failed to inject incident: ${err.message}`);
    } finally {
      setLoadingId(null);
    }
  };

  return (
    <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 mb-8 shadow-sm">
      <div className="flex flex-wrap items-center justify-between gap-2 mb-4">
        <div>
          <h2 className="text-sm font-semibold text-white tracking-tight flex items-center">
            <span className="w-2 h-2 rounded-full bg-amber-400 mr-2"></span>
            Kenyan Infrastructure Fault & Anomaly Simulator
          </h2>
          <p className="text-xs text-slate-400 mt-0.5">
            Inject realistic faults to verify on-call paging, Africa&apos;s Talking SMS, and Slack notifications.
          </p>
        </div>
        <div className="px-2.5 py-1 rounded-md bg-slate-800/80 border border-slate-700/80 text-[11px] font-mono text-slate-300">
          Mode: <strong className="text-emerald-400">Live Interactive Drill</strong>
        </div>
      </div>

      {error && (
        <div className="mb-4 p-3 bg-rose-950/40 border border-rose-800 text-rose-300 text-xs rounded-lg">
          {error}
        </div>
      )}

      {lastInjected && (
        <div className="mb-4 p-3.5 bg-emerald-950/30 border border-emerald-500/40 rounded-xl text-xs text-emerald-200 flex items-center justify-between">
          <div className="flex items-center space-x-2.5">
            <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
            <div>
              <strong className="text-white font-semibold">{lastInjected.title}</strong> injected successfully.
              <span className="text-slate-400 font-mono text-[11px] ml-2">
                Incident ID: {lastInjected.incidentId} • {lastInjected.timestamp}
              </span>
            </div>
          </div>
          <span className="px-2 py-0.5 rounded text-[10px] bg-emerald-500/20 text-emerald-300 border border-emerald-500/30 font-semibold">
            Paging Triggered
          </span>
        </div>
      )}

      {/* 4 Realistic Fault Scenarios Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-3.5">
        {scenarios.map((s) => {
          const isLoading = loadingId === s.id;
          return (
            <div
              key={s.id}
              className="p-4 rounded-xl border border-slate-800/90 bg-slate-950/60 flex flex-col justify-between hover:border-slate-700 transition-all"
            >
              <div>
                <div className="flex items-center justify-between mb-1.5">
                  <span
                    className={`px-1.5 py-0.2 rounded text-[10px] font-bold ${
                      s.severity === 'CRITICAL'
                        ? 'bg-rose-500/20 text-rose-300 border border-rose-500/40'
                        : 'bg-amber-500/20 text-amber-300 border border-amber-500/40'
                    }`}
                  >
                    {s.severity}
                  </span>
                  <span className="text-[11px] font-mono text-slate-400">
                    {s.service}
                  </span>
                </div>

                <h3 className="text-xs font-bold text-white mb-1">{s.title}</h3>
                <p className="text-[11px] text-slate-400 mb-3">{s.description}</p>
              </div>

              <div className="pt-2.5 border-t border-slate-800/80 flex items-center justify-between gap-2">
                <div className="text-[10px] font-mono text-slate-500 truncate">
                  ⚡ {s.expectedSlaViolation}
                </div>

                <button
                  onClick={() => handleInjectFault(s)}
                  disabled={isLoading}
                  className="px-3 py-1.5 rounded-lg text-xs font-semibold bg-indigo-600 hover:bg-indigo-500 text-white transition-colors disabled:opacity-50 shrink-0 shadow-sm"
                >
                  {isLoading ? 'Injecting…' : 'Inject Incident'}
                </button>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
};

