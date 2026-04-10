'use client';

import { useEffect, useState, useCallback } from 'react';
import { ErrorBoundary } from '@/components/ErrorBoundary';
import { invoicesService } from '@/services/invoices';
import { InvoiceResponse } from '@/types';

const statusColor: Record<string, string> = {
  unpaid: 'bg-yellow-100 text-yellow-700',
  paid: 'bg-green-100 text-green-700',
  void: 'bg-gray-100 text-gray-500',
};

function InvoicesContent() {
  const [invoices, setInvoices] = useState<InvoiceResponse[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);

  const load = useCallback(async () => {
    try {
      const res = await invoicesService.list(page);
      setInvoices(res.data.invoices || []);
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
    <section aria-labelledby="inv-heading">
      <h1 id="inv-heading" className="text-2xl font-bold mb-6">Invoices</h1>
      <div className="bg-white rounded-lg shadow overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-gray-50">
            <tr>
              <th scope="col" className="px-4 py-3 text-left text-gray-600">Invoice #</th>
              <th scope="col" className="px-4 py-3 text-left text-gray-600">Amount</th>
              <th scope="col" className="px-4 py-3 text-left text-gray-600">Status</th>
              <th scope="col" className="px-4 py-3 text-left text-gray-600">Due Date</th>
              <th scope="col" className="px-4 py-3 text-left text-gray-600">Paid At</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {invoices.map((inv) => (
              <tr key={inv.id}>
                <td className="px-4 py-3 font-mono text-xs">{inv.invoice_number}</td>
                <td className="px-4 py-3">{(inv.amount / 100).toLocaleString()} {inv.currency}</td>
                <td className="px-4 py-3">
                  <span className={`px-2 py-1 rounded text-xs ${statusColor[inv.status] || 'bg-gray-100'}`}>
                    {inv.status}
                  </span>
                </td>
                <td className="px-4 py-3 text-gray-500">{new Date(inv.due_date).toLocaleDateString()}</td>
                <td className="px-4 py-3 text-gray-500">{inv.paid_at ? new Date(inv.paid_at).toLocaleDateString() : '—'}</td>
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

export default function InvoicesPage() {
  return (
    <ErrorBoundary>
      <InvoicesContent />
    </ErrorBoundary>
  );
}
