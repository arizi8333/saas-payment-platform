'use client';

import { useEffect, useState, useCallback } from 'react';
import { ErrorBoundary } from '@/components/ErrorBoundary';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { subscriptionsService } from '@/services/subscriptions';
import { ProductResponse, SubscriptionResponse } from '@/types';

function SubscriptionsContent() {
  const [products, setProducts] = useState<ProductResponse[]>([]);
  const [subs, setSubs] = useState<SubscriptionResponse[]>([]);
  const [productName, setProductName] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [cancelId, setCancelId] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const [pRes, sRes] = await Promise.all([
        subscriptionsService.listProducts(),
        subscriptionsService.listSubscriptions(),
      ]);
      setProducts(pRes.data.products || []);
      setSubs(sRes.data.subscriptions || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleCreateProduct = async () => {
    if (!productName.trim()) return;
    try {
      await subscriptionsService.createProduct({ name: productName });
      setProductName('');
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed');
    }
  };

  const handleCancel = async () => {
    if (!cancelId) return;
    try {
      await subscriptionsService.cancel(cancelId);
      setCancelId(null);
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed');
    }
  };

  const statusColor: Record<string, string> = {
    active: 'bg-green-100 text-green-700',
    pending_payment: 'bg-yellow-100 text-yellow-700',
    cancelled: 'bg-red-100 text-red-700',
    expired: 'bg-gray-100 text-gray-500',
  };

  if (loading) return <p className="text-gray-500">Loading...</p>;

  return (
    <section aria-labelledby="sub-heading">
      <h1 id="sub-heading" className="text-2xl font-bold mb-6">Subscriptions</h1>

      {error && <div role="alert" className="mb-4 p-3 bg-red-50 text-red-700 rounded text-sm">{error}</div>}

      <h2 className="text-lg font-semibold mb-3">Products</h2>
      <div className="flex gap-2 mb-4">
        <label htmlFor="prodName" className="sr-only">Product name</label>
        <input id="prodName" type="text" placeholder="Product name" value={productName}
          onChange={(e) => setProductName(e.target.value)}
          className="flex-1 px-3 py-2 border border-gray-300 rounded text-sm" />
        <button onClick={handleCreateProduct}
          className="px-4 py-2 bg-blue-600 text-white rounded text-sm hover:bg-blue-700">Create Product</button>
      </div>
      <div className="grid grid-cols-1 md:grid-cols-3 gap-3 mb-8">
        {products.map((p) => (
          <div key={p.id} className="bg-white p-4 rounded-lg shadow">
            <p className="font-medium">{p.name}</p>
            <p className="text-xs text-gray-500 mt-1">{p.description || 'No description'}</p>
          </div>
        ))}
      </div>

      <h2 className="text-lg font-semibold mb-3">Active Subscriptions</h2>
      <div className="bg-white rounded-lg shadow overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-gray-50">
            <tr>
              <th scope="col" className="px-4 py-3 text-left text-gray-600">Plan</th>
              <th scope="col" className="px-4 py-3 text-left text-gray-600">Status</th>
              <th scope="col" className="px-4 py-3 text-left text-gray-600">Period End</th>
              <th scope="col" className="px-4 py-3 text-right text-gray-600">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {subs.map((s) => (
              <tr key={s.id}>
                <td className="px-4 py-3 font-mono text-xs">{s.plan_id}</td>
                <td className="px-4 py-3">
                  <span className={`px-2 py-1 rounded text-xs ${statusColor[s.status] || 'bg-gray-100'}`}>
                    {s.status}
                  </span>
                </td>
                <td className="px-4 py-3 text-gray-500">{new Date(s.current_period_end).toLocaleDateString()}</td>
                <td className="px-4 py-3 text-right">
                  {s.status === 'active' && (
                    <button onClick={() => setCancelId(s.id)}
                      className="text-red-600 hover:underline text-xs">Cancel</button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <ConfirmDialog
        open={!!cancelId}
        title="Cancel Subscription"
        message="Are you sure you want to cancel this subscription?"
        confirmLabel="Cancel Subscription"
        onConfirm={handleCancel}
        onCancel={() => setCancelId(null)}
      />
    </section>
  );
}

export default function SubscriptionsPage() {
  return (
    <ErrorBoundary>
      <SubscriptionsContent />
    </ErrorBoundary>
  );
}
