// path: C:/Users/crnji/Videos/Projects/Web Development/New/observatory-apm/web/components/Heatmap.tsx
import React, { useRef, useEffect } from 'react';
import * as d3 from 'd3';
import { LatencyRow } from '../utils/api';

interface HeatmapProps {
  data: LatencyRow[];
  width?: number;
  height?: number;
  onBarClick?: (row: LatencyRow) => void;
}


// Simple heatmap: x‑axis = time bucket, y‑axis = latency bucket (derived from p95), color intensity = p95.
export const Heatmap: React.FC<HeatmapProps> = ({ data, width = 800, height = 300, onBarClick }) => {
  const svgRef = useRef<SVGSVGElement>(null);

  useEffect(() => {
    if (!data.length) return;
    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove(); // clear previous render

    // Parse timestamps.
    const parseTime = d3.utcParse('%Y-%m-%dT%H:%M:%S%Z');
    const times = data.map(d => parseTime(d.bucket) as Date);
    const p95s = data.map(d => d.p95);

    // Define scales.
    const xScale = d3.scaleTime()
      .domain(d3.extent(times) as [Date, Date])
      .range([40, width - 20]);

    const maxP95 = d3.max(p95s) ?? 1;
    const yScale = d3.scaleLinear()
      .domain([0, maxP95])
      .range([height - 30, 20]);

    const colorScale = d3.scaleSequential(d3.interpolateTurbo)
      .domain([0, maxP95]);

    // Draw rectangles (one per data point).
    svg.selectAll('rect')
      .data(data)
      .enter()
      .append('rect')
      .attr('x', d => xScale(parseTime(d.bucket) as Date) - 2) // thin column
      .attr('y', d => yScale(d.p95))
      .attr('width', 4)
      .attr('height', d => yScale(0) - yScale(d.p95))
      .attr('fill', d => colorScale(d.p95))
      .on('click', (event, d) => {
        if (onBarClick) onBarClick(d);
      })
      .append('title')
      .text(d => `⏰ ${d.bucket}\nP95: ${d.p95.toFixed(2)} ms`);

    // Axes.
    const xAxis = d3.axisBottom(xScale).ticks(5).tickFormat(d3.timeFormat('%H:%M'));
    const yAxis = d3.axisLeft(yScale).ticks(5);
    svg.append('g')
      .attr('transform', `translate(0,${height - 30})`)
      .call(xAxis);
    svg.append('g')
      .attr('transform', 'translate(40,0)')
      .call(yAxis);

    // Labels.
    svg.append('text')
      .attr('x', width / 2)
      .attr('y', height - 5)
      .attr('text-anchor', 'middle')
      .attr('font-size', '12px')
      .text('Time');
    svg.append('text')
      .attr('transform', 'rotate(-90)')
      .attr('x', -height / 2)
      .attr('y', 15)
      .attr('text-anchor', 'middle')
      .attr('font-size', '12px')
      .text('P95 Latency (ms)');
  }, [data, width, height]);

  return <svg ref={svgRef} width={width} height={height} className="border" />;
};
