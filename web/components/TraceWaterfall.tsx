// path: web/components/TraceWaterfall.tsx
import React, { useState, useMemo } from 'react';
import { SpanRow } from '../utils/api';

interface TraceWaterfallProps {
  spans: SpanRow[];
  selectedSpanId?: string | null;
  onSelectSpan?: (span: SpanRow) => void;
}

interface SpanTreeNode extends SpanRow {
  depth: number;
  durationMs: number;
  offsetMs: number;
  children: SpanTreeNode[];
}

export const TraceWaterfall: React.FC<TraceWaterfallProps> = ({
  spans,
  selectedSpanId,
  onSelectSpan,
}) => {
  const [activeSpanId, setActiveSpanId] = useState<string | null>(selectedSpanId || null);
  const [expandedSpanIds, setExpandedSpanIds] = useState<Record<string, boolean>>({});

  // Compute timing bounds and hierarchical tree
  const { tree, totalDurationMs, minStartMs } = useMemo(() => {
    if (!spans || spans.length === 0) {
      return { tree: [], totalDurationMs: 1, minStartMs: 0 };
    }

    // 1. Calculate trace start and end epochs
    let minStart = Infinity;
    let maxEnd = -Infinity;

    spans.forEach((s) => {
      const startMs = new Date(s.start_ts).getTime();
      const durMs = Math.max(0.1, s.duration_ns / 1_000_000);
      const endMs = startMs + durMs;

      if (startMs < minStart) minStart = startMs;
      if (endMs > maxEnd) maxEnd = endMs;
    });

    const totalDur = Math.max(1, maxEnd - minStart);

    // 2. Build map and tree
    const spanMap = new Map<string, SpanTreeNode>();
    spans.forEach((s) => {
      const startMs = new Date(s.start_ts).getTime();
      const durMs = Math.max(0.1, s.duration_ns / 1_000_000);
      spanMap.set(s.span_id, {
        ...s,
        depth: 0,
        durationMs: durMs,
        offsetMs: Math.max(0, startMs - minStart),
        children: [],
      });
    });

    const roots: SpanTreeNode[] = [];
    spanMap.forEach((node) => {
      if (node.parent_id && spanMap.has(node.parent_id)) {
        spanMap.get(node.parent_id)!.children.push(node);
      } else {
        roots.push(node);
      }
    });

    // Recursively assign depths
    const assignDepth = (nodes: SpanTreeNode[], depth: number) => {
      nodes.forEach((n) => {
        n.depth = depth;
        if (n.children.length > 0) {
          assignDepth(n.children, depth + 1);
        }
      });
    };
    assignDepth(roots, 0);

    // Flatten tree into display order
    const flattened: SpanTreeNode[] = [];
    const flatten = (nodes: SpanTreeNode[]) => {
      nodes.forEach((n) => {
        flattened.push(n);
        if (n.children.length > 0) {
          flatten(n.children);
        }
      });
    };
    flatten(roots);

    return { tree: flattened, totalDurationMs: totalDur, minStartMs: minStart };
  }, [spans]);

  const activeSpan = useMemo(() => {
    return spans.find((s) => s.span_id === activeSpanId) || null;
  }, [spans, activeSpanId]);

  const toggleExpand = (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    setExpandedSpanIds((prev) => ({ ...prev, [id]: !prev[id] }));
  };

  const handleRowClick = (span: SpanTreeNode) => {
    setActiveSpanId(span.span_id);
    if (onSelectSpan) onSelectSpan(span);
  };

  // Color generator based on service name
  const getServiceColor = (service: string, isError: boolean) => {
    if (isError) return 'from-rose-500 to-rose-600 border-rose-400';
    const hash = service.split('').reduce((acc, c) => acc + c.charCodeAt(0), 0);
    const hues = [
      'from-indigo-500 to-indigo-600 border-indigo-400',
      'from-sky-500 to-sky-600 border-sky-400',
      'from-emerald-500 to-emerald-600 border-emerald-400',
      'from-amber-500 to-amber-600 border-amber-400',
      'from-purple-500 to-purple-600 border-purple-400',
    ];
    return hues[hash % hues.length];
  };

  return (
    <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden shadow-lg">
      {/* Header Info */}
      <div className="px-4 py-3 bg-slate-900/90 border-b border-slate-800 flex items-center justify-between text-xs">
        <div className="flex items-center space-x-3">
          <span className="font-semibold text-white">Distributed Trace Waterfall</span>
          <span className="px-2 py-0.5 rounded bg-slate-800 text-slate-300 font-mono">
            {spans.length} {spans.length === 1 ? 'span' : 'spans'}
          </span>
          <span className="text-slate-400">
            Total Duration: <strong className="text-slate-200 font-mono">{totalDurationMs.toFixed(1)} ms</strong>
          </span>
        </div>
        <div className="flex items-center space-x-2 text-[11px] text-slate-400">
          <span className="flex items-center">
            <span className="w-2.5 h-2.5 rounded bg-rose-500 mr-1.5"></span> Error Span
          </span>
          <span className="flex items-center ml-2">
            <span className="w-2.5 h-2.5 rounded bg-indigo-500 mr-1.5"></span> OK Span
          </span>
        </div>
      </div>

      {/* Timeline Scale Marks */}
      <div className="grid grid-cols-12 bg-slate-950/80 border-b border-slate-800/80 text-[10px] font-mono text-slate-400 px-4 py-1.5">
        <div className="col-span-5 text-slate-400 font-sans font-medium">Operation & Service Hierarchy</div>
        <div className="col-span-7 flex justify-between px-2">
          <span>0 ms</span>
          <span>{(totalDurationMs * 0.25).toFixed(0)} ms</span>
          <span>{(totalDurationMs * 0.5).toFixed(0)} ms</span>
          <span>{(totalDurationMs * 0.75).toFixed(0)} ms</span>
          <span>{totalDurationMs.toFixed(0)} ms</span>
        </div>
      </div>

      {/* Spans List & Gantt Bars */}
      <div className="divide-y divide-slate-800/50 max-h-[380px] overflow-y-auto">
        {tree.map((node) => {
          const isSelected = node.span_id === activeSpanId;
          const isError = node.status === 'STATUS_CODE_ERROR';
          const leftPercent = Math.min(99, Math.max(0, (node.offsetMs / totalDurationMs) * 100));
          const widthPercent = Math.min(
            100 - leftPercent,
            Math.max(1.5, (node.durationMs / totalDurationMs) * 100)
          );

          return (
            <div
              key={node.span_id}
              onClick={() => handleRowClick(node)}
              className={`grid grid-cols-12 items-center px-4 py-2 text-xs cursor-pointer transition-colors ${
                isSelected
                  ? 'bg-indigo-950/40 border-l-4 border-indigo-500'
                  : 'hover:bg-slate-800/40'
              }`}
            >
              {/* Left Column: Indented Name + Service Badge */}
              <div
                className="col-span-5 flex items-center pr-3 truncate"
                style={{ paddingLeft: `${node.depth * 18}px` }}
              >
                {/* Status indicator dot */}
                <span
                  className={`w-2 h-2 rounded-full mr-2 shrink-0 ${
                    isError ? 'bg-rose-500 animate-pulse' : 'bg-emerald-400'
                  }`}
                />

                <span
                  className="font-medium text-slate-200 truncate mr-2"
                  title={node.name}
                >
                  {node.name}
                </span>

                <span className="shrink-0 px-1.5 py-0.2 text-[10px] rounded bg-slate-800 text-slate-400 border border-slate-700 font-mono">
                  {node.service}
                </span>
              </div>

              {/* Right Column: Timeline Gantt Bar */}
              <div className="col-span-7 relative h-6 flex items-center px-2">
                {/* Background grid line guide */}
                <div className="absolute inset-x-2 inset-y-0 border-l border-r border-slate-800/40 pointer-events-none" />

                {/* Duration Bar */}
                <div
                  className={`absolute h-4 rounded shadow-sm bg-gradient-to-r border transition-all ${getServiceColor(
                    node.service,
                    isError
                  )}`}
                  style={{
                    left: `${leftPercent}%`,
                    width: `${widthPercent}%`,
                  }}
                  title={`${node.name} (${node.durationMs.toFixed(1)} ms)`}
                />

                {/* Duration text beside/inside bar */}
                <span
                  className="absolute text-[10px] font-mono font-medium text-slate-300 drop-shadow"
                  style={{
                    left: `calc(${leftPercent + widthPercent}% + 6px)`,
                  }}
                >
                  {node.durationMs >= 1000
                    ? `${(node.durationMs / 1000).toFixed(2)}s`
                    : `${node.durationMs.toFixed(1)}ms`}
                </span>
              </div>
            </div>
          );
        })}
      </div>

      {/* Selected Span Details Inspection Drawer */}
      {activeSpan && (
        <div className="bg-slate-950 border-t border-slate-800 p-4 text-xs">
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center space-x-2">
              <span
                className={`w-2 h-2 rounded-full ${
                  activeSpan.status === 'STATUS_CODE_ERROR' ? 'bg-rose-500' : 'bg-emerald-400'
                }`}
              />
              <span className="font-bold text-white text-sm">{activeSpan.name}</span>
              <span className="px-2 py-0.5 rounded bg-indigo-500/20 text-indigo-300 font-mono text-[11px]">
                {activeSpan.service}
              </span>
              <span className="text-slate-400 font-mono text-[11px]">
                {activeSpan.kind || 'SPAN_KIND_SERVER'}
              </span>
            </div>
            <div className="font-mono text-slate-300">
              Duration: <strong className="text-amber-400">{(activeSpan.duration_ns / 1_000_000).toFixed(2)} ms</strong>
            </div>
          </div>

          <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 mb-3 font-mono text-[11px]">
            <div className="bg-slate-900 p-2 rounded border border-slate-800">
              <span className="text-slate-500 block text-[10px]">SPAN ID</span>
              <span className="text-slate-200 select-all">{activeSpan.span_id}</span>
            </div>
            <div className="bg-slate-900 p-2 rounded border border-slate-800">
              <span className="text-slate-500 block text-[10px]">PARENT ID</span>
              <span className="text-slate-200 select-all">{activeSpan.parent_id || 'root'}</span>
            </div>
            <div className="bg-slate-900 p-2 rounded border border-slate-800">
              <span className="text-slate-500 block text-[10px]">START TIME</span>
              <span className="text-slate-200">
                {new Date(activeSpan.start_ts).toLocaleTimeString('en-GB', {
                  hour: '2-digit',
                  minute: '2-digit',
                  second: '2-digit',
                })}
              </span>
            </div>
            <div className="bg-slate-900 p-2 rounded border border-slate-800">
              <span className="text-slate-500 block text-[10px]">STATUS</span>
              <span
                className={`font-semibold ${
                  activeSpan.status === 'STATUS_CODE_ERROR' ? 'text-rose-400' : 'text-emerald-400'
                }`}
              >
                {activeSpan.status || 'STATUS_CODE_OK'}
              </span>
            </div>
          </div>

          {/* Span Attributes / Tags */}
          {activeSpan.attrs && Object.keys(activeSpan.attrs).length > 0 && (
            <div>
              <span className="text-[10px] font-semibold text-slate-400 uppercase tracking-wide block mb-1.5">
                Attributes & Context
              </span>
              <div className="bg-slate-900/90 rounded border border-slate-800 overflow-x-auto">
                <table className="w-full text-left font-mono text-[11px]">
                  <tbody className="divide-y divide-slate-800/60">
                    {Object.entries(activeSpan.attrs).map(([k, v]) => (
                      <tr key={k} className="hover:bg-slate-800/30">
                        <td className="py-1 px-3 text-slate-400 w-1/3 border-r border-slate-800">{k}</td>
                        <td className="py-1 px-3 text-slate-200 break-all">
                          {typeof v === 'object' ? JSON.stringify(v) : String(v)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
};

