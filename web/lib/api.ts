import { addMockPartyBalance, applyMockAdjustment, applyMockPurchase, applyMockPurchaseReturn, applyMockReorder, applyMockSaleReturn, applyMockSettlement, getMockCustomerBalances, getMockInventory, getMockSupplierBalances, mockCustomers, mockProducts, mockPurchaseDetail, mockSaleDetail, mockSuppliers, mockNetworkResults, getMockNetworkProfile, setMockNetworkProfile, getMockStoreOffers, setMockStoreOffer, createMockReservation, getMockBuyerReservations, getMockStoreReservations, cancelMockReservation, transitionMockReservation } from "./mock";
import type {
  Customer,
  InventoryAdjustmentResult,
  InventoryStock,
  MeResponse,
  Product,
  PartyBalance,
  PaymentPart,
  PurchaseDetail,
  PurchaseItem,
  PurchaseResult,
  ReturnResult,
  SaleDetail,
  SaleItem,
  SettlementResult,
  Supplier,
  UserSession, NetworkSearchResult, NetworkStoreOffer, StoreNetworkProfile, NetworkReservation, NetworkReservationStatus
} from "./types";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
export const MOCK_MODE = process.env.NEXT_PUBLIC_MOCK_MODE !== "false";

async function request<T>(path: string, init: RequestInit = {}, token?: string): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body) headers.set("Content-Type", "application/json");
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const res = await fetch(`${API_URL}${path}`, { ...init, headers });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(body?.error?.message || `HTTP ${res.status}`);
  return body as T;
}

export async function getMe(token: string): Promise<MeResponse> {
  return request<MeResponse>("/v1/me", {}, token);
}

export async function searchProducts(q: string, session: UserSession): Promise<Product[]> {
  if (MOCK_MODE) {
    const s = q.trim().toLowerCase();
    return mockProducts.filter(p => !s || [p.title, p.sku, p.brand, p.oem_code].filter(Boolean).some(v => String(v).toLowerCase().includes(s)));
  }
  const out = await request<{items: Product[]}>(`/v1/products?q=${encodeURIComponent(q)}&limit=30`, {}, session.token);
  return out.items;
}

export async function searchCustomers(q: string, session: UserSession): Promise<Customer[]> {
  if (MOCK_MODE) return mockCustomers.filter(c => c.name.includes(q) || (c.phone || "").includes(q));
  const out = await request<{items: Customer[]}>(`/v1/customers?q=${encodeURIComponent(q)}&limit=30`, {}, session.token);
  return out.items;
}

export async function searchSuppliers(q: string, session: UserSession): Promise<Supplier[]> {
  if (MOCK_MODE) {
    const s = q.trim();
    return mockSuppliers.filter(x => !s || x.name.includes(s) || (x.phone || "").includes(s) || (x.code || "").includes(s));
  }
  const out = await request<{items: Supplier[]}>(`/v1/suppliers?q=${encodeURIComponent(q)}&limit=30`, {}, session.token);
  return out.items;
}

export async function postSale(session: UserSession, items: SaleItem[], customerId: string | null, paymentMethod: "cash"|"card"|"credit", payments?: PaymentPart[]) {
  if (MOCK_MODE) {
    await new Promise(r => setTimeout(r, 350));
    const total = items.reduce((s, i) => s + i.qty * i.unitPrice, 0);
    const paid = payments ? payments.reduce((s,p)=>s+p.amount,0) : (paymentMethod === "credit" ? 0 : total);
    const due = Math.max(0, total-paid);
    if (customerId && due > 0) addMockPartyBalance("customer", customerId, due);
    return { id: crypto.randomUUID(), total_amount: total, paid_amount: paid, due_amount: due, status: "posted" };
  }
  const idempotencyKey = crypto.randomUUID();
  return request<any>("/v1/sales", {
    method: "POST",
    headers: { "Idempotency-Key": idempotencyKey },
    body: JSON.stringify({
      warehouse_id: session.warehouseId,
      customer_id: customerId,
      payment_method: paymentMethod,
      ...(payments ? { payments } : {}),
      items: items.map(i => ({ product_id: i.product.id, qty: i.qty, unit_price: i.unitPrice }))
    })
  }, session.token);
}

