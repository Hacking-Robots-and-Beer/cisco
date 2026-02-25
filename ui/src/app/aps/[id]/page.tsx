import Link from "next/link";
import { notFound } from "next/navigation";
import { api } from "@/lib/api";
import StatusBadge from "@/components/StatusBadge";
import SyncButton from "./SyncButton";

export const dynamic = "force-dynamic";

interface Props {
  params: Promise<{ id: string }>;
}

export default async function APDetailPage({ params }: Props) {
  const { id } = await params;

  let ap;
  try {
    ap = await api.getAP(id);
  } catch {
    notFound();
  }
  if (!ap) notFound();

  const [ssids, radios, clients] = await Promise.all([
    api.listSSIDs(id).catch(() => []),
    api.listRadioConfigs(id).catch(() => []),
    api.listClients(id).catch(() => []),
  ]);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold">{ap.name}</h1>
            <StatusBadge status={ap.status as "unknown" | "online" | "offline" | "syncing" | "error"} />
          </div>
          <p className="text-sm text-gray-500 mt-1">
            {ap.hostname}:{ap.ssh_port} &nbsp;·&nbsp; {ap.username}
            {ap.model && <> &nbsp;·&nbsp; {ap.model}</>}
            {ap.firmware_version && (
              <span className="ml-2 text-xs bg-gray-100 px-2 py-0.5 rounded">
                {ap.firmware_version}
              </span>
            )}
          </p>
          {ap.sync_error && (
            <p className="text-sm text-error mt-1">{ap.sync_error}</p>
          )}
        </div>
        <div className="flex gap-2">
          <SyncButton apId={id} />
          <Link
            href="/"
            className="px-3 py-1.5 text-sm rounded border border-gray-300 hover:bg-gray-50 transition"
          >
            ← Back
          </Link>
        </div>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        {/* SSIDs */}
        <section className="bg-white rounded-lg border border-gray-200 p-5">
          <div className="flex items-center justify-between mb-4">
            <h2 className="font-semibold text-lg">SSIDs</h2>
            <Link
              href={`/aps/${id}/ssids/new`}
              className="text-sm bg-primary text-white px-3 py-1 rounded hover:bg-primary-light transition"
            >
              + Add SSID
            </Link>
          </div>
          {ssids.length === 0 ? (
            <p className="text-sm text-gray-400">No SSIDs configured.</p>
          ) : (
            <ul className="space-y-3">
              {ssids.map((s) => (
                <li key={s.id} className="flex items-start justify-between text-sm">
                  <div>
                    <span className="font-medium">{s.name}</span>
                    <span className="ml-2 text-xs text-gray-500">
                      {s.security} · {s.radio} · VLAN {s.vlan}
                    </span>
                  </div>
                  <span
                    className={`text-xs px-2 py-0.5 rounded-full ${
                      s.enabled
                        ? "bg-green-100 text-green-700"
                        : "bg-gray-100 text-gray-500"
                    }`}
                  >
                    {s.enabled ? "enabled" : "disabled"}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </section>

        {/* Radios */}
        <section className="bg-white rounded-lg border border-gray-200 p-5">
          <h2 className="font-semibold text-lg mb-4">Radio Configuration</h2>
          {radios.length === 0 ? (
            <p className="text-sm text-gray-400">No radio config overrides (using AP defaults).</p>
          ) : (
            <ul className="space-y-3">
              {radios.map((r) => (
                <li key={r.id} className="text-sm">
                  <div className="flex items-center justify-between">
                    <span className="font-medium">{r.band}</span>
                    <span
                      className={`text-xs px-2 py-0.5 rounded-full ${
                        r.enabled
                          ? "bg-green-100 text-green-700"
                          : "bg-gray-100 text-gray-500"
                      }`}
                    >
                      {r.enabled ? "enabled" : "disabled"}
                    </span>
                  </div>
                  <p className="text-gray-500 mt-0.5">
                    Channel: {r.channel === 0 ? "auto" : r.channel} &nbsp;·&nbsp;
                    Tx Power: {r.tx_power_dbm === 0 ? "auto" : `${r.tx_power_dbm} dBm`}
                  </p>
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>

      {/* Connected Clients */}
      <section className="bg-white rounded-lg border border-gray-200 p-5">
        <h2 className="font-semibold text-lg mb-4">
          Connected Clients
          <span className="ml-2 text-sm font-normal text-gray-500">
            ({clients.length})
          </span>
        </h2>
        {clients.length === 0 ? (
          <p className="text-sm text-gray-400">No clients currently associated.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-gray-100 text-left">
                  <th className="pb-2 font-medium text-gray-600">MAC Address</th>
                  <th className="pb-2 font-medium text-gray-600">IP Address</th>
                  <th className="pb-2 font-medium text-gray-600">SSID</th>
                  <th className="pb-2 font-medium text-gray-600">Radio</th>
                  <th className="pb-2 font-medium text-gray-600">Signal</th>
                  <th className="pb-2 font-medium text-gray-600">Last Seen</th>
                </tr>
              </thead>
              <tbody>
                {clients.map((c) => (
                  <tr key={c.id} className="border-b border-gray-50 hover:bg-gray-50">
                    <td className="py-2 font-mono">{c.mac_address}</td>
                    <td className="py-2 text-gray-600">{c.ip_address ?? "—"}</td>
                    <td className="py-2">{c.ssid ?? "—"}</td>
                    <td className="py-2 text-gray-600">{c.radio ?? "—"}</td>
                    <td className="py-2">
                      {c.signal_dbm != null ? (
                        <span className={signalColour(c.signal_dbm)}>
                          {c.signal_dbm} dBm
                        </span>
                      ) : (
                        "—"
                      )}
                    </td>
                    <td className="py-2 text-gray-400 text-xs">
                      {new Date(c.seen_at).toLocaleTimeString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {/* Timestamps */}
      <p className="text-xs text-gray-400">
        Last sync: {ap.last_sync_at ? new Date(ap.last_sync_at).toLocaleString() : "never"} &nbsp;·&nbsp;
        Last seen: {ap.last_seen_at ? new Date(ap.last_seen_at).toLocaleString() : "never"}
      </p>
    </div>
  );
}

function signalColour(dbm: number) {
  if (dbm >= -60) return "text-green-600";
  if (dbm >= -75) return "text-yellow-600";
  return "text-red-600";
}
