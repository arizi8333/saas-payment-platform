'use client';

import { useEffect, useState } from 'react';
import { ErrorBoundary } from '@/components/ErrorBoundary';
import { adminService } from '@/services/admin';
import { DashboardStatsResponse } from '@/types';

function AdminContent() {
  const [stats, setStats] = useState<DashboardStatsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    adminService.getStats()
      .then((res) => setStats(res.data))
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed'))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <p className="text-gray-500">Loading...</p>;
  if (error) return (
    <div role="alert" className="p-4 bg-red-50 text-red-700 rounded">
      {error}
    </div>
  );
  if (!stats) return null;

  return (
    <section aria-labelledby="admin-heading">
      <h1 id="admin-heading" className="text-2xl font-bold mb-6">Admin Dashboard</h1>

      <h2 className="text-lg font-semibold mb-3">Transaction Stats</h2>
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
        <div className="bg-white p-4 rounded-lg shadow">
          <p className="text-sm text-gray-500">Total</p>
          <p className="text-2xl font-bold">{stats.transactions.total}</p>
        </div>
        <div className="bg-white p-4 rounded-lg shadow">
          <p className="text-sm text-gray-500">Pending</p>
          <p className="text-2xl font-bold text-yellow-600">{stats.transactions.pending}</p>
        </div>
        <div className="bg-white p-4 rounded-lg shadow">
          <p className="text-sm text-gray-500">Success</p>
          <p className="text-2xl font-bold text-green-600">{stats.transactions.success}</p>
        </div>
        <div className="bg-white p-4 rounded-lg shadow">
          <p className="text-sm text-gray-500">Failed</p>
          <p className="text-2xl font-bold text-red-600">{stats.transactions.failed}</p>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div className="bg-white p-4 rounded-lg shadow">
          <h2 className="text-lg font-semibold mb-2">Active Users</h2>
          <p className="text-3xl font-bold">{stats.active_users}</p>
        </div>
        <div className="bg-white p-4 rounded-lg shadow">
          <h2 className="text-lg font-semibold mb-2">Webhook Delivery</h2>
          <div className="space-y-1 text-sm">
            <p>Total: {stats.webhook_stats.total_deliveries}</p>
            <p className="text-green-600">Delivered: {stats.webhook_stats.delivered}</p>
            <p className="text-red-600">Failed: {stats.webhook_stats.failed}</p>
            <p className="text-yellow-600">Pending: {stats.webhook_stats.pending}</p>
            <p className="font-medium">Success Rate: {(stats.webhook_stats.success_rate * 100).toFixed(1)}%</p>
          </div>
        </div>
      </div>
    </section>
  );
}

export default function AdminPage() {
  return (
    <ErrorBoundary>
      <AdminContent />
    </ErrorBoundary>
  );
}
