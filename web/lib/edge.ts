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
  cashier_may_override?: boolean;
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

export async function queueOfflineSale(items:SaleItem[],paymentMethod:"cash"|"card",customerId?:string|null):Promise<StoreEdgeLocalSale>{
  return localRequest<StoreEdgeLocalSale>("/v1/offline-sales",{
    method:"POST",
    body:JSON.stringify({payment_method:paymentMethod,customer_id:customerId||undefined,items:items.map(x=>({product_id:x.product.id,title:x.product.title,qty:x.qty,unit_price:x.unitPrice,manual_price:Boolean(x.manualPrice),preserve_price:true}))})
  },2500);
}

export async function triggerStoreEdgeSync():Promise<void>{
  await localRequest<{status:string}>("/v1/sync",{method:"POST",body:"{}"},30000);
}

export type EdgePrinterConfig = {
  enabled:boolean;
  name:string;
  transport:"tcp9100"|"file"|"windows_share"|"windows_spool_text"|"system_file"|string;
  address:string;
  language:"text"|"escpos"|"zpl"|"pdf"|string;
};
export type EdgeHardwareConfig = {
  receipt_printer:EdgePrinterConfig;
  label_printer:EdgePrinterConfig;
  a4_printer:EdgePrinterConfig;
  cash_drawer:{enabled:boolean;auto_open:boolean};
  pos:{provider:"manual"|"mock"|"tcp_json"|string;address:string;timeout_seconds:number};
  auto_print_receipt:boolean;
};
export type EdgePOSResult={approved:boolean;requires_operator:boolean;provider:string;rrn?:string;trace?:string;message?:string};

export async function getEdgeHardwareConfig():Promise<EdgeHardwareConfig>{return localRequest<EdgeHardwareConfig>("/v1/hardware/config",{},1400)}
export async function saveEdgeHardwareConfig(cfg:EdgeHardwareConfig):Promise<EdgeHardwareConfig>{return localRequest<EdgeHardwareConfig>("/v1/hardware/config",{method:"PUT",body:JSON.stringify(cfg)},2500)}
export async function getEdgeHardwareStatus():Promise<Record<string,unknown>>{return localRequest<Record<string,unknown>>("/v1/hardware/status",{},1400)}
export async function printEdgeReceipt(input:{number:string;store_name?:string;created_at?:string;payment_method:string;total_amount:number;lines:{title:string;qty:number;unit_price:number}[]}):Promise<void>{await localRequest<{status:string}>("/v1/hardware/receipt/print",{method:"POST",body:JSON.stringify(input)},7000)}
export async function printEdgeLabel(input:{title:string;sku?:string;barcode?:string;price?:number;copies?:number}):Promise<void>{await localRequest<{status:string}>("/v1/hardware/label/print",{method:"POST",body:JSON.stringify(input)},7000)}
export async function openEdgeCashDrawer():Promise<void>{await localRequest<{status:string}>("/v1/hardware/cash-drawer/open",{method:"POST",body:"{}"},5000)}
export async function chargeEdgePOS(amount:number,reference:string):Promise<EdgePOSResult>{return localRequest<EdgePOSResult>("/v1/hardware/pos/charge",{method:"POST",body:JSON.stringify({amount,reference})},35000)}
