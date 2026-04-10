'use client';

import { useEffect, useState, useCallback } from 'react';
import { ErrorBoundary } from '@/components/ErrorBoundary';
import { transactionsService } from '@/services/transactions';
import { TransactionResponse } from '@/types';

const statusColor: Record<string, string> = {
  pending: 'bg-yellow-100 text-yellow-700',
  success: 'bg-green-100 text-green-700',
  failed: 'bg-red-100 text-red-700',
  expired: 'bg-gray-100 text-gray-500',
};

function TransactionsContent() {
  const [txs, setTxs] = useState<TransactionResponse[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);

  const load = useCallback(async () => {
    try {
      const res = await transactionsService.list(page);
      setTxs(res.data.transactions || []);
      setTotalPages(res.data.pagination.total_pages);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed');
    } finally {
      setLoading(false);
    }
  }, [page]);

  useEffect(() => { load(); }, [load]);

  if (loading) return <p className="text-gray-500">Loading...</p>;
  if (error) return <p className="text-red-600">{error}</p>;

  return (
    <section aria-labelledby="tx-heading">
      <h1 id="tx-heading" className="text-2xl font-bold mb-6">Transactions</h1>
      <div className="bg-white rounded-lg shadow overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-gray-50">
            <tr>
              <th scope="col" className="px-4 py-3 text-left text-gray-600">External ID</th>
              <th scope="col" className="px-4 py-3 text-left text-gray-600">Amount</th>
              <th scope="col" className="px-4 py-3 text-left text-gray-600">Method</th>
              <th scope="col" className="px-4 py-3 text-left text-gray-600">Status</th>
              <th scope="col" className="px-4 py-3 text-left text-gray-600">Date</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {txs.map((tx) => (
              <tr key={tx.id}>
                <td className="px-4 py-3 font-mono text-xs">{tx.external_id}</td>
                <td className="px-4 py-3">{(tx.amount / 100).toLocaleString()} {tx.currency}</td>
                <td className="px-4 py-3 capitalize">{tx.payment_method.replace('_', ' ')}</td>
                <td className="px-4 py-3">
                  <span className={`px-2 py-1 rounded text-xs ${statusColor[tx.status] || 'bg-gray-100'}`}>
                    {tx.status}
                  </span>
                </td>
                <td className="px-4 py-3 text-gray-500">{new Date(tx.created_at).toLocaleDateString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {totalPages > 1 && (
        <div className="flex justify-center gap-2 mt-4">
          <button disabled={page <= 1} onClick={() => setPage(page - 1)}
            className="px-3 py-1 border rounded text-sm disabled:opacity-50">Previous</button>
          <span className="px-3 py-1 text-sm">Page {page} of {totalPages}</span>
          <button disabled={page >= totalPages} onClick={() => setPage(page + 1)}
            className="px-3 py-1 border rounded text-sm disabled:opacity-50">Next</button>
        </div>
      )}
    </section>
  );
}

export default function TransactionsPage() {
  return (
    <ErrorBoundary>
      <TransactionsContent />
    </ErrorBoundary>
  );
}
