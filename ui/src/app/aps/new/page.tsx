"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";

export default function NewAPPage() {
  const router = useRouter();
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError(null);
    setLoading(true);

    const form = new FormData(e.currentTarget);
    try {
      const ap = await api.createAP({
        name: form.get("name") as string,
        hostname: form.get("hostname") as string,
        ssh_port: parseInt(form.get("ssh_port") as string) || 22,
        username: form.get("username") as string,
        password: form.get("password") as string,
      });
      router.push(`/aps/${ap.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to register AP");
      setLoading(false);
    }
  }

  return (
    <div className="max-w-lg">
      <h1 className="text-2xl font-bold mb-6">Register Access Point</h1>

      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded mb-4">
          {error}
        </div>
      )}

      <form onSubmit={handleSubmit} className="bg-white rounded-lg border border-gray-200 p-6 space-y-4">
        <Field label="Display Name" name="name" placeholder="AP-Office-1" required />
        <Field label="Hostname / IP" name="hostname" placeholder="192.168.1.10" required />
        <Field label="SSH Port" name="ssh_port" placeholder="22" defaultValue="22" type="number" />
        <Field label="Username" name="username" placeholder="admin" required />
        <Field label="Password" name="password" type="password" required />

        <div className="flex gap-3 pt-2">
          <button
            type="submit"
            disabled={loading}
            className="bg-primary text-white px-4 py-2 rounded font-medium hover:bg-primary-light transition disabled:opacity-50"
          >
            {loading ? "Registering…" : "Register AP"}
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

function Field({
  label,
  name,
  placeholder,
  required,
  type = "text",
  defaultValue,
}: {
  label: string;
  name: string;
  placeholder?: string;
  required?: boolean;
  type?: string;
  defaultValue?: string;
}) {
  return (
    <div>
      <label className="block text-sm font-medium text-gray-700 mb-1">
        {label} {required && <span className="text-error">*</span>}
      </label>
      <input
        type={type}
        name={name}
        placeholder={placeholder}
        defaultValue={defaultValue}
        required={required}
        className="w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent"
      />
    </div>
  );
}
