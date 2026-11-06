// path: web/components/Navbar.tsx
import React, { useState, useEffect } from 'react';
import Link from 'next/link';

interface NavbarProps {
  services: string[];
  selectedService: string;
  onSelectService: (service: string) => void;
  timeRange: string;
  onSelectTimeRange: (range: string) => void;
  autoRefresh: number;
  onSelectAutoRefresh: (sec: number) => void;
  onManualRefresh: () => void;
  loading?: boolean;
}

export const Navbar: React.FC<NavbarProps> = ({
  services,
  selectedService,
  onSelectService,
  timeRange,
  onSelectTimeRange,
  autoRefresh,
  onSelectAutoRefresh,
  onManualRefresh,
  loading = false,
}) => {
  const [currentTime, setCurrentTime] = useState<string>('');

  useEffect(() => {
    const updateTime = () => {
      const now = new Date();
      // Format time in Nairobi timezone EAT (UTC+3)
      setCurrentTime(
        now.toLocaleTimeString('en-GB', {
          timeZone: 'Africa/Nairobi',
          hour: '2-digit',
          minute: '2-digit',
          second: '2-digit',
        })
      );
    };
    updateTime();
    const timer = setInterval(updateTime, 1000);
    return () => clearInterval(timer);
  }, []);

  const ranges = [
    { label: '15m', val: '15m' },
    { label: '1h', val: '1h' },
    { label: '6h', val: '6h' },
    { label: '24h', val: '24h' },
  ];

  const refreshOptions = [
    { label: 'Off', val: 0 },
    { label: '5s', val: 5 },
    { label: '10s', val: 10 },
    { label: '30s', val: 30 },
  ];

  return (
    <header className="sticky top-0 z-40 w-full bg-slate-900/95 backdrop-blur border-b border-slate-800 text-slate-200">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between h-16">
          {/* Brand & Navigation */}
          <div className="flex items-center space-x-6">
            <Link href="/" className="flex items-center space-x-3 group">
              <div className="relative flex items-center justify-center w-9 h-9 rounded-lg bg-indigo-950 border border-indigo-500/30 text-indigo-400 group-hover:border-indigo-400 transition-colors">
                <svg className="w-5 h-5 animate-pulse" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
                </svg>
                <span className="absolute -top-1 -right-1 flex h-2.5 w-2.5">
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                  <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-emerald-500"></span>
                </span>
              </div>
              <div>
                <span className="text-lg font-bold tracking-tight text-white group-hover:text-indigo-300 transition-colors">
                  Observatory
                </span>
                <span className="ml-1.5 px-1.5 py-0.5 text-[10px] font-semibold tracking-wide uppercase rounded bg-indigo-500/20 text-indigo-300 border border-indigo-500/30">
                  APM
                </span>
              </div>
            </Link>

            <nav className="hidden md:flex space-x-1">
              <Link
                href="/"
                className="px-3 py-1.5 text-sm font-medium rounded-md bg-slate-800 text-white border border-slate-700/60"
              >
                Dashboard
              </Link>
              <Link
                href="/alerts"
                className="px-3 py-1.5 text-sm font-medium rounded-md text-slate-400 hover:text-white hover:bg-slate-800/60 transition-colors"
              >
                Alerts & Rules
              </Link>
              <Link
                href="/oncall"
                className="px-3 py-1.5 text-sm font-medium rounded-md text-slate-400 hover:text-white hover:bg-slate-800/60 transition-colors"
              >
                On-Call & Incidents
              </Link>
            </nav>
          </div>

          {/* Controls: Service selector, Time window, Refresh, Nairobi Clock */}
          <div className="flex items-center space-x-3">
            {/* Service Dropdown */}
            <div className="relative">
              <select
                value={selectedService}
                onChange={(e) => onSelectService(e.target.value)}
                className="bg-slate-800/90 hover:bg-slate-800 border border-slate-700 text-slate-100 text-xs sm:text-sm font-medium rounded-lg px-3 py-1.5 pr-8 focus:outline-none focus:ring-2 focus:ring-indigo-500 cursor-pointer appearance-none"
              >
                <option value="">Select Service</option>
                {services.map((svc) => (
                  <option key={svc} value={svc}>
                    {svc}
                  </option>
                ))}
              </select>
              <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center px-2 text-slate-400">
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                </svg>
              </div>
            </div>

            {/* Time Range Quick Buttons */}
            <div className="hidden sm:flex items-center bg-slate-800/80 p-0.5 rounded-lg border border-slate-700/80">
              {ranges.map((r) => (
                <button
                  key={r.val}
                  onClick={() => onSelectTimeRange(r.val)}
                  className={`px-2.5 py-1 text-xs font-medium rounded-md transition-all ${
                    timeRange === r.val
                      ? 'bg-indigo-600 text-white shadow-sm'
                      : 'text-slate-400 hover:text-slate-200'
                  }`}
                >
                  {r.label}
                </button>
              ))}
            </div>

            {/* Auto Refresh Dropdown */}
            <div className="flex items-center bg-slate-800/80 rounded-lg border border-slate-700/80 px-2 py-1">
              <span className="text-[11px] text-slate-400 mr-1.5">Auto:</span>
              <select
                value={autoRefresh}
                onChange={(e) => onSelectAutoRefresh(Number(e.target.value))}
                className="bg-transparent text-xs text-indigo-300 font-semibold focus:outline-none cursor-pointer"
              >
                {refreshOptions.map((opt) => (
                  <option key={opt.val} value={opt.val} className="bg-slate-800 text-slate-200">
                    {opt.label}
                  </option>
                ))}
              </select>
              {autoRefresh > 0 && (
                <span className="ml-1.5 flex h-2 w-2 relative">
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                  <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
                </span>
              )}
            </div>

            {/* Manual Refresh Button */}
            <button
              onClick={onManualRefresh}
              disabled={loading}
              title="Refresh Data"
              className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 border border-slate-700 text-slate-300 hover:text-white transition-colors disabled:opacity-50"
            >
              <svg
                className={`w-4 h-4 ${loading ? 'animate-spin text-indigo-400' : ''}`}
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                />
              </svg>
            </button>

            {/* Nairobi EAT Clock Badge */}
            <div className="hidden lg:flex items-center space-x-1.5 px-2.5 py-1 rounded-md bg-slate-800/40 border border-slate-800 text-[11px] font-mono text-slate-400">
              <span className="inline-block w-1.5 h-1.5 rounded-full bg-amber-400"></span>
              <span className="text-slate-300 font-semibold">{currentTime || '--:--:--'}</span>
              <span className="text-slate-500 text-[10px]">EAT (UTC+3)</span>
            </div>
          </div>
        </div>
      </div>
    </header>
  );
};

