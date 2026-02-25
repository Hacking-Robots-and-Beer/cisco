const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export interface AP {
  id: string;
  name: string;
  hostname: string;
  ssh_port: number;
  username: string;
  model?: string;
  firmware_version?: string;
  status: "unknown" | "online" | "offline" | "syncing" | "error";
  last_seen_at?: string;
  last_sync_at?: string;
  sync_error?: string;
  created_at: string;
  updated_at: string;
}

export interface SSID {
  id: string;
  ap_id: string;
  name: string;
  vlan: number;
  radio: "2.4ghz" | "5ghz" | "both";
  security: "open" | "wpa2-psk";
  password?: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface RadioConfig {
  id: string;
  ap_id: string;
  band: "2.4ghz" | "5ghz";
  channel: number;
  tx_power_dbm: number;
  enabled: boolean;
  updated_at: string;
}

export interface Client {
  id: number;
  ap_id: string;
  mac_address: string;
  ip_address?: string;
  ssid?: string;
  radio?: string;
  signal_dbm?: number;
  seen_at: string;
}

export interface CreateAPRequest {
  name: string;
  hostname: string;
  ssh_port?: number;
  username: string;
  password: string;
}

export interface CreateSSIDRequest {
  name: string;
  vlan?: number;
  radio?: string;
  security?: string;
  password?: string;
  enabled?: boolean;
}

export interface UpsertRadioRequest {
  channel?: number;
  tx_power_dbm?: number;
  enabled?: boolean;
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });
  if (!res.ok) {
    const body = await res.text().catch(() => "");
    throw new Error(`API error ${res.status}: ${body}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

export const api = {
  // APs
  listAPs: () => request<AP[]>("/api/v1/aps"),
  getAP: (id: string) => request<AP>(`/api/v1/aps/${id}`),
  createAP: (body: CreateAPRequest) =>
    request<AP>("/api/v1/aps", { method: "POST", body: JSON.stringify(body) }),
  updateAP: (id: string, body: Partial<CreateAPRequest>) =>
    request<AP>(`/api/v1/aps/${id}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteAP: (id: string) =>
    request<void>(`/api/v1/aps/${id}`, { method: "DELETE" }),
  syncAP: (id: string) =>
    request<{ message: string }>(`/api/v1/aps/${id}/sync`, { method: "POST" }),

  // SSIDs
  listSSIDs: (apId: string) => request<SSID[]>(`/api/v1/aps/${apId}/ssids`),
  createSSID: (apId: string, body: CreateSSIDRequest) =>
    request<SSID>(`/api/v1/aps/${apId}/ssids`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  updateSSID: (apId: string, ssidId: string, body: Partial<CreateSSIDRequest>) =>
    request<SSID>(`/api/v1/aps/${apId}/ssids/${ssidId}`, {
      method: "PUT",
      body: JSON.stringify(body),
    }),
  deleteSSID: (apId: string, ssidId: string) =>
    request<void>(`/api/v1/aps/${apId}/ssids/${ssidId}`, { method: "DELETE" }),

  // Radios
  listRadioConfigs: (apId: string) =>
    request<RadioConfig[]>(`/api/v1/aps/${apId}/radios`),
  upsertRadioConfig: (apId: string, band: string, body: UpsertRadioRequest) =>
    request<RadioConfig>(`/api/v1/aps/${apId}/radios/${band}`, {
      method: "PUT",
      body: JSON.stringify(body),
    }),

  // Clients
  listClients: (apId: string) =>
    request<Client[]>(`/api/v1/aps/${apId}/clients`),
};
