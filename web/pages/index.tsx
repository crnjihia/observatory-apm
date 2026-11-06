// path: web/pages/index.tsx
import React, { useEffect, useState, useCallback } from 'react';
import Head from 'next/head';
import {
  fetchServices,
  fetchLatency,
  fetchHeatmapBuckets,
  fetchErrors,
  fetchCurrentOnCall,
  LatencyRow,
  HeatmapResponse,
  HeatmapCell,
  ErrorSummaryResponse,
} from '../utils/api';
import { Navbar } from '../components/Navbar';
import { ServiceStats } from '../components/ServiceStats';
import { Heatmap, HeatmapSelection } from '../components/Heatmap';
import { LatencyTimeseries } from '../components/LatencyTimeseries';
import { ErrorRatePanel } from '../components/ErrorRatePanel';
import { TraceModal } from '../components/TraceModal';

export default function Dashboard() {
  const [services, setServices] = useState<string[]>([]);
  const [selectedService, setSelectedService] = useState<string>('');
  const [timeRange, setTimeRange] = useState<string>('15m');
  const [start, setStart] = useState<string>('');
  const [end, setEnd] = useState<string>('');
  const [autoRefresh, setAutoRefresh] = useState<number>(10); // default 10s

  const [latencyData, setLatencyData] = useState<LatencyRow[]>([]);
  const [heatmapData, setHeatmapData] = useState<HeatmapResponse | null>(null);
  const [errorSummary, setErrorSummary] = useState<ErrorSummaryResponse | null>(null);
  const [onCallUser, setOnCallUser] = useState<string>('+254712345678 (Christopher Njihia)');

  const [selectedBucket, setSelectedBucket] = useState<LatencyRow | null>(null);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>('');

  // Calculate start & end ISO strings based on timeRange label
  const computeTimeRange = useCallback((range: string) => {
    const now = new Date();
    let minutes = 15;
    if (range === '1h') minutes = 60;
    else if (range === '6h') minutes = 360;
    else if (range === '24h') minutes = 1440;

    const fromDate = new Date(now.getTime() - minutes * 60 * 1000);
    setStart(fromDate.toISOString());
    setEnd(now.toISOString());
  }, []);

  // Initialize services and default time window
  useEffect(() => {
    fetchServices()
      .then((res) => {
        setServices(res);
        if (res.length > 0 && !selectedService) {
          setSelectedService(res[0]);
        }
      })
      .catch((err) => setError(err.message));

    fetchCurrentOnCall()
      .then(setOnCallUser)
      .catch(() => {});

    computeTimeRange('15m');
  }, [computeTimeRange, selectedService]);

  // Load all telemetry dashboard data
  const loadData = useCallback(() => {
    if (!selectedService || !start || !end) return;
    setLoading(true);
    setError('');

    Promise.all([
      fetchLatency({ service: selectedService, start, end, limit: 100 }),
      fetchHeatmapBuckets({ service: selectedService, start, end, bins: 20 }),
      fetchErrors({ service: selectedService, limit: 50 }),
    ])
      .then(([lat, hm, errs]) => {
        setLatencyData(lat);
        setHeatmapData(hm);
        setErrorSummary(errs);
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [selectedService, start, end]);

  // Trigger data load on parameters change
  useEffect(() => {
    loadData();
  }, [loadData]);

  // Auto-refresh interval timer
  useEffect(() => {
    if (autoRefresh <= 0) return;
    const interval = setInterval(() => {
      // Advance time range window to now
      computeTimeRange(timeRange);
    }, autoRefresh * 1000);
    return () => clearInterval(interval);
  }, [autoRefresh, timeRange, computeTimeRange]);

  const handleTimeRangeChange = (range: string) => {
    setTimeRange(range);
    computeTimeRange(range);
  };

  const handleCellSelect = (cell: HeatmapCell) => {
    const row: LatencyRow = {
      bucket: cell.time_bucket,
      service: selectedService,
      endpoint: heatmapData?.endpoint || '/v1/callbacks/mpesa',
      p50: cell.latency_min_ms,
      p95: cell.latency_max_ms,
      p99: cell.latency_max_ms,
      span_count: cell.count,
      error_rate: cell.count > 0 ? cell.error_count / cell.count : 0,
    };
    setSelectedBucket(row);
  };

  const handleRegionSelect = (sel: HeatmapSelection) => {
    const row: LatencyRow = {
      bucket: sel.startTime,
      service: selectedService,
      endpoint: heatmapData?.endpoint || '/v1/callbacks/mpesa',
      p50: sel.minLatencyMs,
      p95: sel.maxLatencyMs,
      p99: sel.maxLatencyMs,
      span_count: 50,
      error_rate: 0.1,
    };
    setSelectedBucket(row);
  };

  const handleSelectTrace = (traceId: string) => {
    const row: LatencyRow = {
      bucket: new Date().toISOString(),
      service: selectedService,
      endpoint: '/v1/callbacks/mpesa',
      p50: 100,
      p95: 1500,
      p99: 3000,
      span_count: 1,
      error_rate: 1.0,
    };
    setSelectedBucket(row);
  };

  // Peak P99 latency warning
  const peakP99 = latencyData.length > 0 ? Math.max(...latencyData.map((d) => d.p99 || 0)) : 0;
  const hasSLAViolation = peakP99 >= 1500;

  return (
    <>
      <Head>
        <title>Observatory APM – Real-Time Telemetry & Heatmap</title>
        <meta
          name="description"
          content="Unified metrics, distributed traces, and log analytics for Kenyan high-volume infrastructures."
        />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
      </Head>

      <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col font-sans selection:bg-indigo-500 selection:text-white">
        {/* Navigation Header */}
        <Navbar
          services={services}
          selectedService={selectedService}
          onSelectService={setSelectedService}
          timeRange={timeRange}
          onSelectTimeRange={handleTimeRangeChange}
          autoRefresh={autoRefresh}
          onSelectAutoRefresh={setAutoRefresh}
          onManualRefresh={loadData}
          loading={loading}
        />

        {/* Main Dashboard Content Area */}
        <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-6">
          {/* Critical SLA Warning Banner */}
          {hasSLAViolation && (
            <div className="mb-6 p-3.5 bg-rose-950/40 border border-rose-500/40 rounded-xl flex items-center justify-between text-rose-200 text-xs sm:text-sm animate-pulse">
              <div className="flex items-center space-x-3">
                <span className="flex h-3 w-3 relative">
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-rose-400 opacity-75"></span>
                  <span className="relative inline-flex rounded-full h-3 w-3 bg-rose-500"></span>
                </span>
                <div>
                  <span className="font-bold text-white mr-1.5">Safaricom Daraja SLA Breach Alert:</span>
                  P99 callback latency peaked at <strong className="font-mono text-white">{peakP99} ms</strong> (exceeding 1500ms SLA limit).
                </div>
              </div>
              <button
                onClick={() => handleSelectTrace('54eba8423f6d4350ba3291c0bfd51e11')}
                className="px-2.5 py-1 rounded bg-rose-500 text-white font-semibold text-xs hover:bg-rose-600 transition-colors shadow-sm ml-3 shrink-0"
              >
                Inspect Spans
              </button>
            </div>
          )}

          {error && (
            <div className="mb-4 p-3 bg-rose-900/40 border border-rose-800 text-rose-300 text-xs rounded-lg">
              Error fetching telemetry: {error}
            </div>
          )}

          {/* 1. Metric Overview Cards */}
          <ServiceStats
            data={latencyData}
            serviceName={selectedService}
            onCallUser={onCallUser}
          />

          {/* 2. D3 2D Latency Heatmap */}
          {heatmapData && (
            <Heatmap
              data={heatmapData}
              onSelectBucket={handleCellSelect}
              onSelectRegion={handleRegionSelect}
            />
          )}

          {/* 3. Latency Percentiles (P50 / P95 / P99) Timeseries */}
          {latencyData.length > 0 && (
            <LatencyTimeseries data={latencyData} />
          )}

          {/* 4. Error Analytics & Grouped Root Causes */}
          {errorSummary && (
            <ErrorRatePanel
              errors={errorSummary}
              latencyData={latencyData}
              onSelectTrace={handleSelectTrace}
            />
          )}
        </main>

        {/* Trace Drill-down Modal */}
        {selectedBucket && (
          <TraceModal
            bucket={selectedBucket}
            onClose={() => setSelectedBucket(null)}
          />
        )}
      </div>
    </>
  );
}
