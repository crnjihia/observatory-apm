// path: web/components/LogViewer.tsx
import React, { useState, useMemo } from 'react';
import { LogRow } from '../utils/api';

interface LogViewerProps {
  logs: LogRow[];
  traceId?: string;
  selectedSpanId?: string | null;
}

export const LogViewer: React.FC<LogViewerProps> = ({ logs, traceId, selectedSpanId }) => {
  const [filterText, setFilterText] = useState<string>('');
  const [severityFilter, setSeverityFilter] = useState<string>('ALL');
  const [expandedLogIdx, setExpandedLogIdx] = useState<number | null>(null);

  const filteredLogs = useMemo(() => {
    return logs.filter((log) => {
      // Filter by span if selected
      if (selectedSpanId && log.span_id && log.span_id !== selectedSpanId) {
        return false;
      }

      // Filter by severity
      if (severityFilter !== 'ALL') {
        const sev = (log.severity || 'INFO').toUpperCase();
        if (sev !== severityFilter) return false;
      }

      // Filter by search text
      if (filterText.trim()) {
        const q = filterText.toLowerCase();
        const matchesBody = log.body.toLowerCase().includes(q);
        const matchesService = log.service.toLowerCase().includes(q);
        const matchesAttrs = log.attrs ? JSON.stringify(log.attrs).toLowerCase().includes(q) : false;
        return matchesBody || matchesService || matchesAttrs;
      }

      return true;
    });
  }, [logs, selectedSpanId, severityFilter, filterText]);

  const severities = ['ALL', 'INFO', 'WARN', 'ERROR', 'DEBUG'];

  const getSeverityBadge = (severity?: string) => {
    const s = (severity || 'INFO').toUpperCase();
    switch (s) {
      case 'ERROR':
        return 'bg-rose-500/20 text-rose-300 border-rose-500/40';
      case 'WARN':
      case 'WARNING':
        return 'bg-amber-500/20 text-amber-300 border-amber-500/40';
      case 'DEBUG':
        return 'bg-slate-700/50 text-slate-300 border-slate-600';
      case 'INFO':
      default:
        return 'bg-sky-500/20 text-sky-300 border-sky-500/30';
    }
  };

  return (
    <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden shadow-lg">
      {/* Header & Filter Bar */}
      <div className="px-4 py-3 bg-slate-900/95 border-b border-slate-800 flex flex-wrap items-center justify-between gap-3 text-xs">
        <div className="flex items-center space-x-3">
          <span className="font-semibold text-white">Correlated Application Logs</span>
          <span className="px-2 py-0.5 rounded bg-slate-800 text-slate-300 font-mono text-[11px]">
            {filteredLogs.length} of {logs.length} logs
          </span>
          {selectedSpanId && (
            <span className="px-2 py-0.5 rounded bg-indigo-500/20 text-indigo-300 font-mono text-[11px]">
              Filtered by span: {selectedSpanId.slice(0, 8)}…
            </span>
          )}
        </div>

        {/* Severity filter pills & Search input */}
        <div className="flex items-center space-x-3">
          <div className="flex items-center bg-slate-800/80 p-0.5 rounded-lg border border-slate-700/80">
            {severities.map((sev) => (
              <button
                key={sev}
                onClick={() => setSeverityFilter(sev)}
                className={`px-2 py-0.5 text-[11px] font-medium rounded-md transition-all ${
                  severityFilter === sev
                    ? 'bg-indigo-600 text-white'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                {sev}
              </button>
            ))}
          </div>

          <div className="relative">
            <input
              type="text"
              placeholder="Search logs..."
              value={filterText}
              onChange={(e) => setFilterText(e.target.value)}
              className="bg-slate-800 border border-slate-700 text-slate-200 text-xs rounded-lg px-2.5 py-1 pl-7 focus:outline-none focus:ring-1 focus:ring-indigo-500 w-36 sm:w-48"
            />
            <svg
              className="w-3.5 h-3.5 text-slate-400 absolute left-2 top-2 pointer-events-none"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
          </div>
        </div>
      </div>

      {/* Log Entries Stream */}
      <div className="divide-y divide-slate-800/60 max-h-[380px] overflow-y-auto font-mono text-xs">
        {filteredLogs.length > 0 ? (
          filteredLogs.map((l, idx) => {
            const isExpanded = expandedLogIdx === idx;
            const hasAttrs = l.attrs && Object.keys(l.attrs).length > 0;

            return (
              <div
                key={idx}
                onClick={() => setExpandedLogIdx(isExpanded ? null : idx)}
                className={`p-2.5 transition-colors cursor-pointer ${
                  isExpanded ? 'bg-slate-800/50' : 'hover:bg-slate-800/20'
                }`}
              >
                <div className="flex items-start space-x-2.5">
                  {/* Timestamp */}
                  <span className="text-[11px] text-slate-400 shrink-0">
                    {new Date(l.ts).toLocaleTimeString('en-GB', {
                      hour: '2-digit',
                      minute: '2-digit',
                      second: '2-digit',
                      fractionalSecondDigits: 3,
                    })}
                  </span>

                  {/* Severity badge */}
                  <span
                    className={`px-1.5 py-0.2 text-[10px] rounded border font-semibold shrink-0 uppercase ${getSeverityBadge(
                      l.severity
                    )}`}
                  >
                    {l.severity || 'INFO'}
                  </span>

                  {/* Service tag */}
                  <span className="text-slate-400 text-[11px] shrink-0 font-semibold">
                    [{l.service}]
                  </span>

                  {/* Log body message */}
                  <span className="text-slate-200 flex-1 break-words">{l.body}</span>

                  {/* Span correlation badge */}
                  {l.span_id && (
                    <span className="text-[10px] text-indigo-400 shrink-0 bg-indigo-950/40 px-1 rounded border border-indigo-500/20">
                      span:{l.span_id.slice(0, 6)}
                    </span>
                  )}
                </div>

                {/* Collapsible Attributes Table */}
                {isExpanded && hasAttrs && (
                  <div className="mt-2 pl-6 pt-2 border-t border-slate-800/80">
                    <span className="text-[10px] text-slate-400 uppercase tracking-wide block mb-1">
                      Structured Log Attributes:
                    </span>
                    <pre className="p-2 rounded bg-slate-950 border border-slate-800 text-[11px] text-slate-300 overflow-x-auto">
                      {JSON.stringify(l.attrs, null, 2)}
                    </pre>
                  </div>
                )}
              </div>
            );
          })
        ) : (
          <div className="py-8 text-center text-slate-500 italic">
            No correlated log lines match the active filter criteria.
          </div>
        )}
      </div>
    </div>
  );
};

