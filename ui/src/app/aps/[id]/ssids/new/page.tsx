"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { use } from "react";

interface Props {
  params: Promise<{ id: string }>;
}

export default function NewSSIDPage({ params }: Props) {
  const { id } = use(params);
  const router = useRouter();
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [security, setSecurity] = useState("open");

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError(null);
    setLoading(true);

    const form = new FormData(e.currentTarget);
    const password = form.get("password") as string;

    try {
      await api.createSSID(id, {
        name: form.get("name") as string,
        vlan: parseInt(form.get("vlan") as string) || 1,
        radio: form.get("radio") as string,
        security: form.get("security") as string,
        password: password || undefined,
        enabled: true,
      });
      router.push(`/aps/${id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create SSID");
      setLoading(false);
    }
  }

  return (
    <div className="max-w-lg">
      <h1 className="text-2xl font-bold mb-2">Add SSID</h1>
      <p className="text-sm text-gray-500 mb-6">
        Configure a new wireless network. The reconciler will push it to the AP within 30 seconds.
      </p>

      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded mb-4">
          {error}
        </div>
      )}

      <form onSubmit={handleSubmit} className="bg-white rounded-lg border border-gray-200 p-6 space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            SSID Name <span className="text-error">*</span>
          </label>
          <input
            type="text"
            name="name"
            placeholder="MyNetwork"
            required
            maxLength={32}
            className="w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Radio Band</label>
          <select
            name="radio"
            defaultValue="both"
            className="w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
          >
            <option value="both">Both (2.4 GHz + 5 GHz)</option>
            <option value="2.4ghz">2.4 GHz only</option>
            <option value="5ghz">5 GHz only</option>
          </select>
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">VLAN</label>
          <input
            type="number"
            name="vlan"
            defaultValue="1"
            min={1}
            max={4094}
            className="w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Security</label>
          <select
            name="security"
            value={security}
            onChange={(e) => setSecurity(e.target.value)}
            className="w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
          >
            <option value="open">Open (no password)</option>
            <option value="wpa2-psk">WPA2-PSK</option>
          </select>
        </div>

        {security === "wpa2-psk" && (
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              WPA2 Passphrase <span className="text-error">*</span>
            </label>
            <input
              type="password"
              name="password"
              minLength={8}
              maxLength={63}
              placeholder="Minimum 8 characters"
              required={security === "wpa2-psk"}
              className="w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
            />
          </div>
        )}

        <div className="flex gap-3 pt-2">
          <button
            type="submit"
            disabled={loading}
            className="bg-primary text-white px-4 py-2 rounded font-medium hover:bg-primary-light transition disabled:opacity-50"
          >
            {loading ? "Creating…" : "Create SSID"}
          </button>
          <button
            type="button"
            onClick={() => router.back()}
            className="px-4 py-2 rounded font-medium border border-gray-300 hover:bg-gray-50 transition"
          >
            Cancel
          </button>
        </div>
      </form>
    </div>
  );
}
