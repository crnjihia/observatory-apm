// path: C:/Users/crnji/Videos/Projects/Web Development/New/observatory-apm/web/pages/index.tsx
import React, { useEffect, useState } from 'react';
import { fetchServices, fetchLatency, LatencyRow } from '../utils/api';
import { Heatmap } from '../components/Heatmap';
import { TraceModal } from '../components/TraceModal';


export default function Dashboard() {
  const [services, setServices] = useState<string[]>([]);
  const [selectedService, setSelectedService] = useState<string>('');
  const [start, setStart] = useState<string>('');
  const [end, setEnd] = useState<string>('');
  const [data, setData] = useState<LatencyRow[]>([]);
  const [selectedBucket, setSelectedBucket] = useState<LatencyRow | null>(null);

  const [error, setError] = useState<string>('');

  // Load services on mount.
  useEffect(() => {
    fetchServices()
      .then(setServices)
      .catch(err => setError(err.message));
  }, []);

  // Helper to get ISO timestamps for the last 15 minutes.
  const setDefaultTimeRange = () => {
    const now = new Date();
    const fifteen = new Date(now.getTime() - 15 * 60 * 1000);
    setStart(fifteen.toISOString());
    setEnd(now.toISOString());
  };

  // Load data when service or time range changes.
  useEffect(() => {
    if (!selectedService || !start || !end) return;
    setLoading(true);
    setError('');
    fetchLatency({ service: selectedService, start, end, limit: 200 })
      .then(setData)
      .catch(err => setError(err.message))
      .finally(() => setLoading(false));
  }, [selectedService, start, end]);

  // Initialise default time range on first service selection.
  useEffect(() => {
    if (selectedService && !start && !end) {
      setDefaultTimeRange();
    }
  }, [selectedService]);

  return (
    <main className="max-w-5xl mx-auto p-4">
      <h1 className="text-2xl font-bold mb-4">Observatory APM Dashboard</h1>
      {error && <p className="text-red-600">Error: {error}</p>}
      <section className="flex items-center space-x-4 mb-4">
        <label className="block">
          <span className="text-gray-700">Service</span>
          <select
            className="mt-1 block w-full"
            value={selectedService}
            onChange={e => setSelectedService(e.target.value)}
          >
            <option value="">Select a service</option>
            {services.map(s => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
        </label>
        <label className="block">
          <span className="text-gray-700">Start</span>
          <input
            type="datetime-local"
            className="mt-1 block w-full"
            value={start ? start.slice(0, -1) : ''}
            onChange={e => setStart(new Date(e.target.value).toISOString())}
          />
        </label>
        <label className="block">
          <span className="text-gray-700">End</span>
          <input
            type="datetime-local"
            className="mt-1 block w-full"
            value={end ? end.slice(0, -1) : ''}
            onChange={e => setEnd(new Date(e.target.value).toISOString())}
          />
        </label>
      </section>
      {loading && <p>Loading data…</p>}
      {!loading && data.length > 0 && <Heatmap data={data} onBarClick={setSelectedBucket} />}
      {selectedBucket && (
        <TraceModal bucket={selectedBucket} onClose={() => setSelectedBucket(null)} />
      )}
    </main>
  );
}
