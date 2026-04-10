'use client';

import { useEffect, useState, useCallback } from 'react';
import { ErrorBoundary } from '@/components/ErrorBoundary';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { apiKeysService } from '@/services/apikeys';
import { APIKeyResponse } from '@/types';

function APIKeysContent() {
  const [keys, setKeys] = useState<APIKeyResponse[]>([]);
  const [newKeyName, setNewKeyName] = useState('');
  const [createdKey, setCreatedKey] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [revokeId, setRevokeId] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);

  const loadKeys = useCallback(async () => {
    try {
      const res = await apiKeysService.list(page);
      setKeys(res.data.keys || []);
      setTotalPages(res.data.pagination.total_pages);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load');
    } finally {
      setLoading(false);
    }
  }, [page]);

  useEffect(() => { loadKeys(); }, [loadKeys]);

  const handleCreate = async () => {
    if (!newKeyName.trim()) return;
    try {
      const res = await apiKeysService.create({ name: newKeyName });
      setCreatedKey(res.data.full_key);
      setNewKeyName('');
      loadKeys();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create');
    }
  };

  const handleRevoke = async () => {
    if (!revokeId) return;
    try {
      await apiKeysService.revoke(revokeId);
      setRevokeId(null);
      loadKeys();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed');
    }
  };

  if (loading) return <p className="text-gray-500">Loading...</p>;

  return (
    <section aria-labelledby="apikeys-heading">
      <h1 id="apikeys-heading" className="text-2xl font-bold mb-6">API Keys</h1>

      {error && <div role="alert" className="mb-4 p-3 bg-red-50 text-red-700 rounded text-sm">{error}</div>}

      {createdKey && (
        <div role="alert" className="mb-4 p-3 bg-green-50 border border-green-200 rounded text-sm">
          <p className="font-medium text-green-800">API Key created! Copy it now — it won&apos;t be shown again:</p>
          <code className="block mt-1 p-2 bg-white rounded text-xs break-all">{createdKey}</code>
          <button onClick={() => setCreatedKey('')} className="mt-2 text-xs text-green-700 underline">Dismiss</button>
        </div>
      )}

      <div className="flex gap-2 mb-6">
        <label htmlFor="keyName" className="sr-only">Key name</label>
        <input id="keyName" type="text" placeholder="Key name" value={newKeyName}
          onChange={(e) => setNewKeyName(e.target.value)}
          className="flex-1 px-3 py-2 border border-gray-300 rounded text-sm" />
        <button onClick={handleCreate}
          className="px-4 py-2 bg-blue-600 text-white rounded text-sm hover:bg-blue-700">Create Key</button>
      </div>

      <div className="bg-white rounded-lg shadow overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-gray-50">
            <tr>
              <th scope="col" className="px-4 py-3 text-left text-gray-600">Name</th>
              <th scope="col" className="px-4 py-3 text-left text-gray-600">Prefix</th>
              <th scope="col" className="px-4 py-3 text-left text-gray-600">Status</th>
              <th scope="col" className="px-4 py-3 text-left text-gray-600">Created</th>
              <th scope="col" className="px-4 py-3 text-right text-gray-600">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {keys.map((k) => (
              <tr key={k.id}>
                <td className="px-4 py-3">{k.name}</td>
                <td className="px-4 py-3 font-mono text-xs">{k.key_prefix}</td>
                <td className="px-4 py-3">
                  <span className={`px-2 py-1 rounded text-xs ${k.is_active ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'}`}>
                    {k.is_active ? 'Active' : 'Revoked'}
                  </span>
                </td>
                <td className="px-4 py-3 text-gray-500">{new Date(k.created_at).toLocaleDateString()}</td>
                <td className="px-4 py-3 text-right">
                  {k.is_active && (
                    <button onClick={() => setRevokeId(k.id)}
                      className="text-red-600 hover:underline text-xs">Revoke</button>
                  )}
                </td>
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

      <ConfirmDialog
        open={!!revokeId}
        title="Revoke API Key"
        message="This action cannot be undone. The API key will stop working immediately."
        confirmLabel="Revoke"
        onConfirm={handleRevoke}
        onCancel={() => setRevokeId(null)}
      />
    </section>
  );
}

export default function APIKeysPage() {
  return (
    <ErrorBoundary>
      <APIKeysContent />
    </ErrorBoundary>
  );
}