export async function getInventory(session: UserSession, lowOnly = false): Promise<InventoryStock[]> {
  if (MOCK_MODE) {
    await new Promise(r => setTimeout(r, 180));
    return getMockInventory(lowOnly);
  }
  const out = await request<{items: InventoryStock[]}>(
    `/v1/inventory?warehouse_id=${encodeURIComponent(session.warehouseId)}&low_stock=${lowOnly ? "true" : "false"}&limit=100`,
    {},
    session.token
  );
  return out.items;
}

export async function postInventoryAdjustment(
  session: UserSession,
  productId: string,
  qtyDelta: number,
  reason: string
): Promise<InventoryAdjustmentResult> {
  if (MOCK_MODE) {
    await new Promise(r => setTimeout(r, 280));
    applyMockAdjustment(productId, qtyDelta);
    return { id: crypto.randomUUID(), status: "posted" };
  }
  return request<InventoryAdjustmentResult>("/v1/inventory/adjustments", {
    method: "POST",
    headers: { "Idempotency-Key": crypto.randomUUID() },
    body: JSON.stringify({
      warehouse_id: session.warehouseId,
      product_id: productId,
      qty_delta: qtyDelta,
      reason
    })
  }, session.token);
}

export async function putReorderPoint(
  session: UserSession,
  productId: string,
  minQty: number,
  targetQty: number
): Promise<void> {
  if (MOCK_MODE) {
    await new Promise(r => setTimeout(r, 220));
    applyMockReorder(productId, minQty, targetQty);
    return;
  }
  await request<{status: string}>("/v1/inventory/reorder-point", {
    method: "PUT",
    body: JSON.stringify({
      warehouse_id: session.warehouseId,
      product_id: productId,
      min_qty: minQty,
      target_qty: targetQty
    })
  }, session.token);
}

export async function postPurchase(
  session: UserSession,
  supplierId: string,
  items: PurchaseItem[],
  paymentMethod: "cash"|"card"|"credit",
  payments?: PaymentPart[]
): Promise<PurchaseResult> {
  if (MOCK_MODE) {
    await new Promise(r => setTimeout(r, 420));
    applyMockPurchase(items.map(i => ({ productId: i.product.id, qty: i.qty, unitCost: i.unitCost })));
    const total = items.reduce((s, i) => s + i.qty * i.unitCost, 0);
    const paid = payments ? payments.reduce((s,p)=>s+p.amount,0) : (paymentMethod === "credit" ? 0 : total);
    const due = Math.max(0,total-paid);
    if (due > 0) addMockPartyBalance("supplier", supplierId, due);
    return { id: crypto.randomUUID(), total_amount: total, paid_amount: paid, due_amount: due, status: "posted" };
  }
  return request<PurchaseResult>("/v1/purchases", {
    method: "POST",
    headers: { "Idempotency-Key": crypto.randomUUID() },
    body: JSON.stringify({
      warehouse_id: session.warehouseId,
      supplier_id: supplierId,
      payment_method: paymentMethod,
      ...(payments ? { payments } : {}),
      items: items.map(i => ({ product_id: i.product.id, qty: i.qty, unit_cost: i.unitCost }))
    })
  }, session.token);
}


