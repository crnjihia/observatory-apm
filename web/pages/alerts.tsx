// path: web/pages/alerts.tsx
import React, { useEffect, useState, useCallback } from 'react';
import Head from 'next/head';
import Link from 'next/link';
import {
  fetchAlertRules,
  toggleAlertRule,
  fetchActiveAlerts,
  fetchAlertHistory,
  ackAlert,
  resolveAlert,
  AlertRule,
  ActiveAlertItem,
} from '../utils/api';

export default function AlertsPage() {
  const [rules, setRules] = useState<AlertRule[]>([]);
  const [activeAlerts, setActiveAlerts] = useState<ActiveAlertItem[]>([]);
  const [history, setHistory] = useState<ActiveAlertItem[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [actionMessage, setActionMessage] = useState<string>('');

  const loadAlertData = useCallback(() => {
    setLoading(true);
    Promise.all([fetchAlertRules(), fetchActiveAlerts(), fetchAlertHistory()])
      .then(([r, a, h]) => {
        setRules(r);
        setActiveAlerts(a);
        setHistory(h);
      })
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    loadAlertData();
    const interval = setInterval(loadAlertData, 15000); // 15s auto-refresh
    return () => clearInterval(interval);
  }, [loadAlertData]);

  const handleToggleRule = async (ruleId: string, currentEnabled: boolean) => {
    try {
      await toggleAlertRule(ruleId, !currentEnabled);
      setRules((prev) =>
        prev.map((r) => (r.id === ruleId ? { ...r, enabled: !currentEnabled } : r))
      );
      setActionMessage(`Rule ${ruleId} ${!currentEnabled ? 'enabled' : 'disabled'}`);
      setTimeout(() => setActionMessage(''), 3000);
    } catch (err: any) {
      setActionMessage(`Failed to toggle rule: ${err.message}`);
    }
  };

  const handleAcknowledge = async (alertId: string) => {
    try {
      await ackAlert(alertId);
      setActionMessage(`Alert ${alertId} acknowledged.`);
      loadAlertData();
      setTimeout(() => setActionMessage(''), 3000);
    } catch (err: any) {
      setActionMessage(`Failed to acknowledge: ${err.message}`);
    }
  };

  const handleResolve = async (alertId: string) => {
    try {
      await resolveAlert(alertId);
      setActionMessage(`Alert ${alertId} resolved.`);
      loadAlertData();
      setTimeout(() => setActionMessage(''), 3000);
    } catch (err: any) {
      setActionMessage(`Failed to resolve: ${err.message}`);
    }
  };

  return (
    <>
      <Head>
        <title>Observatory APM – Statistical Alerting & SLA Rules</title>
        <meta name="description" content="Real-time multi-channel alerting for Kenyan fintech infrastructures." />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
      </Head>

      <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col font-sans">
        {/* Navigation Bar */}
        <header className="sticky top-0 z-40 w-full bg-slate-900/95 backdrop-blur border-b border-slate-800 text-slate-200">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
            <div className="flex items-center justify-between h-16">
              <div className="flex items-center space-x-6">
                <Link href="/" className="flex items-center space-x-3 group">
                  <div className="relative flex items-center justify-center w-9 h-9 rounded-lg bg-indigo-950 border border-indigo-500/30 text-indigo-400">
                    <svg className="w-5 h-5 animate-pulse" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
                    </svg>
                  </div>
                  <span className="text-lg font-bold tracking-tight text-white">Observatory</span>
                  <span className="ml-1 px-1.5 py-0.5 text-[10px] font-semibold tracking-wide uppercase rounded bg-indigo-500/20 text-indigo-300 border border-indigo-500/30">
                    Alerts
                  </span>
                </Link>

                <nav className="hidden md:flex space-x-1">
                  <Link
                    href="/"
                    className="px-3 py-1.5 text-sm font-medium rounded-md text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
                  >
                    Dashboard
                  </Link>
                  <Link
                    href="/alerts"
                    className="px-3 py-1.5 text-sm font-medium rounded-md bg-slate-800 text-white border border-slate-700"
                  >
                    Alerts & Rules
                  </Link>
                  <Link
                    href="/oncall"
                    className="px-3 py-1.5 text-sm font-medium rounded-md text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
                  >
                    On-Call & Incidents
                  </Link>
                </nav>
              </div>

              <div className="flex items-center space-x-3 text-xs">
                <div className="flex items-center space-x-1.5 px-2.5 py-1 rounded-md bg-slate-800/60 border border-slate-700/80 font-mono text-slate-300">
                  <span className="w-2 h-2 rounded-full bg-emerald-400"></span>
                  <span>Africa&apos;s Talking & Slack Active</span>
                </div>
              </div>
            </div>
          </div>
        </header>

        {/* Main Content Area */}
        <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-6">
          {actionMessage && (
            <div className="mb-4 p-3 bg-indigo-950/60 border border-indigo-500/40 text-indigo-200 text-xs rounded-lg animate-fade-in">
              {actionMessage}
            </div>
          )}

          {/* Status Metric Strip */}
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6">
            <div className="bg-slate-900 border border-slate-800 rounded-xl p-4">
              <span className="text-slate-400 text-xs font-medium block mb-1">Active Alerts</span>
              <div className="text-2xl font-bold text-white flex items-center">
                {activeAlerts.length}
                {activeAlerts.length > 0 && (
                  <span className="ml-2 px-2 py-0.5 rounded text-[10px] font-semibold bg-rose-500/20 text-rose-300 border border-rose-500/30 animate-pulse">
                    Action Needed
                  </span>
                )}
              </div>
            </div>
            <div className="bg-slate-900 border border-slate-800 rounded-xl p-4">
              <span className="text-slate-400 text-xs font-medium block mb-1">Configured Rules</span>
              <div className="text-2xl font-bold text-white">
                {rules.length}
                <span className="text-xs font-normal text-slate-400 ml-1.5">
                  ({rules.filter((r) => r.enabled).length} enabled)
                </span>
              </div>
            </div>
            <div className="bg-slate-900 border border-slate-800 rounded-xl p-4">
              <span className="text-slate-400 text-xs font-medium block mb-1">Notification Channels</span>
              <div className="text-xs font-mono text-slate-300 space-y-1 mt-1">
                <div>📱 SMS: <strong className="text-white">Africa&apos;s Talking (E.164)</strong></div>
                <div>💬 Slack: <strong className="text-white">Webhook Blocks</strong></div>
              </div>
            </div>
          </div>

          {/* Section 1: Active Alerts */}
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 mb-8 shadow-sm">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-sm font-semibold text-white tracking-tight flex items-center">
                <span className="w-2 h-2 rounded-full bg-rose-500 mr-2"></span>
                Active Alerts & Incident Escalations ({activeAlerts.length})
              </h2>
              <button
                onClick={loadAlertData}
                className="text-xs text-indigo-400 hover:text-indigo-300 font-medium"
              >
                Refresh
              </button>
            </div>

            {activeAlerts.length > 0 ? (
              <div className="space-y-3">
                {activeAlerts.map((alt) => (
                  <div
                    key={alt.id}
                    className="p-4 rounded-xl border border-rose-500/30 bg-rose-950/10 flex flex-col md:flex-row md:items-center justify-between gap-4"
                  >
                    <div>
                      <div className="flex items-center space-x-2.5 mb-1.5">
                        <span className="px-2 py-0.5 rounded text-[10px] font-bold bg-rose-500/20 text-rose-300 border border-rose-500/40">
                          {alt.severity}
                        </span>
                        <span className="text-sm font-bold text-white">{alt.rule_name}</span>
                        <span className="text-xs text-slate-400 font-mono">[{alt.service}]</span>
                      </div>
                      <div className="text-xs text-slate-300 flex flex-wrap gap-x-4 gap-y-1 font-mono">
                        <span>
                          Observed:{' '}
                          <strong className="text-rose-400">{alt.observed_value.toFixed(1)}</strong>{' '}
                          (Threshold: {alt.threshold})
                        </span>
                        <span>Endpoint: {alt.endpoint}</span>
                        <span>
                          Triggered:{' '}
                          {new Date(alt.triggered_at).toLocaleTimeString('en-GB')} EAT
                        </span>
                        <span>Status: <strong className="text-amber-400">{alt.state}</strong></span>
                      </div>
                    </div>

                    <div className="flex items-center space-x-2 shrink-0">
                      {alt.state === 'TRIGGERED' && (
                        <button
                          onClick={() => handleAcknowledge(alt.id)}
                          className="px-3 py-1.5 text-xs font-semibold rounded-lg bg-amber-500/20 text-amber-300 border border-amber-500/30 hover:bg-amber-500/30 transition-colors"
                        >
                          Acknowledge
                        </button>
                      )}
                      <button
                        onClick={() => handleResolve(alt.id)}
                        className="px-3 py-1.5 text-xs font-semibold rounded-lg bg-emerald-500/20 text-emerald-300 border border-emerald-500/30 hover:bg-emerald-500/30 transition-colors"
                      >
                        Resolve
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="py-8 text-center text-slate-500 italic text-xs">
                No active alerts. All Kenyan payment gateways and USSD services operating nominally.
              </div>
            )}
          </div>

          {/* Section 2: Configured Alert Rules */}
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 mb-8 shadow-sm">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-sm font-semibold text-white tracking-tight flex items-center">
                <span className="w-2 h-2 rounded-full bg-indigo-500 mr-2"></span>
                Configured SLA & Anomaly Rules ({rules.length})
              </h2>
            </div>

            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs text-slate-300">
                <thead className="bg-slate-800/60 text-slate-400 uppercase text-[10px] font-semibold border-b border-slate-800">
                  <tr>
                    <th className="py-2.5 px-3">Rule Name & ID</th>
                    <th className="py-2.5 px-3">Scope (Service / Endpoint)</th>
                    <th className="py-2.5 px-3">Condition & Threshold</th>
                    <th className="py-2.5 px-3">Severity</th>
                    <th className="py-2.5 px-3">Channels</th>
                    <th className="py-2.5 px-3 text-right">Status</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800/60 font-mono">
                  {rules.map((rule) => (
                    <tr key={rule.id} className="hover:bg-slate-800/40 transition-colors">
                      <td className="py-3 px-3">
                        <div className="font-bold text-white font-sans text-xs">{rule.name}</div>
                        <div className="text-[10px] text-slate-500">{rule.id}</div>
                      </td>
                      <td className="py-3 px-3">
                        <div className="text-slate-200">{rule.service}</div>
                        <div className="text-[10px] text-slate-400">{rule.endpoint}</div>
                      </td>
                      <td className="py-3 px-3">
                        <span className="text-amber-400 font-semibold">{rule.metric}</span>{' '}
                        {rule.condition}{' '}
                        <strong className="text-white">{rule.threshold}</strong>
                        <div className="text-[10px] text-slate-500">
                          Cooldown: {rule.cooldown_sec}s
                        </div>
                      </td>
                      <td className="py-3 px-3">
                        <span
                          className={`px-1.5 py-0.2 rounded text-[10px] font-bold ${
                            rule.severity === 'CRITICAL'
                              ? 'bg-rose-500/20 text-rose-300 border border-rose-500/30'
                              : 'bg-amber-500/20 text-amber-300 border border-amber-500/30'
                          }`}
                        >
                          {rule.severity}
                        </span>
                      </td>
                      <td className="py-3 px-3">
                        <div className="flex items-center space-x-1">
                          {rule.channels.map((ch) => (
                            <span
                              key={ch}
                              className="px-1.5 py-0.5 rounded bg-slate-800 text-slate-300 text-[10px] uppercase border border-slate-700"
                            >
                              {ch}
                            </span>
                          ))}
                        </div>
                      </td>
                      <td className="py-3 px-3 text-right">
                        <button
                          onClick={() => handleToggleRule(rule.id, rule.enabled)}
                          className={`px-2.5 py-1 text-xs font-semibold rounded-lg transition-colors ${
                            rule.enabled
                              ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30 hover:bg-emerald-500/30'
                              : 'bg-slate-800 text-slate-400 border border-slate-700 hover:bg-slate-700'
                          }`}
                        >
                          {rule.enabled ? 'Enabled' : 'Disabled'}
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          {/* Section 3: Alert History */}
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-sm">
            <h2 className="text-sm font-semibold text-white tracking-tight flex items-center mb-4">
              <span className="w-2 h-2 rounded-full bg-slate-500 mr-2"></span>
              Alert Incident History ({history.length})
            </h2>

            <div className="divide-y divide-slate-800/60 font-mono text-xs">
              {history.map((alt) => (
                <div key={alt.id} className="py-2.5 flex items-center justify-between">
                  <div className="flex items-center space-x-3">
                    <span
                      className={`w-2 h-2 rounded-full ${
                        alt.state === 'RESOLVED' ? 'bg-emerald-400' : 'bg-rose-500'
                      }`}
                    />
                    <div>
                      <span className="text-white font-sans font-semibold mr-2">{alt.rule_name}</span>
                      <span className="text-slate-400 text-[11px]">[{alt.service}]</span>
                    </div>
                  </div>
                  <div className="text-[11px] text-slate-400">
                    Observed: <span className="text-amber-400">{alt.observed_value.toFixed(1)}</span> •{' '}
                    {alt.resolved_at ? (
                      <span className="text-emerald-400">
                        Resolved {new Date(alt.resolved_at).toLocaleTimeString('en-GB')}
                      </span>
                    ) : (
                      <span className="text-rose-400">Triggered {new Date(alt.triggered_at).toLocaleTimeString('en-GB')}</span>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </main>
      </div>
    </>
  );
}

