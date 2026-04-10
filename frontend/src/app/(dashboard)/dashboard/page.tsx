'use client';

import { useEffect, useState } from 'react';
import { ErrorBoundary } from '@/components/ErrorBoundary';
import { authService } from '@/services/auth';
import { UserProfileResponse } from '@/types';

function DashboardContent() {
  const [user, setUser] = useState<UserProfileResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    authService.getProfile()
      .then((res) => setUser(res.data))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <p className="text-gray-500">Loading...</p>;
  if (error) return <p className="text-red-600">{error}</p>;

  return (
    <section aria-labelledby="dashboard-heading">
      <h1 id="dashboard-heading" className="text-2xl font-bold mb-6">Dashboard</h1>
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="bg-white p-6 rounded-lg shadow">
          <h2 className="text-sm text-gray-500">Welcome</h2>
          <p className="text-xl font-semibold mt-1">{user?.full_name}</p>
        </div>
        <div className="bg-white p-6 rounded-lg shadow">
          <h2 className="text-sm text-gray-500">Role</h2>
          <p className="text-xl font-semibold mt-1 capitalize">{user?.role}</p>
        </div>
        <div className="bg-white p-6 rounded-lg shadow">
          <h2 className="text-sm text-gray-500">Email</h2>
          <p className="text-xl font-semibold mt-1">{user?.email}</p>
        </div>
      </div>
    </section>
  );
}

export default function DashboardPage() {
  return (
    <ErrorBoundary>
      <DashboardContent />
    </ErrorBoundary>
  );
}
