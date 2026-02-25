import Link from "next/link";
import { api, AP } from "@/lib/api";
import StatusBadge from "@/components/StatusBadge";

export const dynamic = "force-dynamic";

export default async function DashboardPage() {
  let aps: AP[] = [];
  let error: string | null = null;

  try {
    aps = await api.listAPs();
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load access points";
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-foreground">Access Points</h1>
          <p className="text-sm text-gray-500 mt-1">
            {aps.length} AP{aps.length !== 1 ? "s" : ""} registered
          </p>
        </div>
        <Link
          href="/aps/new"
          className="bg-primary text-white px-4 py-2 rounded font-medium hover:bg-primary-light transition"
        >
          + Register AP
        </Link>
      </div>

      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded mb-6">
          {error}
        </div>
      )}

      {aps.length === 0 && !error ? (
        <div className="text-center py-16 text-gray-400">
          <p className="text-lg">No access points registered yet.</p>
          <Link href="/aps/new" className="text-primary hover:underline mt-2 inline-block">
            Register your first AP →
          </Link>
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {aps.map((ap) => (
            <APCard key={ap.id} ap={ap} />
          ))}
        </div>
      )}
    </div>
  );
}

async function APCard({ ap }: { ap: AP }) {
  let clientCount = 0;
  try {
    const clients = await api.listClients(ap.id);
    clientCount = clients.length;
  } catch {
    // ignore
  }

  return (
    <Link
      href={`/aps/${ap.id}`}
      className="block bg-white rounded-lg border border-gray-200 p-5 hover:border-primary hover:shadow-sm transition"
    >
      <div className="flex items-start justify-between">
        <div>
          <h2 className="font-semibold text-foreground">{ap.name}</h2>
          <p className="text-sm text-gray-500">{ap.hostname}:{ap.ssh_port}</p>
        </div>
        <StatusBadge status={ap.status as "unknown" | "online" | "offline" | "syncing" | "error"} />
      </div>

      <div className="mt-3 flex items-center gap-4 text-sm text-gray-500">
        <span>
          <span className="font-medium text-foreground">{clientCount}</span>{" "}
          client{clientCount !== 1 ? "s" : ""}
        </span>
        {ap.model && <span>{ap.model}</span>}
        {ap.firmware_version && (
          <span className="text-xs bg-gray-100 px-2 py-0.5 rounded">
            {ap.firmware_version}
          </span>
        )}
      </div>

      {ap.last_seen_at && (
        <p className="mt-2 text-xs text-gray-400">
          Last seen: {new Date(ap.last_seen_at).toLocaleString()}
        </p>
      )}
      {ap.sync_error && (
        <p className="mt-2 text-xs text-error truncate">{ap.sync_error}</p>
      )}
    </Link>
  );
}
