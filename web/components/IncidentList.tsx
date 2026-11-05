// path: C:/Users/crnji/Videos/Projects/Web Development/New/observatory-apm/web/components/IncidentList.tsx
import React, { useEffect, useState } from 'react';
import { Incident, fetchIncidents, ackIncident, resolveIncident, createIncident } from '../utils/api';

export const IncidentList: React.FC = () => {
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>('');

  const load = () => {
    setLoading(true);
    fetchIncidents()
      .then(setIncidents)
      .catch(err => setError(err.message))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    load();
    // Poll every 30 seconds to keep UI fresh.
    const interval = setInterval(load, 30_000);
    return () => clearInterval(interval);
  }, []);

  const handleAck = (id: string) => {
    ackIncident(id)
      .then(() => load())
      .catch(err => setError(err.message));
  };
  const handleResolve = (id: string) => {
    resolveIncident(id)
      .then(() => load())
      .catch(err => setError(err.message));
  };

  const handleInject = () => {
    // Simulate an M‑Pay latency incident.
    createIncident({
      service: 'demo',
      endpoint: '/pay',
      description: 'Synthetic M‑Pay callback latency spike',
    })
      .then(() => load())
      .catch(err => setError(err.message));
  };

  return (
    <div className="p-4 bg-white rounded shadow">
      <h3 className="text-lg font-medium mb-2">Incidents</h3>
      <button onClick={handleInject} className="mb-2 px-3 py-1 bg-blue-600 text-white rounded">
        Inject Incident
      </button>
      {error && <p className="text-red-600">Error: {error}</p>}
      {loading ? (
        <p>Loading incidents…</p>
      ) : (
        <table className="min-w-full table-auto border">
          <thead className="bg-gray-100">
            <tr>
              <th className="border px-2 py-1">ID</th>
              <th className="border px-2 py-1">Service</th>
              <th className="border px-2 py-1">Endpoint</th>
              <th className="border px-2 py-1">State</th>
              <th className="border px-2 py-1">Created</th>
              <th className="border px-2 py-1">Actions</th>
            </tr>
          </thead>
          <tbody>
            {incidents.map(inc => (
              <tr key={inc.id}>
                <td className="border px-2 py-1 text-sm" title={inc.id}>{inc.id.slice(0, 8)}…</td>
                <td className="border px-2 py-1">{inc.service}</td>
                <td className="border px-2 py-1">{inc.endpoint}</td>
                <td className="border px-2 py-1 capitalize">{inc.state}</td>
                <td className="border px-2 py-1 text-sm">{new Date(inc.created_at).toLocaleString()}</td>
                <td className="border px-2 py-1 space-x-2">
                  {inc.state === 'triggered' && (
                    <button onClick={() => handleAck(inc.id)} className="px-2 py-1 bg-yellow-500 text-white rounded">
                      Ack
                    </button>
                  )}
                  {inc.state === 'acknowledged' && (
                    <button onClick={() => handleResolve(inc.id)} className="px-2 py-1 bg-green-600 text-white rounded">
                      Resolve
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
};