export async function getCustomerBalances(session: UserSession, q=""): Promise<PartyBalance[]> {
  if (MOCK_MODE) return getMockCustomerBalances().filter(x=>!q||x.name.includes(q)||(x.phone||"").includes(q));
  const out=await request<{items:PartyBalance[]}>(`/v1/accounts/customers?q=${encodeURIComponent(q)}&limit=100`,{},session.token); return out.items;
}
export async function getSupplierBalances(session: UserSession, q=""): Promise<PartyBalance[]> {
  if (MOCK_MODE) return getMockSupplierBalances().filter(x=>!q||x.name.includes(q)||(x.phone||"").includes(q));
  const out=await request<{items:PartyBalance[]}>(`/v1/accounts/suppliers?q=${encodeURIComponent(q)}&limit=100`,{},session.token); return out.items;
}
export async function postCustomerReceipt(session:UserSession, customerId:string, amount:number, method:"cash"|"card", note=""):Promise<SettlementResult>{
  if(MOCK_MODE){await new Promise(r=>setTimeout(r,250));return{id:crypto.randomUUID(),party_type:"customer",method,amount,balance:applyMockSettlement("customer",customerId,amount),status:"posted"};}
  return request<SettlementResult>("/v1/settlements/customer-receipts",{method:"POST",headers:{"Idempotency-Key":crypto.randomUUID()},body:JSON.stringify({customer_id:customerId,amount,method,note})},session.token);
}
export async function postSupplierPayment(session:UserSession, supplierId:string, amount:number, method:"cash"|"card", note=""):Promise<SettlementResult>{
  if(MOCK_MODE){await new Promise(r=>setTimeout(r,250));return{id:crypto.randomUUID(),party_type:"supplier",method,amount,balance:applyMockSettlement("supplier",supplierId,amount),status:"posted"};}
  return request<SettlementResult>("/v1/settlements/supplier-payments",{method:"POST",headers:{"Idempotency-Key":crypto.randomUUID()},body:JSON.stringify({supplier_id:supplierId,amount,method,note})},session.token);
}
export async function getSaleDetail(session:UserSession,id:string):Promise<SaleDetail>{ if(MOCK_MODE){if(id!==mockSaleDetail.id)throw new Error(`در Mock Mode شناسه نمونه: ${mockSaleDetail.id}`);return structuredClone(mockSaleDetail);} return request<SaleDetail>(`/v1/sales/${encodeURIComponent(id)}`,{},session.token);}
export async function getPurchaseDetail(session:UserSession,id:string):Promise<PurchaseDetail>{ if(MOCK_MODE){if(id!==mockPurchaseDetail.id)throw new Error(`در Mock Mode شناسه نمونه: ${mockPurchaseDetail.id}`);return structuredClone(mockPurchaseDetail);} return request<PurchaseDetail>(`/v1/purchases/${encodeURIComponent(id)}`,{},session.token);}
export async function postSaleReturn(session:UserSession,saleId:string,items:{source_item_id:string;qty:number}[],refundMethod:"cash"|"card"|"credit_balance"):Promise<ReturnResult>{ if(MOCK_MODE){const amount=items.reduce((s,x)=>s+applyMockSaleReturn(x.qty),0);if(refundMethod==="credit_balance"&&mockSaleDetail.customer_id)addMockPartyBalance("customer",mockSaleDetail.customer_id,-amount);return{id:crypto.randomUUID(),total_amount:amount,status:"posted"};} return request<ReturnResult>("/v1/returns/sales",{method:"POST",headers:{"Idempotency-Key":crypto.randomUUID()},body:JSON.stringify({sale_id:saleId,refund_method:refundMethod,items})},session.token);}
export async function postPurchaseReturn(session:UserSession,purchaseId:string,items:{source_item_id:string;qty:number}[],refundMethod:"cash"|"card"|"credit_balance"):Promise<ReturnResult>{ if(MOCK_MODE){const amount=items.reduce((s,x)=>s+applyMockPurchaseReturn(x.qty),0);if(refundMethod==="credit_balance")addMockPartyBalance("supplier",mockPurchaseDetail.supplier_id,-amount);return{id:crypto.randomUUID(),total_amount:amount,status:"posted"};} return request<ReturnResult>("/v1/returns/purchases",{method:"POST",headers:{"Idempotency-Key":crypto.randomUUID()},body:JSON.stringify({purchase_id:purchaseId,refund_method:refundMethod,items})},session.token);}


