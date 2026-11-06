// path: web/pages/oncall.tsx
import React, { useState, useEffect } from 'react';
import Head from 'next/head';
import Link from 'next/link';
import { OnCallHeader } from '../components/OnCallHeader';
import { IncidentSimulator } from '../components/IncidentSimulator';
import { IncidentList } from '../components/IncidentList';

export default function OnCallPage() {
  const [refreshKey, setRefreshKey] = useState<number>(0);

  const handleIncidentInjected = () => {
    // Trigger refresh in IncidentList
    setRefreshKey((prev) => prev + 1);
  };

  return (
    <>
      <Head>
        <title>Observatory APM – On-Call & Fault Injection Simulator</title>
        <meta
          name="description"
          content="Simulate Kenyan telecom & fintech infrastructure failures, verify SMS paging and incident lifecycle."
        />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
      </Head>

      <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col font-sans">
        {/* Navigation Bar */}
        <header className="sticky top-0 z-40 w-full bg-slate-900/95 backdrop-blur border-b border-slate-800 text-slate-200">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
            <div className="flex items-center justify-between h-16">
              <div className="flex items-center space-x-6">
                <Link href="/" className="flex items-center space-x-3 group">
                  <div className="relative flex items-center justify-center w-9 h-9 rounded-lg bg-indigo-950 border border-indigo-500/30 text-indigo-400">
                    <svg className="w-5 h-5 animate-pulse" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
                    </svg>
                  </div>
                  <span className="text-lg font-bold tracking-tight text-white">Observatory</span>
                  <span className="ml-1 px-1.5 py-0.5 text-[10px] font-semibold tracking-wide uppercase rounded bg-indigo-500/20 text-indigo-300 border border-indigo-500/30">
                    On-Call
                  </span>
                </Link>

                <nav className="hidden md:flex space-x-1">
                  <Link
                    href="/"
                    className="px-3 py-1.5 text-sm font-medium rounded-md text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
                  >
                    Dashboard
                  </Link>
                  <Link
                    href="/alerts"
                    className="px-3 py-1.5 text-sm font-medium rounded-md text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
                  >
                    Alerts & Rules
                  </Link>
                  <Link
                    href="/oncall"
                    className="px-3 py-1.5 text-sm font-medium rounded-md bg-slate-800 text-white border border-slate-700"
                  >
                    On-Call & Incidents
                  </Link>
                </nav>
              </div>

              <div className="flex items-center space-x-2 text-xs font-mono text-slate-400">
                <span className="w-2 h-2 rounded-full bg-emerald-400"></span>
                <span>SMS Paging Active (+254)</span>
              </div>
            </div>
          </div>
        </header>

        {/* Main Content Area */}
        <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-6">
          {/* Active On-Call Header */}
          <OnCallHeader />

          {/* Interactive Fault Injection Simulator */}
          <IncidentSimulator onIncidentInjected={handleIncidentInjected} />

          {/* Incident Log & Actions */}
          <IncidentList key={refreshKey} />
        </main>
      </div>
    </>
  );
}
