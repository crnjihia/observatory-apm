// path: web/components/TraceModal.tsx
import React, { useEffect, useState } from 'react';
import {
  LatencyRow,
  fetchTracesForBucket,
  fetchTraceSpans,
  fetchTraceLogs,
  SpanRow,
  LogRow,
} from '../utils/api';
import { TraceWaterfall } from './TraceWaterfall';
import { LogViewer } from './LogViewer';

interface TraceModalProps {
  bucket: LatencyRow;
  onClose: () => void;
}

export const TraceModal: React.FC<TraceModalProps> = ({ bucket, onClose }) => {
  const [traceIds, setTraceIds] = useState<string[]>([]);
  const [selectedTrace, setSelectedTrace] = useState<string | null>(null);
  const [spans, setSpans] = useState<SpanRow[]>([]);
  const [logs, setLogs] = useState<LogRow[]>([]);
  const [selectedSpan, setSelectedSpan] = useState<SpanRow | null>(null);

  const [activeTab, setActiveTab] = useState<'waterfall' | 'logs' | 'raw'>('waterfall');
  const [loadingTraces, setLoadingTraces] = useState<boolean>(false);
  const [loadingDetails, setLoadingDetails] = useState<boolean>(false);
  const [error, setError] = useState<string>('');

  // Close on Escape key press
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [onClose]);

  // Load trace IDs for the selected bucket on mount
  useEffect(() => {
    setLoadingTraces(true);
    fetchTracesForBucket({
      service: bucket.service,
      endpoint: bucket.endpoint,
      bucket: bucket.bucket,
      limit: 25,
    })
      .then((ids) => {
        setTraceIds(ids);
        if (ids.length > 0) {
          setSelectedTrace(ids[0]);
        }
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoadingTraces(false));
  }, [bucket]);

  // Load spans and logs whenever selectedTrace changes
  useEffect(() => {
    if (!selectedTrace) return;
    setLoadingDetails(true);
    setSelectedSpan(null);
    setError('');

    Promise.all([fetchTraceSpans(selectedTrace), fetchTraceLogs(selectedTrace)])
      .then(([s, l]) => {
        setSpans(s);
        setLogs(l);
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoadingDetails(false));
  }, [selectedTrace]);

  const hasErrors = spans.some((s) => s.status === 'STATUS_CODE_ERROR');
  const rootSpan = spans.find((s) => !s.parent_id) || spans[0];
  const totalDurationMs = spans.reduce((max, s) => Math.max(max, s.duration_ns / 1e6), 0);

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-3 sm:p-6 bg-black/80 backdrop-blur-sm animate-fade-in"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="relative w-full max-w-6xl max-h-[92vh] bg-slate-900 border border-slate-800 rounded-2xl shadow-2xl flex flex-col overflow-hidden text-slate-100">
        {/* Modal Header */}
        <div className="px-6 py-4 bg-slate-900/90 border-b border-slate-800 flex items-center justify-between shrink-0">
          <div className="flex items-center space-x-3">
            <div className="p-2 rounded-lg bg-indigo-500/10 border border-indigo-500/30 text-indigo-400">
              <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"
                />
              </svg>
            </div>
            <div>
              <div className="flex items-center space-x-2">
                <h2 className="text-base font-bold text-white tracking-tight">
                  Trace Drill-Down: {bucket.service}
                </h2>
                <span className="px-2 py-0.5 rounded text-[11px] font-mono bg-slate-800 text-slate-300 border border-slate-700">
                  {bucket.endpoint || '/v1/callbacks/mpesa'}
                </span>
                {hasErrors && (
                  <span className="px-2 py-0.5 rounded text-[11px] font-semibold bg-rose-500/20 text-rose-300 border border-rose-500/30 animate-pulse">
                    Error Detected
                  </span>
                )}
              </div>
              <div className="text-xs text-slate-400 mt-0.5">
                Bucket: <span className="font-mono text-slate-300">{new Date(bucket.bucket).toLocaleString('en-GB')} EAT</span> • P95: <span className="font-mono text-amber-400">{bucket.p95}ms</span>
              </div>
            </div>
          </div>

          <button
            onClick={onClose}
            className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 border border-transparent hover:border-slate-700 transition-colors"
            title="Close (ESC)"
          >
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {/* Content Body: Split layout with Left Trace Selector + Right Drill-down Tabs */}
        <div className="flex-1 grid grid-cols-1 md:grid-cols-12 overflow-hidden divide-y md:divide-y-0 md:divide-x divide-slate-800">
          {/* Left Column: Trace Selector List (3 cols) */}
          <div className="md:col-span-4 bg-slate-950/60 p-4 flex flex-col overflow-y-auto max-h-[300px] md:max-h-full">
            <div className="flex items-center justify-between mb-3 text-xs">
              <span className="font-semibold text-slate-300 uppercase tracking-wide text-[10px]">
                Correlated Traces ({traceIds.length})
              </span>
              {loadingTraces && <span className="text-indigo-400 animate-spin">⟳</span>}
            </div>

            {error && (
              <div className="p-2 mb-3 bg-rose-950/40 border border-rose-800 rounded text-rose-300 text-xs">
                {error}
              </div>
            )}

            <div className="space-y-1.5 flex-1 overflow-y-auto">
              {traceIds.map((id, idx) => {
                const isSelected = id === selectedTrace;
                return (
                  <button
                    key={id}
                    onClick={() => setSelectedTrace(id)}
                    className={`w-full text-left p-2.5 rounded-xl border transition-all text-xs ${
                      isSelected
                        ? 'bg-indigo-950/60 border-indigo-500/80 text-white shadow-sm'
                        : 'bg-slate-900/80 border-slate-800/80 text-slate-300 hover:bg-slate-800/60'
                    }`}
                  >
                    <div className="flex items-center justify-between mb-1">
                      <span className="font-mono font-semibold text-slate-200">
                        {id.slice(0, 14)}…{id.slice(-6)}
                      </span>
                      {idx === 0 && hasErrors ? (
                        <span className="w-2 h-2 rounded-full bg-rose-500 animate-ping" />
                      ) : (
                        <span className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
                      )}
                    </div>
                    <div className="flex items-center justify-between text-[11px] text-slate-400">
                      <span>{bucket.service}</span>
                      <span className="font-mono text-amber-400">
                        {idx === 0 && totalDurationMs > 0 ? `${totalDurationMs.toFixed(0)}ms` : `${bucket.p95}ms`}
                      </span>
                    </div>
                  </button>
                );
              })}
            </div>
          </div>

          {/* Right Column: Active Trace Drill-Down (8 cols) */}
          <div className="md:col-span-8 flex flex-col overflow-hidden bg-slate-900">
            {/* View Tabs & Trace Info Bar */}
            <div className="px-6 py-2.5 bg-slate-900/95 border-b border-slate-800 flex flex-wrap items-center justify-between gap-3 shrink-0">
              {/* Tab Switcher */}
              <div className="flex items-center space-x-1 bg-slate-950 p-1 rounded-lg border border-slate-800">
                <button
                  onClick={() => setActiveTab('waterfall')}
                  className={`px-3 py-1 text-xs font-semibold rounded-md transition-colors ${
                    activeTab === 'waterfall'
                      ? 'bg-indigo-600 text-white shadow-sm'
                      : 'text-slate-400 hover:text-slate-200'
                  }`}
                >
                  Waterfall Timeline
                </button>
                <button
                  onClick={() => setActiveTab('logs')}
                  className={`px-3 py-1 text-xs font-semibold rounded-md transition-colors ${
                    activeTab === 'logs'
                      ? 'bg-indigo-600 text-white shadow-sm'
                      : 'text-slate-400 hover:text-slate-200'
                  }`}
                >
                  Correlated Logs ({logs.length})
                </button>
                <button
                  onClick={() => setActiveTab('raw')}
                  className={`px-3 py-1 text-xs font-semibold rounded-md transition-colors ${
                    activeTab === 'raw'
                      ? 'bg-indigo-600 text-white shadow-sm'
                      : 'text-slate-400 hover:text-slate-200'
                  }`}
                >
                  Raw JSON
                </button>
              </div>

              {selectedTrace && (
                <div className="text-[11px] font-mono text-slate-400">
                  Trace ID: <span className="text-indigo-300 font-semibold select-all">{selectedTrace}</span>
                </div>
              )}
            </div>

            {/* Drill-down View Area */}
            <div className="flex-1 p-6 overflow-y-auto">
              {loadingDetails ? (
                <div className="flex flex-col items-center justify-center py-16 text-slate-400">
                  <div className="w-8 h-8 border-2 border-indigo-500 border-t-transparent rounded-full animate-spin mb-3" />
                  <span className="text-xs">Loading trace spans & logs…</span>
                </div>
              ) : selectedTrace ? (
                <>
                  {activeTab === 'waterfall' && (
                    <TraceWaterfall
                      spans={spans}
                      selectedSpanId={selectedSpan?.span_id}
                      onSelectSpan={setSelectedSpan}
                    />
                  )}

                  {activeTab === 'logs' && (
                    <LogViewer
                      logs={logs}
                      traceId={selectedTrace}
                      selectedSpanId={selectedSpan?.span_id}
                    />
                  )}

                  {activeTab === 'raw' && (
                    <div className="rounded-xl bg-slate-950 border border-slate-800 p-4 font-mono text-xs overflow-x-auto text-slate-300">
                      <pre>{JSON.stringify({ trace_id: selectedTrace, spans, logs }, null, 2)}</pre>
                    </div>
                  )}
                </>
              ) : (
                <div className="py-16 text-center text-slate-500 italic text-sm">
                  Select a trace from the left panel to inspect the span waterfall.
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
