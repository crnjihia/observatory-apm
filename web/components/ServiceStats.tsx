// path: web/components/ServiceStats.tsx
import React from 'react';
import { LatencyRow } from '../utils/api';

interface ServiceStatsProps {
  data: LatencyRow[];
  serviceName: string;
  onCallUser?: string;
}

export const ServiceStats: React.FC<ServiceStatsProps> = ({
  data,
  serviceName,
  onCallUser = '+254712345678 (Christopher Njihia)',
}) => {
  // Compute aggregates across latency buckets
  const totalRequests = data.reduce((acc, row) => acc + (row.span_count || 0), 0);
  const avgErrorRate =
    data.length > 0 ? (data.reduce((acc, r) => acc + (r.error_rate || 0), 0) / data.length) * 100 : 0;

  // Max or weighted percentiles
  const p50 = data.length > 0 ? Math.round(data.reduce((acc, r) => acc + (r.p50 || 0), 0) / data.length) : 0;
  const p95 = data.length > 0 ? Math.round(data.reduce((acc, r) => acc + (r.p95 || 0), 0) / data.length) : 0;
  const p99 = data.length > 0 ? Math.round(Math.max(...data.map((r) => r.p99 || 0))) : 0;

  const isP99Critical = p99 >= 1500; // Daraja 504 threshold
  const isErrorCritical = avgErrorRate >= 1.0;

  return (
    <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-3 mb-6">
      {/* 1. Throughput */}
      <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-3.5 shadow-sm">
        <div className="flex items-center justify-between text-slate-400 text-xs font-medium mb-1">
          <span>Throughput</span>
          <svg className="w-4 h-4 text-sky-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
          </svg>
        </div>
        <div className="text-xl font-bold text-white tracking-tight">
          {totalRequests.toLocaleString()}
          <span className="text-[11px] font-normal text-slate-400 ml-1">reqs</span>
        </div>
        <div className="text-[10px] text-slate-500 mt-1 flex items-center">
          <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 inline-block mr-1"></span>
          {Math.round(totalRequests / Math.max(1, data.length))} req/bucket
        </div>
      </div>

      {/* 2. P50 Latency */}
      <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-3.5 shadow-sm">
        <div className="flex items-center justify-between text-slate-400 text-xs font-medium mb-1">
          <span>P50 Latency</span>
          <span className="text-[10px] px-1.5 py-0.2 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
            Median
          </span>
        </div>
        <div className="text-xl font-bold text-emerald-400 tracking-tight">
          {p50}
          <span className="text-xs font-normal text-slate-400 ml-1">ms</span>
        </div>
        <div className="text-[10px] text-slate-500 mt-1">Normal service latency</div>
      </div>

      {/* 3. P95 Latency */}
      <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-3.5 shadow-sm">
        <div className="flex items-center justify-between text-slate-400 text-xs font-medium mb-1">
          <span>P95 Latency</span>
          <span className="text-[10px] px-1.5 py-0.2 rounded bg-amber-500/10 text-amber-400 border border-amber-500/20">
            Tail
          </span>
        </div>
        <div className="text-xl font-bold text-amber-400 tracking-tight">
          {p95}
          <span className="text-xs font-normal text-slate-400 ml-1">ms</span>
        </div>
        <div className="text-[10px] text-slate-500 mt-1">95% under threshold</div>
      </div>

      {/* 4. P99 Latency (SLA) */}
      <div
        className={`bg-slate-900/90 border rounded-xl p-3.5 shadow-sm transition-colors ${
          isP99Critical ? 'border-rose-500/50 bg-rose-950/10' : 'border-slate-800'
        }`}
      >
        <div className="flex items-center justify-between text-slate-400 text-xs font-medium mb-1">
          <span>P99 (SLA Peak)</span>
          <span
            className={`text-[10px] px-1.5 py-0.2 rounded font-semibold ${
              isP99Critical
                ? 'bg-rose-500/20 text-rose-300 border border-rose-500/40 animate-pulse'
                : 'bg-indigo-500/10 text-indigo-400 border border-indigo-500/20'
            }`}
          >
            {isP99Critical ? 'SLA BREACH' : 'OK (<1.5s)'}
          </span>
        </div>
        <div
          className={`text-xl font-bold tracking-tight ${
            isP99Critical ? 'text-rose-400' : 'text-indigo-300'
          }`}
        >
          {p99}
          <span className="text-xs font-normal text-slate-400 ml-1">ms</span>
        </div>
        <div className="text-[10px] text-slate-500 mt-1">
          {isP99Critical ? 'Daraja 1500ms timeout' : 'Within Safaricom SLA'}
        </div>
      </div>

      {/* 5. Error Rate */}
      <div
        className={`bg-slate-900/90 border rounded-xl p-3.5 shadow-sm transition-colors ${
          isErrorCritical ? 'border-rose-500/50 bg-rose-950/10' : 'border-slate-800'
        }`}
      >
        <div className="flex items-center justify-between text-slate-400 text-xs font-medium mb-1">
          <span>Error Rate</span>
          <span
            className={`text-[10px] px-1.5 py-0.2 rounded font-semibold ${
              isErrorCritical
                ? 'bg-rose-500/20 text-rose-300 border border-rose-500/40'
                : 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
            }`}
          >
            {isErrorCritical ? 'Degraded' : 'Nominal'}
          </span>
        </div>
        <div
          className={`text-xl font-bold tracking-tight ${
            isErrorCritical ? 'text-rose-400' : 'text-emerald-400'
          }`}
        >
          {avgErrorRate.toFixed(2)}
          <span className="text-xs font-normal text-slate-400 ml-0.5">%</span>
        </div>
        <div className="text-[10px] text-slate-500 mt-1">
          {isErrorCritical ? 'SLA Alert > 1.0%' : 'Under 1% threshold'}
        </div>
      </div>

      {/* 6. Active On-Call Contact */}
      <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-3.5 shadow-sm">
        <div className="flex items-center justify-between text-slate-400 text-xs font-medium mb-1">
          <span>On-Call Rotation</span>
          <span className="flex h-2 w-2 relative">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
            <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
          </span>
        </div>
        <div className="text-xs font-semibold text-slate-200 truncate mt-1">
          {onCallUser.split(' ')[0]}
        </div>
        <div className="text-[10px] text-slate-400 font-mono truncate mt-0.5">
          {onCallUser.split(' ').slice(1).join(' ') || onCallUser}
        </div>
        <div className="text-[10px] text-slate-500 mt-1">SMS Escalation Active</div>
      </div>
    </div>
  );
};

