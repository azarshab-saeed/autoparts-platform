import { MOCK_MODE } from "./api";
import type { UserSession } from "./types";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export type VehicleNotebookEntryKind = "service" | "part" | "mileage" | "note";

export type VehicleNotebookVehicle = {
  id: string;
  public_token: string;
  owner_customer_id?: string;
  owner_name?: string;
  owner_phone?: string;
  plate?: string;
  vin?: string;
  make?: string;
  model?: string;
  trim?: string;
  model_year?: number;
  owner_code?: string;
  updated_at: string;
};

export type VehicleNotebookEntry = {
  id: string;
  kind: VehicleNotebookEntryKind;
  title: string;
  mileage?: number;
  occurred_on: string;
  next_due_mileage?: number;
  next_due_date?: string;
  notes?: string;
  actor_role: string;
  actor_name: string;
  owner_reported: boolean;
  created_at: string;
};

export type VehicleNotebookDetail = {
  vehicle: VehicleNotebookVehicle;
  entries: VehicleNotebookEntry[];
};

export type PublicVehicleNotebookDetail = {
  vehicle: {
    public_token: string;
    plate_masked?: string;
    make?: string;
    model?: string;
    trim?: string;
    model_year?: number;
  };
  entries: VehicleNotebookEntry[];
};

export type CreateVehicleNotebookInput = {
  owner_customer_id?: string;
  owner_name?: string;
  owner_phone?: string;
  plate?: string;
  vin?: string;
  make?: string;
  model?: string;
  trim?: string;
  model_year?: number;
};

export type AddVehicleNotebookEntryInput = {
  kind: VehicleNotebookEntryKind;
  title: string;
  mileage?: number;
  occurred_on?: string;
  next_due_mileage?: number;
  next_due_date?: string;
  notes?: string;
};

async function request<T>(path: string, init: RequestInit = {}, token?: string): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body) headers.set("Content-Type", "application/json");
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const res = await fetch(`${API_URL}${path}`, { ...init, headers });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(body?.error?.message || `HTTP ${res.status}`);
  return body as T;
}

const mockToken = "11111111-aaaa-4444-bbbb-111111111111";
const mockVehicle: VehicleNotebookVehicle = {
  id: "11111111-1111-4444-8888-111111111111",
  public_token: mockToken,
  owner_name: "علی احمدی",
  owner_phone: "09121234567",
  plate: "12الف345 ایران 67",
  make: "پژو",
  model: "206",
  trim: "تیپ 5",
  model_year: 1399,
  updated_at: new Date().toISOString(),
};
let mockOwnerCode = "246810";
let mockEntries: VehicleNotebookEntry[] = [
  {
    id: cryptoRandom(),
    kind: "service",
    title: "تعویض تسمه تایم",
    mileage: 87400,
    occurred_on: new Date(Date.now() - 35 * 86400000).toISOString(),
    next_due_mileage: 147400,
    actor_role: "mechanic",
    actor_name: "تعمیرگاه نمونه",
    owner_reported: false,
    created_at: new Date(Date.now() - 35 * 86400000).toISOString(),
  },
];

function cryptoRandom() {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) return crypto.randomUUID();
  return Math.random().toString(36).slice(2);
}

function publicMock(): PublicVehicleNotebookDetail {
  return {
    vehicle: {
      public_token: mockVehicle.public_token,
      plate_masked: "12•••67",
      make: mockVehicle.make,
      model: mockVehicle.model,
      trim: mockVehicle.trim,
      model_year: mockVehicle.model_year,
    },
    entries: mockEntries.map(x => ({ ...x })),
  };
}

