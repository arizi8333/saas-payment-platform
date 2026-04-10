'use client';

import { useEffect, useState, useCallback } from 'react';
import { ErrorBoundary } from '@/components/ErrorBoundary';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { webhooksService } from '@/services/webhooks';
import { WebhookResponse } from '@/types';

function WebhooksContent() {
  const [endpoints, setEndpoints] = useState<WebhookResponse[]>([]);
  const [url, setUrl] = useState('');
  const [events, setEvents] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [deleteId, setDeleteId] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const res = await webhooksService.list();
      setEndpoints(res.data.endpoints || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleCreate = async () => {
    if (!url.trim() || !events.trim()) return;
    try {
      await webhooksService.create({
        url,
        events: events.split(',').map((e) => e.trim()),
      });
      setUrl('');
      setEvents('');
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed');
    }
  };

  const handleDelete = async () => {
    if (!deleteId) return;
    try {
      await webhooksService.remove(deleteId);
      setDeleteId(null);
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed');
    }
  };

  if (loading) return <p className="text-gray-500">Loading...</p>;

  return (
    <section aria-labelledby="wh-heading">
      <h1 id="wh-heading" className="text-2xl font-bold mb-6">Webhook Endpoints</h1>

      {error && <div role="alert" className="mb-4 p-3 bg-red-50 text-red-700 rounded text-sm">{error}</div>}

      <div className="flex gap-2 mb-6">
        <label htmlFor="whUrl" className="sr-only">Webhook URL</label>
        <input id="whUrl" type="url" placeholder="https://example.com/webhook" value={url}
          onChange={(e) => setUrl(e.target.value)}
          className="flex-1 px-3 py-2 border border-gray-300 rounded text-sm" />
        <label htmlFor="whEvents" className="sr-only">Events</label>
        <input id="whEvents" type="text" placeholder="transaction.success,transaction.failed" value={events}
          onChange={(e) => setEvents(e.target.value)}
          className="flex-1 px-3 py-2 border border-gray-300 rounded text-sm" />
        <button onClick={handleCreate}
          className="px-4 py-2 bg-blue-600 text-white rounded text-sm hover:bg-blue-700">Add</button>
      </div>

      <div className="space-y-3">
        {endpoints.map((ep) => (
          <div key={ep.id} className="bg-white p-4 rounded-lg shadow flex justify-between items-center">
            <div>
              <p className="font-mono text-sm">{ep.url}</p>
              <p className="text-xs text-gray-500 mt-1">Events: {ep.events.join(', ')}</p>
            </div>
            <button onClick={() => setDeleteId(ep.id)}
              className="text-red-600 hover:underline text-xs">Delete</button>
          </div>
        ))}
        {endpoints.length === 0 && <p className="text-gray-400 text-sm">No webhook endpoints configured.</p>}
      </div>

      <ConfirmDialog
        open={!!deleteId}
        title="Delete Webhook"
        message="This will permanently remove the webhook endpoint."
        confirmLabel="Delete"
        onConfirm={handleDelete}
        onCancel={() => setDeleteId(null)}
      />
    </section>
  );
}

export default function WebhooksPage() {
  return (
    <ErrorBoundary>
      <WebhooksContent />
    </ErrorBoundary>
  );
}
