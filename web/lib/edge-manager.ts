export const STORE_EDGE_MANAGER_URL = process.env.NEXT_PUBLIC_STORE_EDGE_MANAGER_URL || "http://127.0.0.1:17623";

export type EdgeWorkerLifecycle = {
  state: "running" | "starting" | "stopped" | string;
  pid?: number;
  version?: string;
  started_at?: string;
  last_exit_at?: string;
  last_exit_error?: string;
  healthy: boolean;
};

export type EdgeLifecycleStatus = {
  manager_version: string;
  os: "windows" | "linux" | string;
  arch: string;
  service_mode: "windows" | "systemd-user" | "none" | string;
  worker: EdgeWorkerLifecycle;
  update_enabled: boolean;
  update_state: "idle" | "checking" | "downloading" | "applying" | "error" | string;
  latest_version?: string;
  update_available: boolean;
  last_update_error?: string;
  manifest_url?: string;
};

export type EdgeUpdateCheck = {
  current_version: string;
  latest_version: string;
  update_available: boolean;
  release_notes?: string;
  published_at?: string;
};

async function managerRequest<T>(path: string, init: RequestInit = {}, timeoutMs = 5000): Promise<T> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const headers = new Headers(init.headers);
    headers.set("X-AutoParts-Edge", "1");
    if (init.body) headers.set("Content-Type", "application/json");
    const res = await fetch(`${STORE_EDGE_MANAGER_URL}${path}`, { ...init, headers, signal: controller.signal });
    const body = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(body?.error?.message || `Store Edge Manager HTTP ${res.status}`);
    return body as T;
  } finally {
    clearTimeout(timer);
  }
}

export async function getEdgeLifecycleStatus(): Promise<EdgeLifecycleStatus> {
  return managerRequest<EdgeLifecycleStatus>("/v1/lifecycle/status", {}, 1400);
}
export async function startStoreEdge(): Promise<void> {
  await managerRequest("/v1/lifecycle/start", { method: "POST", body: "{}" }, 12000);
}
export async function stopStoreEdge(): Promise<void> {
  await managerRequest("/v1/lifecycle/stop", { method: "POST", body: "{}" }, 16000);
}
export async function restartStoreEdge(): Promise<void> {
  await managerRequest("/v1/lifecycle/restart", { method: "POST", body: "{}" }, 18000);
}
export async function checkStoreEdgeUpdate(): Promise<EdgeUpdateCheck> {
  return managerRequest<EdgeUpdateCheck>("/v1/lifecycle/update/check", { method: "POST", body: "{}" }, 25000);
}
export async function applyStoreEdgeUpdate(): Promise<{ status: string; version: string }> {
  return managerRequest<{ status: string; version: string }>("/v1/lifecycle/update/apply", { method: "POST", body: "{}" }, 125000);
}
