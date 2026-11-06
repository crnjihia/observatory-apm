// path: web/components/Heatmap.tsx
import React, { useRef, useEffect, useState } from 'react';
import * as d3 from 'd3';
import { HeatmapResponse, HeatmapCell, PercentileOverlay } from '../utils/api';

export interface HeatmapSelection {
  startTime: string;
  endTime: string;
  minLatencyMs: number;
  maxLatencyMs: number;
}

interface HeatmapProps {
  data: HeatmapResponse;
  onSelectBucket?: (cell: HeatmapCell) => void;
  onSelectRegion?: (selection: HeatmapSelection) => void;
}

export const Heatmap: React.FC<HeatmapProps> = ({ data, onSelectBucket, onSelectRegion }) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const svgRef = useRef<SVGSVGElement>(null);
  const tooltipRef = useRef<HTMLDivElement>(null);

  const [showOverlays, setShowOverlays] = useState<boolean>(true);
  const [activeSelection, setActiveSelection] = useState<HeatmapSelection | null>(null);

  useEffect(() => {
    if (!svgRef.current || !data || !data.cells || data.cells.length === 0) return;

    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove();

    const margin = { top: 25, right: 30, bottom: 40, left: 60 };
    const width = 960 - margin.left - margin.right;
    const height = 360 - margin.top - margin.bottom;

    const g = svg
      .append('g')
      .attr('transform', `translate(${margin.left},${margin.top})`);

    // Parse timestamps
    const parseTime = d3.isoParse;
    const timeBuckets = Array.from(new Set(data.cells.map((c) => c.time_bucket))).sort();
    const timeDates = timeBuckets.map((t) => parseTime(t) as Date);

    const minTime = d3.min(timeDates) || new Date();
    const maxTime = d3.max(timeDates) || new Date();

    // X Scale: Time
    const xScale = d3.scaleTime().domain([minTime, maxTime]).range([0, width]);

    // Y Scale: Latency
    const maxDuration = data.max_duration_ms > 0 ? data.max_duration_ms : 3500;
    const yScale = d3.scaleLinear().domain([0, maxDuration]).range([height, 0]);

    // Color Scale: Count intensity using Plasma / Turbo
    const maxCount = d3.max(data.cells, (d) => d.count) || 1;
    const colorScale = d3
      .scaleSequential(d3.interpolateTurbo)
      .domain([0, Math.max(10, maxCount)]);

    // SLA 1500ms Threshold line
    g.append('line')
      .attr('x1', 0)
      .attr('x2', width)
      .attr('y1', yScale(1500))
      .attr('y2', yScale(1500))
      .attr('stroke', '#f43f5e')
      .attr('stroke-dasharray', '4 3')
      .attr('stroke-width', 1.5)
      .attr('opacity', 0.85);

    g.append('text')
      .attr('x', width - 8)
      .attr('y', yScale(1500) - 5)
      .attr('text-anchor', 'end')
      .attr('fill', '#f43f5e')
      .attr('font-size', '10px')
      .attr('font-weight', '600')
      .text('1500ms SLA Threshold (Daraja Timeout)');

    // Cell dimensions
    const cellWidth = Math.max(8, width / Math.max(1, timeBuckets.length) - 2);

    // Draw Heatmap Cells
    const cellGroup = g.append('g').attr('class', 'cells');

    cellGroup
      .selectAll('rect')
      .data(data.cells)
      .enter()
      .append('rect')
      .attr('x', (d) => {
        const t = parseTime(d.time_bucket);
        return t ? xScale(t) - cellWidth / 2 : 0;
      })
      .attr('y', (d) => yScale(d.latency_max_ms))
      .attr('width', cellWidth)
      .attr('height', (d) => {
        const h = yScale(d.latency_min_ms) - yScale(d.latency_max_ms);
        return Math.max(2, h - 1);
      })
      .attr('rx', 2)
      .attr('fill', (d) => {
        if (d.error_count > 0 && d.error_count >= d.count * 0.4) {
          return '#e11d48'; // crimson accent for error clusters
        }
        return colorScale(d.count);
      })
      .attr('stroke', (d) => (d.error_count > 0 ? '#fda4af' : 'transparent'))
      .attr('stroke-width', (d) => (d.error_count > 0 ? 1 : 0))
      .attr('class', 'cursor-pointer transition-opacity hover:opacity-100 opacity-90')
      .on('mouseenter', (event: MouseEvent, d) => {
        if (!tooltipRef.current || !event.currentTarget) return;
        const target = event.currentTarget as SVGRectElement;
        const tt = d3.select(tooltipRef.current);
        const rect = target.getBoundingClientRect();
        const parentRect = containerRef.current?.getBoundingClientRect();

        const xPos = rect.left - (parentRect?.left || 0) + cellWidth / 2;
        const yPos = rect.top - (parentRect?.top || 0) - 10;

        const errorPercent = d.count > 0 ? ((d.error_count / d.count) * 100).toFixed(1) : '0';

        tt.style('display', 'block')
          .style('left', `${xPos}px`)
          .style('top', `${yPos}px`)
          .html(`
            <div class="p-2.5 bg-slate-900/95 border border-slate-700 rounded-lg shadow-xl text-xs backdrop-blur pointer-events-none">
              <div class="text-[11px] text-slate-400 font-mono mb-1">
                ⏰ ${new Date(d.time_bucket).toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' })} EAT
              </div>
              <div class="text-white font-semibold flex items-center justify-between mb-1">
                <span>Latency Range:</span>
                <span class="text-amber-400 font-mono ml-2">${Math.round(d.latency_min_ms)} - ${Math.round(d.latency_max_ms)} ms</span>
              </div>
              <div class="flex items-center justify-between text-slate-300 mb-1">
                <span>Sample Count:</span>
                <span class="font-mono font-bold text-white ml-2">${d.count.toLocaleString()}</span>
              </div>
              ${
                d.error_count > 0
                  ? `<div class="flex items-center justify-between text-rose-400 font-semibold border-t border-slate-800 pt-1 mt-1">
                      <span>Errors (${errorPercent}%):</span>
                      <span class="font-mono ml-2">${d.error_count}</span>
                    </div>`
                  : ''
              }
              <div class="text-[10px] text-indigo-400 mt-1 italic">Click cell or brush to inspect traces</div>
            </div>
          `);
      })
      .on('mouseleave', () => {
        if (tooltipRef.current) {
          d3.select(tooltipRef.current).style('display', 'none');
        }
      })
      .on('click', (_, d) => {
        if (onSelectBucket) onSelectBucket(d);
        if (onSelectRegion) {
          onSelectRegion({
            startTime: d.time_bucket,
            endTime: d.time_bucket,
            minLatencyMs: d.latency_min_ms,
            maxLatencyMs: d.latency_max_ms,
          });
        }
      });

    // Percentile Overlays (P50, P95, P99)
    if (showOverlays && data.overlays && data.overlays.length > 0) {
      const lineGroup = g.append('g').attr('class', 'overlays');

      // P50 line (emerald)
      const p50Line = d3
        .line<PercentileOverlay>()
        .defined((d) => d.p50 > 0)
        .x((d) => xScale(parseTime(d.time_bucket) as Date))
        .y((d) => yScale(d.p50))
        .curve(d3.curveMonotoneX);

      lineGroup
        .append('path')
        .datum(data.overlays)
        .attr('fill', 'none')
        .attr('stroke', '#10b981')
        .attr('stroke-width', 2)
        .attr('stroke-dasharray', '3 2')
        .attr('d', p50Line);

      // P95 line (amber)
      const p95Line = d3
        .line<PercentileOverlay>()
        .defined((d) => d.p95 > 0)
        .x((d) => xScale(parseTime(d.time_bucket) as Date))
        .y((d) => yScale(d.p95))
        .curve(d3.curveMonotoneX);

      lineGroup
        .append('path')
        .datum(data.overlays)
        .attr('fill', 'none')
        .attr('stroke', '#f59e0b')
        .attr('stroke-width', 2.5)
        .attr('d', p95Line);

      // P99 line (rose)
      const p99Line = d3
        .line<PercentileOverlay>()
        .defined((d) => d.p99 > 0)
        .x((d) => xScale(parseTime(d.time_bucket) as Date))
        .y((d) => yScale(d.p99))
        .curve(d3.curveMonotoneX);

      lineGroup
        .append('path')
        .datum(data.overlays)
        .attr('fill', 'none')
        .attr('stroke', '#f43f5e')
        .attr('stroke-width', 2.5)
        .attr('d', p99Line);
    }

    // 2D D3 Brush for Interactive Region Drill-down
    const brush = d3
      .brush()
      .extent([
        [0, 0],
        [width, height],
      ])
      .on('end', (event) => {
        if (!event.selection) {
          setActiveSelection(null);
          return;
        }
        const [[x0, y0], [x1, y1]] = event.selection as [[number, number], [number, number]];
        const t0 = xScale.invert(x0);
        const t1 = xScale.invert(x1);
        const latMax = yScale.invert(y0);
        const latMin = yScale.invert(y1);

        const sel: HeatmapSelection = {
          startTime: t0.toISOString(),
          endTime: t1.toISOString(),
          minLatencyMs: Math.max(0, Math.round(latMin)),
          maxLatencyMs: Math.round(latMax),
        };

        setActiveSelection(sel);
        if (onSelectRegion) {
          onSelectRegion(sel);
        }
      });

    g.append('g').attr('class', 'brush').call(brush);

    // Axes
    const xAxis = d3
      .axisBottom(xScale)
      .ticks(6)
      .tickFormat((d) =>
        d instanceof Date
          ? d3.timeFormat('%H:%M')(d)
          : d3.timeFormat('%H:%M')(new Date(Number(d)))
      );

    const yAxis = d3
      .axisLeft(yScale)
      .ticks(6)
      .tickFormat((d) => `${d}ms`);

    g.append('g')
      .attr('transform', `translate(0,${height})`)
      .attr('class', 'text-slate-400 text-xs font-mono')
      .call(xAxis as any)
      .selectAll('text')
      .attr('fill', '#94a3b8');

    g.append('g')
      .attr('class', 'text-slate-400 text-xs font-mono')
      .call(yAxis as any)
      .selectAll('text')
      .attr('fill', '#94a3b8');

    // Axes labels
    g.append('text')
      .attr('x', width / 2)
      .attr('y', height + 35)
      .attr('text-anchor', 'middle')
      .attr('fill', '#94a3b8')
      .attr('font-size', '11px')
      .attr('font-weight', '500')
      .text('Time (Nairobi EAT UTC+3)');

    g.append('text')
      .attr('transform', 'rotate(-90)')
      .attr('x', -height / 2)
      .attr('y', -45)
      .attr('text-anchor', 'middle')
      .attr('fill', '#94a3b8')
      .attr('font-size', '11px')
      .attr('font-weight', '500')
      .text('Latency (ms)');
  }, [data, showOverlays, onSelectBucket, onSelectRegion]);

  return (
    <div
      ref={containerRef}
      className="relative bg-slate-900 border border-slate-800 rounded-xl p-4 shadow-sm mb-6"
    >
      {/* Header with Title and Overlays Toggle */}
      <div className="flex flex-wrap items-center justify-between mb-3 gap-2">
        <div className="flex items-center space-x-3">
          <h2 className="text-sm font-semibold text-white tracking-tight flex items-center">
            <span className="w-2 h-2 rounded-full bg-indigo-500 mr-2"></span>
            Latency Distribution Heatmap
          </h2>
          <span className="text-xs text-slate-400">
            {data.service} • 2D time x duration density
          </span>
        </div>

        {/* Legend & Controls */}
        <div className="flex items-center space-x-4 text-xs">
          <div className="flex items-center space-x-3">
            <label className="flex items-center space-x-1.5 cursor-pointer text-slate-300 hover:text-white">
              <input
                type="checkbox"
                checked={showOverlays}
                onChange={(e) => setShowOverlays(e.target.checked)}
                className="rounded border-slate-700 bg-slate-800 text-indigo-600 focus:ring-0"
              />
              <span>Percentile Curves</span>
            </label>
            {showOverlays && (
              <div className="flex items-center space-x-2 font-mono text-[10px]">
                <span className="text-emerald-400 flex items-center">
                  <span className="w-2 h-0.5 bg-emerald-400 mr-1"></span> P50
                </span>
                <span className="text-amber-400 flex items-center">
                  <span className="w-2 h-0.5 bg-amber-400 mr-1"></span> P95
                </span>
                <span className="text-rose-400 flex items-center">
                  <span className="w-2 h-0.5 bg-rose-400 mr-1"></span> P99
                </span>
              </div>
            )}
          </div>

          {activeSelection && (
            <button
              onClick={() => {
                setActiveSelection(null);
                if (svgRef.current) {
                  const svg = d3.select(svgRef.current);
                  svg.select('.brush').call(d3.brush().clear as any);
                }
              }}
              className="text-[11px] px-2 py-0.5 rounded bg-rose-500/20 text-rose-300 border border-rose-500/30 hover:bg-rose-500/30"
            >
              Clear Brush Filter
            </button>
          )}
        </div>
      </div>

      {/* Responsive SVG Container */}
      <div className="w-full overflow-x-auto">
        <svg
          ref={svgRef}
          viewBox="0 0 960 360"
          className="w-full h-auto min-w-[700px] select-none"
        />
      </div>

      {/* Floating Hover Tooltip */}
      <div
        ref={tooltipRef}
        className="absolute z-50 pointer-events-none -translate-x-1/2 -translate-y-full transition-all duration-75"
        style={{ display: 'none' }}
      />
    </div>
  );
};
