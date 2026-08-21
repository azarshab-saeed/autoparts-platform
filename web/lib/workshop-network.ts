import type { UserSession } from "./types";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

async function request<T>(session: UserSession, path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set("Authorization", `Bearer ${session.token}`);
  if (init.body) headers.set("Content-Type", "application/json");
  const res = await fetch(`${API_URL}${path}`, { ...init, headers });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(body?.error?.message || `HTTP ${res.status}`);
  return body as T;
}

export type WorkshopSummary = {
  open_jobs: number; completed_30_days: number; customer_receivable_amount: number;
  revenue_30_days: number; vehicle_count: number; network_part_amount_30_days: number;
};
export type WorkshopItem = { id:string; item_type:"service"|"part"|"labor"; title:string; product_id?:string; source_store_id?:string; source_store_name?:string; reservation_id?:string; qty:number; unit_price:number; line_total:number; notes?:string; created_at:string };
export type WorkshopPayment = { id:string; method:string; amount:number; note?:string; created_at:string };
export type WorkshopJob = {
  id:string; vehicle_id:string; public_token:string; vehicle_label:string; plate?:string; customer_name?:string; customer_phone?:string;
  mileage?:number; complaint?:string; diagnosis?:string; status:"open"|"completed"|"cancelled";
  labor_amount:number; parts_amount:number; total_amount:number; paid_amount:number; due_amount:number;
  opened_at:string; completed_at?:string; updated_at:string; items?:WorkshopItem[]; payments?:WorkshopPayment[];
};
export type TradeAccount = { id:string; mechanic_user_id:string; mechanic_name:string; mechanic_email?:string; tenant_id:string; store_id:string; store_name:string; balance:number; pending_amount:number; updated_at:string };
export type TradeRequest = { id:string; account_id:string; request_type:string; amount:number; method?:string; reservation_id?:string; reference_type?:string; reference_id?:string; note?:string; status:string; initiated_by_role:string; initiated_by_user_id:string; confirmed_by_role?:string; created_at:string; resolved_at?:string };
export type LedgerEntry = { id:string; request_id?:string; entry_type:string; reference_type?:string; reference_id?:string; debit:number; credit:number; note?:string; balance:number; created_at:string };
export type ReservationLite = { id:string; product_id:string; product_title:string; store_id:string; store_name:string; qty:number; unit_price:number; total_amount:number; status:string; created_at:string };

export type ManagementOverview = {
  generated_at:string; days:number;
  kpis:{ sales_amount:number; sales_count:number; gross_profit:number; discount_amount:number; customer_due_amount:number; return_amount:number; return_count:number; below_margin_lines:number; override_lines:number; unattributed_sales:number; active_mechanics:number; mechanic_receivable:number; pending_trade_amount:number; network_workshop_parts_amount:number };
  users:Array<{user_id:string; role:string; sales_count:number; sales_amount:number; gross_profit:number; discount_amount:number; override_lines:number; below_margin_lines:number; return_count:number; return_amount:number}>;
  inventory_signals:Array<{product_id:string; title:string; on_hand:number; available:number; sold_30_days:number; last_sale_at?:string; days_idle:number; signal:string; suggested_qty:number}>;
  customer_signals:Array<{customer_id:string; name:string; balance:number; last_sale_at?:string; days_idle:number; signal:string}>;
  mechanic_signals:Array<{mechanic_user_id:string; mechanic_name:string; balance:number; pending_amount:number; network_workshop_parts_amount_30_days:number}>;
  actions:Array<{kind:string; severity:string; title:string; detail:string; href:string}>;
};

