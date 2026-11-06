// path: web/components/LatencyTimeseries.tsx
import React, { useRef, useEffect } from 'react';
import * as d3 from 'd3';
import { LatencyRow } from '../utils/api';

interface LatencyTimeseriesProps {
  data: LatencyRow[];
  title?: string;
}

export const LatencyTimeseries: React.FC<LatencyTimeseriesProps> = ({
  data,
  title = 'Percentile Latency Trends (P50 / P95 / P99)',
}) => {
  const svgRef = useRef<SVGSVGElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!svgRef.current || !data || data.length === 0) return;

    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove();

    const margin = { top: 20, right: 30, bottom: 35, left: 55 };
    const width = 960 - margin.left - margin.right;
    const height = 240 - margin.top - margin.bottom;

    const g = svg
      .append('g')
      .attr('transform', `translate(${margin.left},${margin.top})`);

    const parseTime = d3.isoParse;
    const times = data.map((d) => parseTime(d.bucket) as Date).filter(Boolean);

    const minTime = d3.min(times) || new Date();
    const maxTime = d3.max(times) || new Date();

    const xScale = d3.scaleTime().domain([minTime, maxTime]).range([0, width]);

    const maxP99 = d3.max(data, (d) => d.p99) || 1000;
    const yScale = d3
      .scaleLinear()
      .domain([0, Math.max(1600, maxP99 * 1.15)])
      .range([height, 0]);

    // Grid lines
    g.append('g')
      .attr('class', 'grid text-slate-800')
      .call(
        d3
          .axisLeft(yScale)
          .ticks(5)
          .tickSize(-width)
          .tickFormat(() => '')
      )
      .selectAll('line')
      .attr('stroke', '#1e293b')
      .attr('stroke-dasharray', '2 2');

    // 1500ms SLA Line
    g.append('line')
      .attr('x1', 0)
      .attr('x2', width)
      .attr('y1', yScale(1500))
      .attr('y2', yScale(1500))
      .attr('stroke', '#f43f5e')
      .attr('stroke-dasharray', '3 3')
      .attr('stroke-width', 1.2)
      .attr('opacity', 0.8);

    // Gradient for P50 area fill
    const defs = svg.append('defs');
    const areaGradient = defs
      .append('linearGradient')
      .attr('id', 'p50-gradient')
      .attr('x1', '0')
      .attr('y1', '0')
      .attr('x2', '0')
      .attr('y2', '1');

    areaGradient.append('stop').attr('offset', '0%').attr('stop-color', '#10b981').attr('stop-opacity', 0.25);
    areaGradient.append('stop').attr('offset', '100%').attr('stop-color', '#10b981').attr('stop-opacity', 0.0);

    // Area fill P50
    const p50Area = d3
      .area<LatencyRow>()
      .x((d) => xScale(parseTime(d.bucket) as Date))
      .y0(height)
      .y1((d) => yScale(d.p50))
      .curve(d3.curveMonotoneX);

    g.append('path').datum(data).attr('fill', 'url(#p50-gradient)').attr('d', p50Area);

    // Line generators
    const p50Line = d3
      .line<LatencyRow>()
      .x((d) => xScale(parseTime(d.bucket) as Date))
      .y((d) => yScale(d.p50))
      .curve(d3.curveMonotoneX);

    const p95Line = d3
      .line<LatencyRow>()
      .x((d) => xScale(parseTime(d.bucket) as Date))
      .y((d) => yScale(d.p95))
      .curve(d3.curveMonotoneX);

    const p99Line = d3
      .line<LatencyRow>()
      .x((d) => xScale(parseTime(d.bucket) as Date))
      .y((d) => yScale(d.p99))
      .curve(d3.curveMonotoneX);

    // Draw Lines
    g.append('path')
      .datum(data)
      .attr('fill', 'none')
      .attr('stroke', '#10b981')
      .attr('stroke-width', 2)
      .attr('d', p50Line);

    g.append('path')
      .datum(data)
      .attr('fill', 'none')
      .attr('stroke', '#f59e0b')
      .attr('stroke-width', 2)
      .attr('d', p95Line);

    g.append('path')
      .datum(data)
      .attr('fill', 'none')
      .attr('stroke', '#f43f5e')
      .attr('stroke-width', 2.5)
      .attr('d', p99Line);

    // Axes
    const xAxis = d3
      .axisBottom(xScale)
      .ticks(6)
      .tickFormat((d) =>
        d instanceof Date ? d3.timeFormat('%H:%M')(d) : d3.timeFormat('%H:%M')(new Date(Number(d)))
      );

    const yAxis = d3.axisLeft(yScale).ticks(5).tickFormat((d) => `${d}ms`);

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
  }, [data]);

  return (
    <div
      ref={containerRef}
      className="bg-slate-900 border border-slate-800 rounded-xl p-4 shadow-sm mb-6"
    >
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-sm font-semibold text-white tracking-tight flex items-center">
          <span className="w-2 h-2 rounded-full bg-emerald-500 mr-2"></span>
          {title}
        </h2>
        <div className="flex items-center space-x-3 text-xs font-mono">
          <span className="text-emerald-400 flex items-center">
            <span className="w-2.5 h-0.5 bg-emerald-400 mr-1.5 inline-block"></span> P50 Median
          </span>
          <span className="text-amber-400 flex items-center">
            <span className="w-2.5 h-0.5 bg-amber-400 mr-1.5 inline-block"></span> P95 Tail
          </span>
          <span className="text-rose-400 flex items-center">
            <span className="w-2.5 h-0.5 bg-rose-400 mr-1.5 inline-block"></span> P99 Peak SLA
          </span>
        </div>
      </div>
      <div className="w-full overflow-x-auto">
        <svg ref={svgRef} viewBox="0 0 960 240" className="w-full h-auto min-w-[700px]" />
      </div>
    </div>
  );
};

