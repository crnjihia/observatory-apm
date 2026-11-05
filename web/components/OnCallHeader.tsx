// path: C:/Users/crnji/Videos/Projects/Web Development/New/observatory-apm/web/components/OnCallHeader.tsx
import React, { useEffect, useState } from 'react';
import { fetchCurrentOnCall } from '../utils/api';

export const OnCallHeader: React.FC = () => {
  const [onCall, setOnCall] = useState<string>('');
  const [error, setError] = useState<string>('');

  useEffect(() => {
    fetchCurrentOnCall()
      .then(setOnCall)
      .catch(err => setError(err.message));
  }, []);

  return (
    <div className="p-4 bg-gray-100 rounded mb-4">
      <h2 className="text-xl font-semibold">Current On‑Call Engineer</h2>
      {error && <p className="text-red-600">Error: {error}</p>}
      <p className="mt-1">{onCall || '—'}</p>
    </div>
  );
};