export async function workshopSummary(session:UserSession){ return request<WorkshopSummary>(session,"/v1/mechanic/workshop/summary"); }
export async function listWorkshopJobs(session:UserSession,status="all"){ const x=await request<{items:WorkshopJob[]}>(session,`/v1/mechanic/workshop/jobs?status=${encodeURIComponent(status)}`);return x.items; }
export async function getWorkshopJob(session:UserSession,id:string){ return request<WorkshopJob>(session,`/v1/mechanic/workshop/jobs/${id}`); }
export async function createWorkshopJob(session:UserSession,input:{vehicle_token:string;customer_name?:string;customer_phone?:string;mileage?:number;complaint?:string;diagnosis?:string}){ return request<WorkshopJob>(session,"/v1/mechanic/workshop/jobs",{method:"POST",body:JSON.stringify(input)}); }
export async function addWorkshopItem(session:UserSession,id:string,input:{item_type:string;title:string;qty?:number;unit_price?:number;reservation_id?:string;notes?:string}){ return request<WorkshopJob>(session,`/v1/mechanic/workshop/jobs/${id}/items`,{method:"POST",body:JSON.stringify(input)}); }
export async function addWorkshopPayment(session:UserSession,id:string,input:{method:string;amount:number;note?:string}){ return request<WorkshopJob>(session,`/v1/mechanic/workshop/jobs/${id}/payments`,{method:"POST",body:JSON.stringify(input)}); }
export async function completeWorkshopJob(session:UserSession,id:string){ return request<WorkshopJob>(session,`/v1/mechanic/workshop/jobs/${id}/complete`,{method:"POST"}); }

export async function listMechanicTradeAccounts(session:UserSession){ const x=await request<{items:TradeAccount[]}>(session,"/v1/mechanic/trade-accounts");return x.items; }
export async function listMechanicTradeRequests(session:UserSession,status="pending"){ const x=await request<{items:TradeRequest[]}>(session,`/v1/mechanic/trade-requests?status=${status}`);return x.items; }
export async function mechanicLedger(session:UserSession,id:string){ const x=await request<{items:LedgerEntry[]}>(session,`/v1/mechanic/trade-accounts/${id}/ledger`);return x.items; }
export async function mechanicSettlement(session:UserSession,id:string,input:{amount:number;method:string;note?:string}){ return request<TradeRequest>(session,`/v1/mechanic/trade-accounts/${id}/settlements`,{method:"POST",body:JSON.stringify(input)}); }
export async function requestReservationCharge(session:UserSession,id:string,note=""){ return request<TradeRequest>(session,`/v1/mechanic/trade-requests/reservations/${id}`,{method:"POST",body:JSON.stringify({note})}); }
export async function resolveMechanicTradeRequest(session:UserSession,id:string,action:"confirm"|"reject"){ return request<TradeRequest>(session,`/v1/mechanic/trade-requests/${id}/resolve`,{method:"POST",body:JSON.stringify({action})}); }
export async function listMyReservations(session:UserSession){ const x=await request<{items:ReservationLite[]}>(session,"/v1/me/reservations");return x.items; }

export async function listStoreMechanicAccounts(session:UserSession){ const x=await request<{items:TradeAccount[]}>(session,"/v1/store/mechanic-accounts");return x.items; }
export async function listStoreMechanicRequests(session:UserSession,status="pending"){ const x=await request<{items:TradeRequest[]}>(session,`/v1/store/mechanic-trade-requests?status=${status}`);return x.items; }
export async function storeMechanicLedger(session:UserSession,id:string){ const x=await request<{items:LedgerEntry[]}>(session,`/v1/store/mechanic-accounts/${id}/ledger`);return x.items; }
export async function createStoreMechanicRequest(session:UserSession,id:string,input:{request_type:string;amount:number;method?:string;note?:string}){ return request<TradeRequest>(session,`/v1/store/mechanic-accounts/${id}/requests`,{method:"POST",body:JSON.stringify(input)}); }
export async function resolveStoreTradeRequest(session:UserSession,id:string,action:"confirm"|"reject"){ return request<TradeRequest>(session,`/v1/store/mechanic-trade-requests/${id}/resolve`,{method:"POST",body:JSON.stringify({action})}); }
export async function getManagementOverview(session:UserSession,days=30){ return request<ManagementOverview>(session,`/v1/management/overview?days=${days}`); }