export async function listVehicleNotebooks(session: UserSession, q = ""): Promise<VehicleNotebookVehicle[]> {
  if (MOCK_MODE) {
    const needle = q.trim().toLowerCase();
    return !needle || [mockVehicle.plate, mockVehicle.owner_name, mockVehicle.owner_phone, mockVehicle.make, mockVehicle.model].filter(Boolean).some(v => String(v).toLowerCase().includes(needle)) ? [{ ...mockVehicle }] : [];
  }
  const out = await request<{ items: VehicleNotebookVehicle[] }>(`/v1/vehicle-notebook?q=${encodeURIComponent(q)}&limit=60`, {}, session.token);
  return out.items;
}

export async function createVehicleNotebook(session: UserSession, input: CreateVehicleNotebookInput): Promise<VehicleNotebookVehicle> {
  if (MOCK_MODE) {
    Object.assign(mockVehicle, input, { updated_at: new Date().toISOString(), owner_code: mockOwnerCode });
    return { ...mockVehicle, owner_code: mockOwnerCode };
  }
  return request<VehicleNotebookVehicle>("/v1/vehicle-notebook", { method: "POST", body: JSON.stringify(input) }, session.token);
}

export async function getVehicleNotebookByToken(session: UserSession, token: string): Promise<VehicleNotebookDetail> {
  if (MOCK_MODE) return { vehicle: { ...mockVehicle }, entries: mockEntries.map(x => ({ ...x })) };
  return request<VehicleNotebookDetail>(`/v1/vehicle-notebook/by-token/${encodeURIComponent(token)}`, {}, session.token);
}

export async function addVehicleNotebookEntry(session: UserSession, token: string, input: AddVehicleNotebookEntryInput): Promise<VehicleNotebookEntry> {
  if (MOCK_MODE) {
    const row: VehicleNotebookEntry = {
      id: cryptoRandom(),
      kind: input.kind,
      title: input.title,
      mileage: input.mileage,
      occurred_on: input.occurred_on || new Date().toISOString(),
      next_due_mileage: input.next_due_mileage,
      next_due_date: input.next_due_date,
      notes: input.notes,
      actor_role: session.role,
      actor_name: session.displayName,
      owner_reported: false,
      created_at: new Date().toISOString(),
    };
    mockEntries = [row, ...mockEntries];
    return row;
  }
  return request<VehicleNotebookEntry>(`/v1/vehicle-notebook/by-token/${encodeURIComponent(token)}/entries`, { method: "POST", body: JSON.stringify(input) }, session.token);
}

export async function rotateVehicleOwnerCode(session: UserSession, vehicleId: string): Promise<string> {
  if (MOCK_MODE) {
    mockOwnerCode = String(Math.floor(100000 + Math.random() * 900000));
    return mockOwnerCode;
  }
  const out = await request<{ owner_code: string }>(`/v1/vehicle-notebook/${encodeURIComponent(vehicleId)}/owner-code`, { method: "POST" }, session.token);
  return out.owner_code;
}

export async function getPublicVehicleNotebook(token: string): Promise<PublicVehicleNotebookDetail> {
  if (MOCK_MODE) return publicMock();
  return request<PublicVehicleNotebookDetail>(`/v1/public/vehicle-notebook/${encodeURIComponent(token)}`);
}

export async function addOwnerMileage(token: string, ownerCode: string, mileage: number, occurredOn?: string): Promise<VehicleNotebookEntry> {
  if (MOCK_MODE) {
    if (ownerCode !== mockOwnerCode) throw new Error("کد مالک اشتباه است.");
    const row: VehicleNotebookEntry = {
      id: cryptoRandom(),
      kind: "mileage",
      title: "ثبت کیلومتر توسط مالک",
      mileage,
      occurred_on: occurredOn || new Date().toISOString(),
      actor_role: "owner",
      actor_name: "مالک خودرو",
      owner_reported: true,
      created_at: new Date().toISOString(),
    };
    mockEntries = [row, ...mockEntries];
    return row;
  }
  return request<VehicleNotebookEntry>(`/v1/public/vehicle-notebook/${encodeURIComponent(token)}/mileage`, { method: "POST", body: JSON.stringify({ owner_code: ownerCode, mileage, occurred_on: occurredOn }) });
}
