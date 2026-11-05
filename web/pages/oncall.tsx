// path: C:/Users/crnji/Videos/Projects/Web Development/New/observatory-apm/web/pages/oncall.tsx
import React from 'react';
import { OnCallHeader } from '../components/OnCallHeader';
import { IncidentList } from '../components/IncidentList';

export default function OnCallPage() {
  return (
    <main className="max-w-4xl mx-auto p-4">
      <h1 className="text-2xl font-bold mb-4">On‑Call Simulator</h1>
      <OnCallHeader />
      <IncidentList />
    </main>
  );
};
