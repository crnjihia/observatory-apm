// path: web/components/OnCallHeader.tsx
import React, { useEffect, useState } from 'react';
import { fetchCurrentOnCall } from '../utils/api';

export const OnCallHeader: React.FC = () => {
  const [onCall, setOnCall] = useState<string>('+254712345678 (Christopher Njihia)');
  const [error, setError] = useState<string>('');

  useEffect(() => {
    fetchCurrentOnCall()
      .then(setOnCall)
      .catch((err) => setError(err.message));
  }, []);

  return (
    <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 mb-6 shadow-sm">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="flex items-center space-x-4">
          <div className="relative flex items-center justify-center w-12 h-12 rounded-xl bg-indigo-950 border border-indigo-500/30 text-indigo-400">
            <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 18h.01M8 21h8a2 2 0 002-2V5a2 2 0 00-2-2H8a2 2 0 00-2 2v14a2 2 0 002 2z"
              />
            </svg>
            <span className="absolute -top-1 -right-1 flex h-3 w-3">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
              <span className="relative inline-flex rounded-full h-3 w-3 bg-emerald-500"></span>
            </span>
          </div>

          <div>
            <div className="flex items-center space-x-2">
              <h2 className="text-sm font-semibold text-white tracking-tight">Active Primary On-Call Engineer</h2>
              <span className="px-2 py-0.5 rounded text-[10px] font-semibold bg-emerald-500/20 text-emerald-300 border border-emerald-500/30">
                Paging Enabled
              </span>
            </div>
            {error && <p className="text-rose-400 text-xs mt-1">Error: {error}</p>}
            <div className="text-base font-bold text-white font-mono mt-0.5">{onCall}</div>
            <div className="text-xs text-slate-400 mt-0.5">
              Rotation: 24h cycle • SMS paging via Africa&apos;s Talking • 5m Escalation Timeout
            </div>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2 text-xs font-mono">
          <div className="px-3 py-1.5 rounded-lg bg-slate-950 border border-slate-800 text-slate-300">
            <span className="text-slate-500 block text-[10px]">TIMEZONE</span>
            <span>Africa/Nairobi (EAT)</span>
          </div>
          <div className="px-3 py-1.5 rounded-lg bg-slate-950 border border-slate-800 text-slate-300">
            <span className="text-slate-500 block text-[10px]">ESCALATION 1</span>
            <span>+254 712 345 678</span>
          </div>
          <div className="px-3 py-1.5 rounded-lg bg-slate-950 border border-slate-800 text-slate-300">
            <span className="text-slate-500 block text-[10px]">ESCALATION 2</span>
            <span>+254 722 000 001</span>
          </div>
        </div>
      </div>
    </div>
  );
};
