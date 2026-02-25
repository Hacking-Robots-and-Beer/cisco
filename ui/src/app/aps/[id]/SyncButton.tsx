"use client";

import { useState } from "react";
import { api } from "@/lib/api";

export default function SyncButton({ apId }: { apId: string }) {
  const [syncing, setSyncing] = useState(false);
  const [message, setMessage] = useState<string | null>(null);

  async function handleSync() {
    setSyncing(true);
    setMessage(null);
    try {
      await api.syncAP(apId);
      setMessage("Sync triggered");
      setTimeout(() => setMessage(null), 3000);
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "Failed");
    } finally {
      setSyncing(false);
    }
  }

  return (
    <div className="flex items-center gap-2">
      {message && <span className="text-xs text-gray-500">{message}</span>}
      <button
        onClick={handleSync}
        disabled={syncing}
        className="px-3 py-1.5 text-sm rounded bg-primary text-white hover:bg-primary-light transition disabled:opacity-50"
      >
        {syncing ? "Syncing…" : "Sync Now"}
      </button>
    </div>
  );
}
