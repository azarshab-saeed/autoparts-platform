import type { SaleItem } from "./types";

export const STORE_EDGE_URL = process.env.NEXT_PUBLIC_STORE_EDGE_URL || "http://127.0.0.1:17624";

export type StoreEdgeStatus = {
  paired: boolean;
  store_name?: string;
  device_name?: string;
  pending_sales: number;
  conflicts: number;
  last_sync_at?: string;
  last_sync_error?: string;
  catalog_items: number;
  snapshot_at?: string;
};

export type StoreEdgeLocalSale = {
  local_operation_id: string;
  local_number: string;
  created_at: string;
  payment_method: "cash" | "card";
  total_amount: number;
  status: "pending" | "synced" | "conflict";
  server_sale_id?: string;
  last_error?: string;
};

async function localRequest<T>(path:string, init:RequestInit={}, timeoutMs=1400):Promise<T>{
  const controller=new AbortController();
  const timer=setTimeout(()=>controller.abort(),timeoutMs);
  try{
    const headers=new Headers(init.headers);
    if(init.body)headers.set("Content-Type","application/json");
    headers.set("X-AutoParts-Edge","1");
    const res=await fetch(`${STORE_EDGE_URL}${path}`,{...init,headers,signal:controller.signal});
    const body=await res.json().catch(()=>({}));
    if(!res.ok)throw new Error(body?.error?.message||`Store Edge HTTP ${res.status}`);
    return body as T;
  }finally{clearTimeout(timer)}
}

export async function getStoreEdgeStatus():Promise<StoreEdgeStatus>{
  return localRequest<StoreEdgeStatus>("/v1/status",{},900);
}

export async function queueOfflineSale(items:SaleItem[],paymentMethod:"cash"|"card"):Promise<StoreEdgeLocalSale>{
  return localRequest<StoreEdgeLocalSale>("/v1/offline-sales",{
    method:"POST",
    body:JSON.stringify({payment_method:paymentMethod,items:items.map(x=>({product_id:x.product.id,title:x.product.title,qty:x.qty,unit_price:x.unitPrice}))})
  },2500);
}

export async function triggerStoreEdgeSync():Promise<void>{
  await localRequest<{status:string}>("/v1/sync",{method:"POST",body:"{}"},30000);
}
