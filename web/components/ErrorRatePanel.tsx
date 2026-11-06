// path: web/components/ErrorRatePanel.tsx
import React, { useRef, useEffect } from 'react';
import * as d3 from 'd3';
import { ErrorSummaryResponse, ErrorGroup, LatencyRow } from '../utils/api';

interface ErrorRatePanelProps {
  errors: ErrorSummaryResponse;
  latencyData: LatencyRow[];
  onSelectTrace?: (traceId: string) => void;
  onSelectErrorGroup?: (group: ErrorGroup) => void;
}

export const ErrorRatePanel: React.FC<ErrorRatePanelProps> = ({
  errors,
  latencyData,
  onSelectTrace,
  onSelectErrorGroup,
}) => {
  const chartRef = useRef<SVGSVGElement>(null);

  useEffect(() => {
    if (!chartRef.current || !latencyData || latencyData.length === 0) return;

    const svg = d3.select(chartRef.current);
    svg.selectAll('*').remove();

    const margin = { top: 15, right: 20, bottom: 30, left: 45 };
    const width = 450 - margin.left - margin.right;
    const height = 180 - margin.top - margin.bottom;

    const g = svg.append('g').attr('transform', `translate(${margin.left},${margin.top})`);

    const parseTime = d3.isoParse;
    const times = latencyData.map((d) => parseTime(d.bucket) as Date).filter(Boolean);
    const minTime = d3.min(times) || new Date();
    const maxTime = d3.max(times) || new Date();

    const xScale = d3.scaleTime().domain([minTime, maxTime]).range([0, width]);

    const maxErrorRate = d3.max(latencyData, (d) => (d.error_rate || 0) * 100) || 1.0;
    const yScale = d3
      .scaleLinear()
      .domain([0, Math.max(2.0, maxErrorRate * 1.25)])
      .range([height, 0]);

    // 1.0% SLA line
    g.append('line')
      .attr('x1', 0)
      .attr('x2', width)
      .attr('y1', yScale(1.0))
      .attr('y2', yScale(1.0))
      .attr('stroke', '#f43f5e')
      .attr('stroke-dasharray', '3 2')
      .attr('stroke-width', 1.2);

    g.append('text')
      .attr('x', width - 4)
      .attr('y', yScale(1.0) - 4)
      .attr('text-anchor', 'end')
      .attr('fill', '#f43f5e')
      .attr('font-size', '9px')
      .text('1.0% SLA limit');

    // Bars for error rate
    const barWidth = Math.max(4, width / Math.max(1, latencyData.length) - 2);

    g.selectAll('rect')
      .data(latencyData)
      .enter()
      .append('rect')
      .attr('x', (d) => {
        const t = parseTime(d.bucket);
        return t ? xScale(t) - barWidth / 2 : 0;
      })
      .attr('y', (d) => yScale((d.error_rate || 0) * 100))
      .attr('width', barWidth)
      .attr('height', (d) => height - yScale((d.error_rate || 0) * 100))
      .attr('fill', (d) => ((d.error_rate || 0) * 100 >= 1.0 ? '#f43f5e' : '#fbbf24'))
      .attr('rx', 1.5)
      .attr('opacity', 0.85);

    // Axes
    const xAxis = d3
      .axisBottom(xScale)
      .ticks(4)
      .tickFormat((d) => (d instanceof Date ? d3.timeFormat('%H:%M')(d) : ''));

    const yAxis = d3.axisLeft(yScale).ticks(3).tickFormat((d) => `${d}%`);

    g.append('g')
      .attr('transform', `translate(0,${height})`)
      .attr('class', 'text-slate-400 text-[10px] font-mono')
      .call(xAxis as any)
      .selectAll('text')
      .attr('fill', '#94a3b8');

    g.append('g')
      .attr('class', 'text-slate-400 text-[10px] font-mono')
      .call(yAxis as any)
      .selectAll('text')
      .attr('fill', '#94a3b8');
  }, [latencyData]);

  const totalErrors = errors?.total_errors || 0;

  return (
    <div className="grid grid-cols-1 lg:grid-cols-12 gap-6 mb-6">
      {/* Left Col: Error Rate Trend Chart */}
      <div className="lg:col-span-5 bg-slate-900 border border-slate-800 rounded-xl p-4 shadow-sm flex flex-col justify-between">
        <div>
          <div className="flex items-center justify-between mb-2">
            <h3 className="text-sm font-semibold text-white tracking-tight flex items-center">
              <span className="w-2 h-2 rounded-full bg-rose-500 mr-2"></span>
              Error Rate Over Time (%)
            </h3>
            <span
              className={`text-[10px] font-semibold px-2 py-0.5 rounded ${
                totalErrors > 0
                  ? 'bg-rose-500/20 text-rose-300 border border-rose-500/30'
                  : 'bg-emerald-500/20 text-emerald-300'
              }`}
            >
              {totalErrors} Failed Spans
            </span>
          </div>
          <p className="text-xs text-slate-400 mb-3">
            Percentage of transactions returning HTTP 5xx or <code className="text-rose-400">STATUS_CODE_ERROR</code>.
          </p>
        </div>

        <div className="w-full overflow-hidden flex justify-center">
          <svg ref={chartRef} viewBox="0 0 450 180" className="w-full h-auto max-w-md" />
        </div>
      </div>

      {/* Right Col: Grouped Error Root Causes & Signatures */}
      <div className="lg:col-span-7 bg-slate-900 border border-slate-800 rounded-xl p-4 shadow-sm flex flex-col">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-sm font-semibold text-white tracking-tight flex items-center">
            <span className="w-2 h-2 rounded-full bg-amber-500 mr-2"></span>
            Top Error Signatures & Root Causes
          </h3>
          <span className="text-xs text-slate-400">Classified by failure pattern</span>
        </div>

        <div className="overflow-x-auto flex-1">
          <table className="w-full text-left text-xs text-slate-300">
            <thead className="bg-slate-800/60 text-slate-400 uppercase text-[10px] font-semibold border-b border-slate-800">
              <tr>
                <th className="py-2 px-3">Service / Endpoint</th>
                <th className="py-2 px-3">Error Signature</th>
                <th className="py-2 px-3 text-right">Count</th>
                <th className="py-2 px-3 text-right">Action</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60">
              {errors?.error_groups && errors.error_groups.length > 0 ? (
                errors.error_groups.map((eg, idx) => (
                  <tr key={idx} className="hover:bg-slate-800/40 transition-colors">
                    <td className="py-2.5 px-3">
                      <div className="font-semibold text-slate-100">{eg.service}</div>
                      <div className="text-[11px] text-slate-400 font-mono truncate max-w-[140px]">
                        {eg.endpoint}
                      </div>
                    </td>
                    <td className="py-2.5 px-3 max-w-[260px]">
                      <span className="text-rose-300 font-medium font-mono text-[11px] line-clamp-2">
                        {eg.error_message}
                      </span>
                    </td>
                    <td className="py-2.5 px-3 text-right font-mono font-bold text-white">
                      {eg.count}
                    </td>
                    <td className="py-2.5 px-3 text-right">
                      <button
                        onClick={() => {
                          if (onSelectErrorGroup) onSelectErrorGroup(eg);
                          // If there are recent spans matching this service, pick first trace
                          const match = errors.recent_spans?.find((s) => s.service === eg.service);
                          if (match && onSelectTrace) {
                            onSelectTrace(match.trace_id);
                          }
                        }}
                        className="text-[11px] font-semibold px-2 py-1 rounded bg-indigo-500/10 text-indigo-300 border border-indigo-500/30 hover:bg-indigo-500/20 transition-colors"
                      >
                        Drill Down
                      </button>
                    </td>
                  </tr>
                ))
              ) : (
                <tr>
                  <td colSpan={4} className="py-6 text-center text-slate-500 italic">
                    No errors detected in this service window. All transactions nominal.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};