export async function searchNetwork(q:string, opts:{lat?:number;lng?:number;sort?:"best"|"price"|"distance"|"fresh";limit?:number}={}):Promise<NetworkSearchResult[]>{
  if(MOCK_MODE){
    const tokens=q.trim().toLowerCase().split(/\s+/).filter(Boolean); let items=mockNetworkResults.filter(x=>{const hay=[x.title,x.brand,x.oem_code,x.sku].filter(Boolean).join(" ").toLowerCase().replace(/[۰-۹]/g,d=>String("۰۱۲۳۴۵۶۷۸۹".indexOf(d)));return tokens.every(t=>hay.includes(t));});
    const sort=opts.sort||"best"; if(sort==="price")items=[...items].sort((a,b)=>a.selling_price-b.selling_price); if(sort==="distance")items=[...items].sort((a,b)=>(a.distance_km||999)-(b.distance_km||999));
    await new Promise(r=>setTimeout(r,180)); return items.slice(0,opts.limit||30);
  }
  const p=new URLSearchParams({q,sort:opts.sort||"best",limit:String(opts.limit||30)}); if(opts.lat!=null&&opts.lng!=null){p.set("lat",String(opts.lat));p.set("lng",String(opts.lng));}
  const out=await request<{items:NetworkSearchResult[]}>(`/v1/network/search?${p.toString()}`); return out.items;
}
export async function getNetworkStoreProfile(session:UserSession):Promise<StoreNetworkProfile>{ if(MOCK_MODE)return getMockNetworkProfile(); return request<StoreNetworkProfile>("/v1/network/store-profile",{},session.token); }
export async function updateNetworkStoreProfile(session:UserSession,profile:StoreNetworkProfile):Promise<void>{ if(MOCK_MODE){setMockNetworkProfile(profile as any);return;} await request("/v1/network/store-profile",{method:"PUT",body:JSON.stringify({network_enabled:profile.network_enabled,address:profile.address||"",phone:profile.phone||"",city:profile.city||"",latitude:profile.latitude??null,longitude:profile.longitude??null})},session.token); }
export async function getNetworkStoreOffers(session:UserSession):Promise<NetworkStoreOffer[]>{ if(MOCK_MODE)return getMockStoreOffers(); const out=await request<{items:NetworkStoreOffer[]}>(`/v1/network/offers?warehouse_id=${encodeURIComponent(session.warehouseId)}`,{},session.token);return out.items; }
export async function updateNetworkStoreOffer(session:UserSession,productId:string,price:number,visible:boolean,allowReservation:boolean):Promise<void>{ if(MOCK_MODE){setMockStoreOffer(productId,price,visible,allowReservation);return;} await request(`/v1/network/offers/${encodeURIComponent(productId)}`,{method:"PUT",body:JSON.stringify({warehouse_id:session.warehouseId,selling_price:price,visible,allow_reservation:allowReservation})},session.token); }


export async function createNetworkReservation(session:UserSession,offerId:string,qty:number):Promise<NetworkReservation>{
  if(MOCK_MODE){await new Promise(r=>setTimeout(r,220));return createMockReservation(offerId,qty);}
  return request<NetworkReservation>("/v1/network/reservations",{method:"POST",headers:{"Idempotency-Key":crypto.randomUUID()},body:JSON.stringify({offer_id:offerId,qty})},session.token);
}
export async function getMyReservations(session:UserSession):Promise<NetworkReservation[]>{
  if(MOCK_MODE)return getMockBuyerReservations(); const out=await request<{items:NetworkReservation[]}>("/v1/me/reservations",{},session.token);return out.items;
}
export async function cancelMyReservation(session:UserSession,id:string):Promise<NetworkReservation>{
  if(MOCK_MODE)return cancelMockReservation(id); return request<NetworkReservation>(`/v1/me/reservations/${encodeURIComponent(id)}/cancel`,{method:"POST"},session.token);
}
export async function getStoreReservations(session:UserSession,status="all"):Promise<NetworkReservation[]>{
  if(MOCK_MODE)return getMockStoreReservations().filter(x=>status==="all"||x.status===status); const out=await request<{items:NetworkReservation[]}>(`/v1/network/reservations?status=${encodeURIComponent(status)}`,{},session.token);return out.items;
}
export async function transitionStoreReservation(session:UserSession,id:string,status:"accepted"|"ready"|"rejected"):Promise<NetworkReservation>{
  if(MOCK_MODE)return transitionMockReservation(id,status); return request<NetworkReservation>(`/v1/network/reservations/${encodeURIComponent(id)}`,{method:"PATCH",body:JSON.stringify({status})},session.token);
}
