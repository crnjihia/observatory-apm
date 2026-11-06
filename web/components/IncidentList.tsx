// path: web/components/IncidentList.tsx
import React, { useEffect, useState, useMemo } from 'react';
import { Incident, fetchIncidents, ackIncident, resolveIncident } from '../utils/api';

export const IncidentList: React.FC = () => {
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>('');
  const [filter, setFilter] = useState<'ALL' | 'TRIGGERED' | 'ACKNOWLEDGED' | 'RESOLVED'>('ALL');

  const load = () => {
    setLoading(true);
    fetchIncidents()
      .then(setIncidents)
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    load();
    const interval = setInterval(load, 15_000);
    return () => clearInterval(interval);
  }, []);

  const handleAck = (id: string) => {
    ackIncident(id)
      .then(() => load())
      .catch((err) => setError(err.message));
  };

  const handleResolve = (id: string) => {
    resolveIncident(id)
      .then(() => load())
      .catch((err) => setError(err.message));
  };

  const filteredIncidents = useMemo(() => {
    if (filter === 'ALL') return incidents;
    return incidents.filter((i) => i.state.toUpperCase() === filter);
  }, [incidents, filter]);

  const getStateBadge = (state: string) => {
    const s = state.toUpperCase();
    switch (s) {
      case 'TRIGGERED':
        return 'bg-rose-500/20 text-rose-300 border-rose-500/40 animate-pulse';
      case 'ACKNOWLEDGED':
        return 'bg-amber-500/20 text-amber-300 border-amber-500/40';
      case 'RESOLVED':
        return 'bg-emerald-500/20 text-emerald-300 border-emerald-500/30';
      default:
        return 'bg-slate-800 text-slate-300 border-slate-700';
    }
  };

  return (
    <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-sm">
      {/* Header & Filter Controls */}
      <div className="flex flex-wrap items-center justify-between gap-3 mb-4">
        <div className="flex items-center space-x-3">
          <h2 className="text-sm font-semibold text-white tracking-tight flex items-center">
            <span className="w-2 h-2 rounded-full bg-rose-500 mr-2"></span>
            Incident Log & Escalations ({filteredIncidents.length})
          </h2>
          {loading && <span className="text-indigo-400 text-xs animate-spin">⟳</span>}
        </div>

        {/* Filter Pills */}
        <div className="flex items-center space-x-1 bg-slate-950 p-1 rounded-lg border border-slate-800 text-xs">
          {(['ALL', 'TRIGGERED', 'ACKNOWLEDGED', 'RESOLVED'] as const).map((tab) => (
            <button
              key={tab}
              onClick={() => setFilter(tab)}
              className={`px-2.5 py-1 rounded-md font-semibold transition-colors ${
                filter === tab
                  ? 'bg-indigo-600 text-white'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              {tab}
            </button>
          ))}
        </div>
      </div>

      {error && (
        <div className="mb-4 p-3 bg-rose-950/40 border border-rose-800 text-rose-300 text-xs rounded-lg">
          {error}
        </div>
      )}

      {/* Incidents Table */}
      <div className="overflow-x-auto">
        <table className="w-full text-left text-xs text-slate-300">
          <thead className="bg-slate-800/60 text-slate-400 uppercase text-[10px] font-semibold border-b border-slate-800">
            <tr>
              <th className="py-2.5 px-3">Incident ID</th>
              <th className="py-2.5 px-3">Service & Endpoint</th>
              <th className="py-2.5 px-3">Incident Description</th>
              <th className="py-2.5 px-3">State</th>
              <th className="py-2.5 px-3">Timestamp (EAT)</th>
              <th className="py-2.5 px-3 text-right">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-800/60">
            {filteredIncidents.length > 0 ? (
              filteredIncidents.map((inc) => (
                <tr key={inc.id} className="hover:bg-slate-800/40 transition-colors">
                  <td className="py-3 px-3 font-mono font-semibold text-slate-200" title={inc.id}>
                    {inc.id.slice(0, 12)}…
                  </td>
                  <td className="py-3 px-3">
                    <div className="font-semibold text-white">{inc.service}</div>
                    <div className="text-[11px] text-slate-400 font-mono">{inc.endpoint}</div>
                  </td>
                  <td className="py-3 px-3 max-w-sm">
                    <span className="text-slate-200 line-clamp-2">{inc.description}</span>
                  </td>
                  <td className="py-3 px-3">
                    <span
                      className={`px-2 py-0.5 rounded text-[10px] font-bold border uppercase ${getStateBadge(
                        inc.state
                      )}`}
                    >
                      {inc.state}
                    </span>
                  </td>
                  <td className="py-3 px-3 font-mono text-[11px] text-slate-400">
                    {new Date(inc.created_at).toLocaleTimeString('en-GB')}
                  </td>
                  <td className="py-3 px-3 text-right">
                    <div className="flex items-center justify-end space-x-2">
                      {inc.state.toLowerCase() === 'triggered' && (
                        <button
                          onClick={() => handleAck(inc.id)}
                          className="px-2.5 py-1 text-xs font-semibold rounded-lg bg-amber-500/20 text-amber-300 border border-amber-500/40 hover:bg-amber-500/30 transition-colors"
                        >
                          Ack
                        </button>
                      )}
                      {inc.state.toLowerCase() === 'acknowledged' && (
                        <button
                          onClick={() => handleResolve(inc.id)}
                          className="px-2.5 py-1 text-xs font-semibold rounded-lg bg-emerald-500/20 text-emerald-300 border border-emerald-500/40 hover:bg-emerald-500/30 transition-colors"
                        >
                          Resolve
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ))
            ) : (
              <tr>
                <td colSpan={6} className="py-8 text-center text-slate-500 italic">
                  No incidents matching filter &lsquo;{filter}&rsquo;. System nominal.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
};
