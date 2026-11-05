// path: C:/Users/crnji/Videos/Projects/Web Development/New/observatory-apm/web/components/TraceModal.tsx
import React, { useEffect, useState } from 'react';
import { LatencyRow, fetchTracesForBucket, fetchTraceSpans, fetchTraceLogs, SpanRow, LogRow } from '../utils/api';

interface TraceModalProps {
  bucket: LatencyRow; // the heatmap bar that was clicked
  onClose: () => void;
}

export const TraceModal: React.FC<TraceModalProps> = ({ bucket, onClose }) => {
  const [traceIds, setTraceIds] = useState<string[]>([]);
  const [selectedTrace, setSelectedTrace] = useState<string | null>(null);
  const [spans, setSpans] = useState<SpanRow[]>([]);
  const [logs, setLogs] = useState<LogRow[]>([]);
  const [error, setError] = useState<string>('');
  const [loadingTraces, setLoadingTraces] = useState<boolean>(false);
  const [loadingDetails, setLoadingDetails] = useState<boolean>(false);

  // Load trace IDs for the bucket when the modal opens.
  useEffect(() => {
    setLoadingTraces(true);
    fetchTracesForBucket({
      service: bucket.service,
      endpoint: bucket.endpoint,
      bucket: bucket.bucket,
      limit: 20,
    })
      .then(setTraceIds)
      .catch(err => setError(err.message))
      .finally(() => setLoadingTraces(false));
  }, [bucket]);

  // Load spans & logs when a trace is selected.
  useEffect(() => {
    if (!selectedTrace) return;
    setLoadingDetails(true);
    Promise.all([
      fetchTraceSpans(selectedTrace),
      fetchTraceLogs(selectedTrace),
    ])
      .then(([s, l]) => {
        setSpans(s);
        setLogs(l);
      })
      .catch(err => setError(err.message))
      .finally(() => setLoadingDetails(false));
  }, [selectedTrace]);

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-lg w-11/12 max-w-4xl p-4 overflow-y-auto max-h-screen">
        <div className="flex justify-between items-center mb-4">
          <h2 className="text-xl font-semibold">Drill‑down: {bucket.service} – {bucket.endpoint} – {bucket.bucket}</h2>
          <button onClick={onClose} className="text-gray-600 hover:text-gray-800">✖</button>
        </div>
        {error && <p className="text-red-600">Error: {error}</p>}
        {loadingTraces ? (
          <p>Loading trace IDs…</p>
        ) : (
          <div>
            <h3 className="font-medium mb-2">Traces ({traceIds.length})</h3>
            <ul className="list-disc list-inside mb-4">
              {traceIds.map(id => (
                <li key={id} className="cursor-pointer hover:underline" onClick={() => setSelectedTrace(id)}>
                  {id}
                </li>
              ))}
            </ul>
          </div>
        )}
        {selectedTrace && (
          <div>
            <h3 className="font-medium mb-2">Trace ID: {selectedTrace}</h3>
            {loadingDetails ? (
              <p>Loading trace details…</p>
            ) : (
              <>
                <h4 className="mt-3 font-semibold">Spans ({spans.length})</h4>
                <table className="min-w-full table-auto border">
                  <thead className="bg-gray-100">
                    <tr>
                      <th className="border px-2 py-1">Span ID</th>
                      <th className="border px-2 py-1">Parent ID</th>
                      <th className="border px-2 py-1">Name</th>
                      <th className="border px-2 py-1">Start</th>
                      <th className="border px-2 py-1">Duration ms</th>
                      <th className="border px-2 py-1">Status</th>
                    </tr>
                  </thead>
                  <tbody>
                    {spans.map(s => (
                      <tr key={s.span_id}>
                        <td className="border px-2 py-1 text-sm" title={s.span_id}>{s.span_id.slice(0, 8)}…</td>
                        <td className="border px-2 py-1 text-sm" title={s.parent_id ?? ''}>{s.parent_id ? s.parent_id.slice(0, 8) + '…' : ''}</td>
                        <td className="border px-2 py-1">{s.name}</td>
                        <td className="border px-2 py-1 text-sm">{new Date(s.start_ts).toLocaleTimeString()}</td>
                        <td className="border px-2 py-1 text-sm">{(s.duration_ns / 1e6).toFixed(2)}</td>
                        <td className="border px-2 py-1">{s.status ?? ''}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                <h4 className="mt-3 font-semibold">Logs ({logs.length})</h4>
                <table className="min-w-full table-auto border">
                  <thead className="bg-gray-100">
                    <tr>
                      <th className="border px-2 py-1">Timestamp</th>
                      <th className="border px-2 py-1">Severity</th>
                      <th className="border px-2 py-1">Body</th>
                    </tr>
                  </thead>
                  <tbody>
                    {logs.map(l => (
                      <tr key={l.ts + l.body}>
                        <td className="border px-2 py-1 text-sm">{new Date(l.ts).toLocaleTimeString()}</td>
                        <td className="border px-2 py-1 text-sm">{l.severity ?? ''}</td>
                        <td className="border px-2 py-1 break-all">{l.body}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </>
            )}
          </div>
        )}
      </div>
    </div>
  );
};
