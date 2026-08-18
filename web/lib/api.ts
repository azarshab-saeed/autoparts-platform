import { addMockExpense, addMockPartyBalance, applyMockAdjustment, applyMockPurchase, applyMockPurchaseReturn, applyMockReorder, applyMockSaleReturn, applyMockSettlement, getMockCustomerBalances, getMockExpenseCategories, getMockExpenses, getMockInventory, getMockPartyStatement, getMockProfitLoss, getMockSupplierBalances, mockCustomers, mockProducts, mockPurchaseDetail, mockSaleDetail, mockSuppliers, mockNetworkResults, getMockNetworkProfile, setMockNetworkProfile, getMockStoreOffers, setMockStoreOffer, createMockReservation, getMockBuyerReservations, getMockStoreReservations, cancelMockReservation, transitionMockReservation, fulfillMockReservation, getMockDashboardSummary, getMockSalesHistory, getMockPurchasesHistory, getMockInventoryInsights, getMockCashReport, closeMockBusinessDay } from "./mock";
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
  UserSession, NetworkSearchResult, NetworkStoreOffer, StoreNetworkProfile, NetworkReservation, NetworkReservationStatus, ReservationFulfillmentResult, Expense, ExpenseCategory, PartyStatement, ProfitLoss, CashReport, DailyClosing, DashboardSummary, InventoryInsightReport, PagedResult, PurchaseHistoryItem, SaleHistoryItem, NetworkProcurement, ProcurementReceiveResult, VehicleMake, ProductSearchMetadata, ProductSearchTerm, ProductFitmentInput, AuditLogEntry, ProductImportRow, ProductImportResult, EdgePairing, EdgeDevice, BankAccount, BankLedger, StoreCheck, CheckSummary, CheckDirection, CheckAction
} from "./types";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
export const MOCK_MODE = process.env.NEXT_PUBLIC_MOCK_MODE !== "false";

export class ApiError extends Error {
  status: number;
  constructor(status:number,message:string){super(message);this.status=status;this.name="ApiError";}
}

async function request<T>(path: string, init: RequestInit = {}, token?: string): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body) headers.set("Content-Type", "application/json");
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const res = await fetch(`${API_URL}${path}`, { ...init, headers });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new ApiError(res.status,body?.error?.message || `HTTP ${res.status}`);
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

export async function importProducts(session: UserSession, rows: ProductImportRow[]): Promise<ProductImportResult> {
  if (MOCK_MODE) {
    await new Promise(r => setTimeout(r, 500));
    return {
      batch_id: crypto.randomUUID(), row_count: rows.length, created_count: rows.length, updated_count: 0,
      inventory_initialized_count: rows.length, inventory_preserved_count: 0,
      offers_upserted_count: rows.filter(x => x.selling_price > 0).length,
      opening_inventory_value: rows.reduce((sum,x)=>sum+x.on_hand*x.avg_unit_cost,0),
      rows: rows.map(x=>({row_number:x.row_number,product_id:crypto.randomUUID(),product_action:"created" as const,inventory_action:"initialized" as const,offer_action:x.selling_price>0?"upserted" as const:"none" as const}))
    };
  }
  return request<ProductImportResult>("/v1/products/import", {
    method: "POST",
    headers: { "Idempotency-Key": crypto.randomUUID() },
    body: JSON.stringify({ warehouse_id: session.warehouseId, rows })
  }, session.token);
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

let mockBankAccounts:BankAccount[]=[];
let mockChecks:StoreCheck[]=[];

export async function getBankAccounts(session:UserSession):Promise<BankAccount[]>{
  if(MOCK_MODE)return mockBankAccounts.map(x=>({...x}));
  const out=await request<{items:BankAccount[]}>("/v1/banking/accounts",{},session.token);return out.items??[];
}
export async function createBankAccount(session:UserSession,input:{name:string;bankName:string;accountNumber?:string;cardNumber?:string;iban?:string;openingBalance:number;isDefault:boolean}):Promise<BankAccount>{
  if(MOCK_MODE){const x:BankAccount={id:crypto.randomUUID(),name:input.name,bank_name:input.bankName,account_number:input.accountNumber,card_number:input.cardNumber,iban:input.iban,opening_balance:input.openingBalance,balance:input.openingBalance,active:true,is_default:input.isDefault,created_at:new Date().toISOString()};if(input.isDefault)mockBankAccounts=mockBankAccounts.map(b=>({...b,is_default:false}));mockBankAccounts=[x,...mockBankAccounts];return x;}
  return request<BankAccount>("/v1/banking/accounts",{method:"POST",headers:{"Idempotency-Key":crypto.randomUUID()},body:JSON.stringify({name:input.name,bank_name:input.bankName,account_number:input.accountNumber||"",card_number:input.cardNumber||"",iban:input.iban||"",opening_balance:input.openingBalance,is_default:input.isDefault})},session.token);
}
export async function getBankLedger(session:UserSession,id:string):Promise<BankLedger>{
  if(MOCK_MODE){const account=mockBankAccounts.find(x=>x.id===id);if(!account)throw new Error("حساب بانکی پیدا نشد");return{account:{...account},items:[]};}
  return request<BankLedger>(`/v1/banking/accounts/${encodeURIComponent(id)}/ledger`,{},session.token);
}
export async function getChecks(session:UserSession,opts:{direction?:CheckDirection;status?:string;q?:string;cursor?:string;limit?:number}={}):Promise<PagedResult<StoreCheck>>{
  if(MOCK_MODE){let items=mockChecks.filter(x=>(!opts.direction||x.direction===opts.direction)&&(!opts.status||x.status===opts.status)&&(!opts.q||[x.check_number,x.sayad_id,x.customer_name,x.supplier_name].filter(Boolean).some(v=>String(v).includes(opts.q!))));return{items,total:items.length,next_cursor:""};}
  const p=new URLSearchParams({limit:String(opts.limit||100)});if(opts.direction)p.set("direction",opts.direction);if(opts.status)p.set("status",opts.status);if(opts.q)p.set("q",opts.q);if(opts.cursor)p.set("cursor",opts.cursor);return request<PagedResult<StoreCheck>>(`/v1/checks?${p.toString()}`,{},session.token);
}
export async function getCheckSummary(session:UserSession):Promise<CheckSummary>{
  if(MOCK_MODE){const today=new Date().toISOString().slice(0,10);const next=new Date(Date.now()+7*86400000).toISOString().slice(0,10);const open=(x:StoreCheck)=>["held","deposited","issued","returned"].includes(x.status);return{receivable_open_amount:mockChecks.filter(x=>x.direction==="receivable"&&["held","deposited","returned"].includes(x.status)).reduce((s,x)=>s+x.amount,0),payable_open_amount:mockChecks.filter(x=>x.direction==="payable"&&x.status==="issued").reduce((s,x)=>s+x.amount,0),due_today_count:mockChecks.filter(x=>open(x)&&x.due_date===today).length,due_next_7_count:mockChecks.filter(x=>open(x)&&x.due_date>today&&x.due_date<=next).length,overdue_count:mockChecks.filter(x=>open(x)&&x.due_date<today).length,bounced_count:mockChecks.filter(x=>x.status==="bounced").length};}
  return request<CheckSummary>("/v1/checks/summary",{},session.token);
}
export async function createStoreCheck(session:UserSession,direction:CheckDirection,input:{partyId:string;checkNumber:string;sayadId?:string;bankName?:string;branchName?:string;amount:number;issueDate:string;dueDate:string;note?:string}):Promise<StoreCheck>{
  if(MOCK_MODE){const customer=direction==="receivable"?getMockCustomerBalances().find(x=>x.id===input.partyId):undefined;const supplier=direction==="payable"?getMockSupplierBalances().find(x=>x.id===input.partyId):undefined;const x:StoreCheck={id:crypto.randomUUID(),direction,...(customer?{customer_id:customer.id,customer_name:customer.name}:{}),...(supplier?{supplier_id:supplier.id,supplier_name:supplier.name}:{}),check_number:input.checkNumber,sayad_id:input.sayadId,bank_name:input.bankName,branch_name:input.branchName,amount:input.amount,issue_date:input.issueDate,due_date:input.dueDate,status:direction==="receivable"?"held":"issued",note:input.note,created_at:new Date().toISOString(),updated_at:new Date().toISOString()};mockChecks=[x,...mockChecks];applyMockSettlement(direction==="receivable"?"customer":"supplier",input.partyId,input.amount);return x;}
  const party=direction==="receivable"?{customer_id:input.partyId}:{supplier_id:input.partyId};return request<StoreCheck>(`/v1/checks/${direction}`,{method:"POST",headers:{"Idempotency-Key":crypto.randomUUID()},body:JSON.stringify({...party,check_number:input.checkNumber,sayad_id:input.sayadId||"",bank_name:input.bankName||"",branch_name:input.branchName||"",amount:input.amount,issue_date:input.issueDate,due_date:input.dueDate,note:input.note||""})},session.token);
}
export async function transitionStoreCheck(session:UserSession,id:string,action:CheckAction,input:{bankAccountId?:string;supplierId?:string;note?:string}={}):Promise<StoreCheck>{
  if(MOCK_MODE){
    const i=mockChecks.findIndex(x=>x.id===id);if(i<0)throw new Error("چک پیدا نشد");const current=mockChecks[i];
    const nextStatus:Record<CheckAction,StoreCheck["status"]>={deposit:"deposited",clear:"cleared",bounce:"bounced",endorse:"endorsed",return_endorsement:"returned",return:"returned",cancel:"cancelled"};
    if(action==="bounce"&&current.customer_id)addMockPartyBalance("customer",current.customer_id,current.amount);
    if(action==="cancel"&&current.direction==="receivable"&&current.customer_id)addMockPartyBalance("customer",current.customer_id,current.amount);
    if(action==="endorse"&&input.supplierId)applyMockSettlement("supplier",input.supplierId,current.amount);
    if(action==="return_endorsement"&&current.endorsed_supplier_id)addMockPartyBalance("supplier",current.endorsed_supplier_id,current.amount);
    if((action==="return"||action==="cancel")&&current.direction==="payable"&&current.supplier_id)addMockPartyBalance("supplier",current.supplier_id,current.amount);
    if(action==="clear"&&input.bankAccountId){const b=mockBankAccounts.find(x=>x.id===input.bankAccountId);if(b)b.balance+=current.direction==="receivable"?current.amount:-current.amount;}
    const updated={...current,status:nextStatus[action],bank_account_id:input.bankAccountId||current.bank_account_id,endorsed_supplier_id:action==="endorse"?input.supplierId:(action==="return_endorsement"?undefined:current.endorsed_supplier_id),updated_at:new Date().toISOString()};mockChecks[i]=updated;return{...updated};
  }
  return request<StoreCheck>(`/v1/checks/${encodeURIComponent(id)}/transition`,{method:"POST",headers:{"Idempotency-Key":crypto.randomUUID()},body:JSON.stringify({action,bank_account_id:input.bankAccountId||undefined,supplier_id:input.supplierId||undefined,note:input.note||""})},session.token);
}
export async function getSaleDetail(session:UserSession,id:string):Promise<SaleDetail>{ if(MOCK_MODE){if(id!==mockSaleDetail.id)throw new Error(`در Mock Mode شناسه نمونه: ${mockSaleDetail.id}`);return structuredClone(mockSaleDetail);} return request<SaleDetail>(`/v1/sales/${encodeURIComponent(id)}`,{},session.token);}
export async function getPurchaseDetail(session:UserSession,id:string):Promise<PurchaseDetail>{ if(MOCK_MODE){if(id!==mockPurchaseDetail.id)throw new Error(`در Mock Mode شناسه نمونه: ${mockPurchaseDetail.id}`);return structuredClone(mockPurchaseDetail);} return request<PurchaseDetail>(`/v1/purchases/${encodeURIComponent(id)}`,{},session.token);}
export async function postSaleReturn(session:UserSession,saleId:string,items:{source_item_id:string;qty:number}[],refundMethod:"cash"|"card"|"credit_balance"):Promise<ReturnResult>{ if(MOCK_MODE){const amount=items.reduce((s,x)=>s+applyMockSaleReturn(x.qty),0);if(refundMethod==="credit_balance"&&mockSaleDetail.customer_id)addMockPartyBalance("customer",mockSaleDetail.customer_id,-amount);return{id:crypto.randomUUID(),total_amount:amount,status:"posted"};} return request<ReturnResult>("/v1/returns/sales",{method:"POST",headers:{"Idempotency-Key":crypto.randomUUID()},body:JSON.stringify({sale_id:saleId,refund_method:refundMethod,items})},session.token);}
export async function postPurchaseReturn(session:UserSession,purchaseId:string,items:{source_item_id:string;qty:number}[],refundMethod:"cash"|"card"|"credit_balance"):Promise<ReturnResult>{ if(MOCK_MODE){const amount=items.reduce((s,x)=>s+applyMockPurchaseReturn(x.qty),0);if(refundMethod==="credit_balance")addMockPartyBalance("supplier",mockPurchaseDetail.supplier_id,-amount);return{id:crypto.randomUUID(),total_amount:amount,status:"posted"};} return request<ReturnResult>("/v1/returns/purchases",{method:"POST",headers:{"Idempotency-Key":crypto.randomUUID()},body:JSON.stringify({purchase_id:purchaseId,refund_method:refundMethod,items})},session.token);}


export async function searchNetwork(q:string, opts:{lat?:number;lng?:number;sort?:"best"|"price"|"distance"|"fresh";limit?:number;vehicleVariantId?:string;year?:number}={}):Promise<NetworkSearchResult[]>{
  if(MOCK_MODE){
    const tokens=q.trim().toLowerCase().split(/\s+/).filter(Boolean); let items=mockNetworkResults.filter(x=>{const hay=[x.title,x.brand,x.oem_code,x.sku].filter(Boolean).join(" ").toLowerCase().replace(/[۰-۹]/g,d=>String("۰۱۲۳۴۵۶۷۸۹".indexOf(d)));return (!tokens.length||tokens.every(t=>hay.includes(t)));});
    const sort=opts.sort||"best"; if(sort==="price")items=[...items].sort((a,b)=>a.selling_price-b.selling_price); if(sort==="distance")items=[...items].sort((a,b)=>(a.distance_km||999)-(b.distance_km||999));
    await new Promise(r=>setTimeout(r,180)); return items.slice(0,opts.limit||30);
  }
  const p=new URLSearchParams({q,sort:opts.sort||"best",limit:String(opts.limit||30)});
  if(opts.lat!=null&&opts.lng!=null){p.set("lat",String(opts.lat));p.set("lng",String(opts.lng));}
  if(opts.vehicleVariantId)p.set("vehicle_variant_id",opts.vehicleVariantId);
  if(opts.year!=null)p.set("year",String(opts.year));
  const out=await request<{items:NetworkSearchResult[]}>(`/v1/network/search?${p.toString()}`); return out.items ?? [];
}
export async function getVehicleCatalog():Promise<VehicleMake[]>{
  if(MOCK_MODE)return [
    {id:"make-peugeot",name:"پژو",models:[{id:"model-206",name:"206",variants:[{id:"92000000-0000-0000-0000-000000000001",name:"تیپ 5",engine_code:"TU5",year_from:1380,year_to:1400},{id:"92000000-0000-0000-0000-000000000002",name:"تیپ 2",engine_code:"TU3",year_from:1380,year_to:1400}]}]},
    {id:"make-ikco",name:"ایران خودرو",models:[{id:"model-samand",name:"سمند",variants:[{id:"92000000-0000-0000-0000-000000000004",name:"LX EF7",engine_code:"EF7",year_from:1388,year_to:1402}]}]}
  ];
  const out=await request<{items:VehicleMake[]}>("/v1/vehicles/catalog");return out.items ?? [];
}
export async function getProductSearchMetadata(session:UserSession,productId:string):Promise<ProductSearchMetadata>{
  if(MOCK_MODE)return{product_id:productId,terms:[],fitments:[]};
  return request<ProductSearchMetadata>(`/v1/products/${encodeURIComponent(productId)}/search-metadata`,{},session.token);
}
export async function updateProductSearchMetadata(session:UserSession,productId:string,terms:ProductSearchTerm[],fitments:ProductFitmentInput[]):Promise<ProductSearchMetadata>{
  if(MOCK_MODE)return{product_id:productId,terms,fitments:[]};
  return request<ProductSearchMetadata>(`/v1/products/${encodeURIComponent(productId)}/search-metadata`,{method:"PUT",body:JSON.stringify({terms,fitments})},session.token);
}
export async function getNetworkStoreProfile(session:UserSession):Promise<StoreNetworkProfile>{ if(MOCK_MODE)return getMockNetworkProfile(); return request<StoreNetworkProfile>("/v1/network/store-profile",{},session.token); }
export async function updateNetworkStoreProfile(session:UserSession,profile:StoreNetworkProfile):Promise<void>{ if(MOCK_MODE){setMockNetworkProfile(profile as any);return;} await request("/v1/network/store-profile",{method:"PUT",body:JSON.stringify({network_enabled:profile.network_enabled,address:profile.address||"",phone:profile.phone||"",city:profile.city||"",latitude:profile.latitude??null,longitude:profile.longitude??null})},session.token); }
export async function getNetworkStoreOffers(session:UserSession):Promise<NetworkStoreOffer[]>{ if(MOCK_MODE)return getMockStoreOffers(); const out=await request<{items:NetworkStoreOffer[]}>(`/v1/network/offers?warehouse_id=${encodeURIComponent(session.warehouseId)}`,{},session.token);return out.items; }
export async function updateNetworkStoreOffer(session:UserSession,productId:string,price:number,visible:boolean,allowReservation:boolean,allowProcurement:boolean):Promise<void>{ if(MOCK_MODE){setMockStoreOffer(productId,price,visible,allowReservation);return;} await request(`/v1/network/offers/${encodeURIComponent(productId)}`,{method:"PUT",body:JSON.stringify({warehouse_id:session.warehouseId,selling_price:price,visible,allow_reservation:allowReservation,allow_procurement:allowProcurement})},session.token); }

let mockProcurements:NetworkProcurement[]=[];
export async function searchNetworkProcurement(session:UserSession,q:string,sort:"best"|"price"|"distance"|"fresh"="best"):Promise<NetworkSearchResult[]>{
  if(MOCK_MODE)return mockNetworkResults.filter(x=>x.store_id!==session.storeId);
  const out=await request<{items:NetworkSearchResult[]}>(`/v1/network/procurement/search?q=${encodeURIComponent(q)}&sort=${sort}&limit=50`,{},session.token);return out.items;
}
export async function createNetworkProcurement(session:UserSession,input:{offerId:string;buyerProductId:string;qty:number}):Promise<NetworkProcurement>{
  if(MOCK_MODE){const offer=mockNetworkResults.find(x=>x.offer_id===input.offerId)!;const now=new Date();const x:NetworkProcurement={id:crypto.randomUUID(),buyer_store_id:session.storeId,buyer_store_name:session.storeName,buyer_warehouse_id:session.warehouseId,buyer_product_id:input.buyerProductId,buyer_product_title:offer.title,seller_store_id:offer.store_id,seller_store_name:offer.store_name,seller_warehouse_id:"mock",seller_product_id:offer.product_id,seller_product_title:offer.title,offer_id:offer.offer_id,qty:input.qty,unit_price:offer.selling_price,total_amount:input.qty*offer.selling_price,status:"requested",expires_at:new Date(now.getTime()+12*3600000).toISOString(),created_at:now.toISOString(),updated_at:now.toISOString()};mockProcurements=[x,...mockProcurements];return x;}
  return request<NetworkProcurement>("/v1/network/procurements",{method:"POST",headers:{"Idempotency-Key":crypto.randomUUID()},body:JSON.stringify({offer_id:input.offerId,buyer_product_id:input.buyerProductId,warehouse_id:session.warehouseId,qty:input.qty})},session.token);
}
export async function getBuyingProcurements(session:UserSession):Promise<NetworkProcurement[]>{
  if(MOCK_MODE)return mockProcurements.filter(x=>x.buyer_store_id===session.storeId);const out=await request<{items:NetworkProcurement[]}>("/v1/network/procurements/buying",{},session.token);return out.items ?? [];
}
export async function getSellingProcurements(session:UserSession):Promise<NetworkProcurement[]>{
  if(MOCK_MODE)return mockProcurements.filter(x=>x.seller_store_id===session.storeId);const out=await request<{items:NetworkProcurement[]}>("/v1/network/procurements/selling",{},session.token);return out.items ?? [];
}
export async function transitionSellingProcurement(session:UserSession,id:string,status:"accepted"|"ready"|"rejected"):Promise<NetworkProcurement>{
  if(MOCK_MODE){let out=mockProcurements.find(x=>x.id===id)!;out={...out,status,updated_at:new Date().toISOString()};mockProcurements=mockProcurements.map(x=>x.id===id?out:x);return out;}
  return request<NetworkProcurement>(`/v1/network/procurements/${encodeURIComponent(id)}`,{method:"PATCH",body:JSON.stringify({status})},session.token);
}
export async function cancelBuyingProcurement(session:UserSession,id:string):Promise<NetworkProcurement>{
  if(MOCK_MODE){let out=mockProcurements.find(x=>x.id===id)!;out={...out,status:"cancelled",updated_at:new Date().toISOString()};mockProcurements=mockProcurements.map(x=>x.id===id?out:x);return out;}
  return request<NetworkProcurement>(`/v1/network/procurements/${encodeURIComponent(id)}/cancel`,{method:"POST"},session.token);
}
export async function receiveNetworkProcurement(session:UserSession,id:string):Promise<ProcurementReceiveResult>{
  if(MOCK_MODE){const purchase=crypto.randomUUID(),sale=crypto.randomUUID();mockProcurements=mockProcurements.map(x=>x.id===id?{...x,status:"received",buyer_purchase_id:purchase,seller_sale_id:sale,updated_at:new Date().toISOString()}:x);return{procurement_id:id,buyer_purchase_id:purchase,seller_sale_id:sale,total_amount:mockProcurements.find(x=>x.id===id)?.total_amount||0,status:"received"};}
  return request<ProcurementReceiveResult>(`/v1/network/procurements/${encodeURIComponent(id)}/receive`,{method:"POST",headers:{"Idempotency-Key":crypto.randomUUID()}},session.token);
}



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
export async function fulfillStoreReservation(session:UserSession,id:string,paymentMethod:"cash"|"card"|"credit",customerId?:string,payments?:PaymentPart[]):Promise<ReservationFulfillmentResult>{
  if(MOCK_MODE){await new Promise(r=>setTimeout(r,250));return fulfillMockReservation(id);}
  return request<ReservationFulfillmentResult>(`/v1/network/reservations/${encodeURIComponent(id)}/fulfill`,{
    method:"POST",
    headers:{"Idempotency-Key":crypto.randomUUID()},
    body:JSON.stringify({payment_method:paymentMethod,...(customerId?{customer_id:customerId}:{}),...(payments?{payments}:{})})
  },session.token);
}

export async function getExpenseCategories(session:UserSession):Promise<ExpenseCategory[]>{
  if(MOCK_MODE)return getMockExpenseCategories();const out=await request<{items:ExpenseCategory[]}>("/v1/expenses/categories",{},session.token);return out.items;
}
export async function getExpenses(session:UserSession,from:string,to:string,categoryId=""):Promise<Expense[]>{
  if(MOCK_MODE)return getMockExpenses().filter(x=>(!categoryId||x.category_id===categoryId)&&x.occurred_on>=from&&x.occurred_on<=to);
  const p=new URLSearchParams({from,to,limit:"200"});if(categoryId)p.set("category_id",categoryId);const out=await request<{items:Expense[]}>(`/v1/expenses?${p.toString()}`,{},session.token);return out.items;
}
export async function postExpense(session:UserSession,input:{categoryId:string;amount:number;method:"cash"|"card";note:string;occurredOn:string}):Promise<Expense>{
  if(MOCK_MODE){await new Promise(r=>setTimeout(r,220));return addMockExpense(input.categoryId,input.amount,input.method,input.note,input.occurredOn);}
  return request<Expense>("/v1/expenses",{method:"POST",headers:{"Idempotency-Key":crypto.randomUUID()},body:JSON.stringify({category_id:input.categoryId,amount:input.amount,method:input.method,note:input.note,occurred_on:input.occurredOn})},session.token);
}
export async function getProfitLoss(session:UserSession,from:string,to:string):Promise<ProfitLoss>{
  if(MOCK_MODE)return getMockProfitLoss(from,to);return request<ProfitLoss>(`/v1/reports/profit-loss?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`,{},session.token);
}
export async function getPartyStatement(session:UserSession,type:"customer"|"supplier",id:string):Promise<PartyStatement>{
  if(MOCK_MODE)return getMockPartyStatement(type,id);const base=type==="customer"?"customers":"suppliers";return request<PartyStatement>(`/v1/accounts/${base}/${encodeURIComponent(id)}/statement`,{},session.token);
}

export async function getDashboardSummary(session:UserSession):Promise<DashboardSummary>{
  if(MOCK_MODE)return getMockDashboardSummary();
  return request<DashboardSummary>("/v1/dashboard",{},session.token);
}

export async function getSalesHistory(session:UserSession,opts:{from:string;to:string;q?:string;paymentState?:"all"|"paid"|"due";customerId?:string;cursor?:string;limit?:number}):Promise<PagedResult<SaleHistoryItem>>{
  if(MOCK_MODE){const items=getMockSalesHistory(opts.q||"",opts.paymentState||"all");return{items,total:items.length,next_cursor:""};}
  const p=new URLSearchParams({from:opts.from,to:opts.to,limit:String(opts.limit||50),payment_state:opts.paymentState||"all"});if(opts.q)p.set("q",opts.q);if(opts.customerId)p.set("customer_id",opts.customerId);if(opts.cursor)p.set("cursor",opts.cursor);
  return request<PagedResult<SaleHistoryItem>>(`/v1/sales?${p.toString()}`,{},session.token);
}

export async function getPurchasesHistory(session:UserSession,opts:{from:string;to:string;q?:string;paymentState?:"all"|"paid"|"due";supplierId?:string;cursor?:string;limit?:number}):Promise<PagedResult<PurchaseHistoryItem>>{
  if(MOCK_MODE){const items=getMockPurchasesHistory(opts.q||"",opts.paymentState||"all");return{items,total:items.length,next_cursor:""};}
  const p=new URLSearchParams({from:opts.from,to:opts.to,limit:String(opts.limit||50),payment_state:opts.paymentState||"all"});if(opts.q)p.set("q",opts.q);if(opts.supplierId)p.set("supplier_id",opts.supplierId);if(opts.cursor)p.set("cursor",opts.cursor);
  return request<PagedResult<PurchaseHistoryItem>>(`/v1/purchases?${p.toString()}`,{},session.token);
}

export async function getInventoryInsights(session:UserSession,opts:{q?:string;sort?:"title"|"value"|"low_stock"|"sold_qty"|"dead_stock";cursor?:string;limit?:number}={}):Promise<InventoryInsightReport>{
  if(MOCK_MODE)return getMockInventoryInsights(opts.q||"",opts.sort||"title");
  const p=new URLSearchParams({warehouse_id:session.warehouseId,limit:String(opts.limit||100),sort:opts.sort||"title"});if(opts.q)p.set("q",opts.q);if(opts.cursor)p.set("cursor",opts.cursor);
  return request<InventoryInsightReport>(`/v1/reports/inventory?${p.toString()}`,{},session.token);
}

export async function getCashReport(session:UserSession,date:string):Promise<CashReport>{
  if(MOCK_MODE)return getMockCashReport(date);
  return request<CashReport>(`/v1/reports/cash?date=${encodeURIComponent(date)}`,{},session.token);
}

export async function closeBusinessDay(session:UserSession,input:{businessDate:string;openingCash:number;actualCash:number;note:string}):Promise<DailyClosing>{
  if(MOCK_MODE)return closeMockBusinessDay(input);
  return request<DailyClosing>("/v1/daily-closings",{method:"POST",headers:{"Idempotency-Key":crypto.randomUUID()},body:JSON.stringify({business_date:input.businessDate,opening_cash:input.openingCash,actual_cash:input.actualCash,note:input.note})},session.token);
}


export async function getAuditLogs(session:UserSession, cursor=""):Promise<PagedResult<AuditLogEntry>>{
  if(MOCK_MODE)return{items:[],total:0,next_cursor:""};
  const p=new URLSearchParams({limit:"100"});if(cursor)p.set("cursor",cursor);
  const out=await request<PagedResult<AuditLogEntry>>(`/v1/audit-logs?${p.toString()}`,{},session.token);
  return {...out,items:out.items??[]};
}


export async function createEdgePairing(session:UserSession):Promise<EdgePairing>{
  if(MOCK_MODE)return{pair_code:"mock-pair-code",expires_at:new Date(Date.now()+10*60*1000).toISOString()};
  return request<EdgePairing>("/v1/edge/pairings",{method:"POST",body:JSON.stringify({warehouse_id:session.warehouseId})},session.token);
}
export async function getEdgeDevices(session:UserSession):Promise<EdgeDevice[]>{
  if(MOCK_MODE)return[];
  const out=await request<{items:EdgeDevice[]}>("/v1/edge/devices",{},session.token);return out.items??[];
}
export async function revokeEdgeDevice(session:UserSession,id:string):Promise<void>{
  if(MOCK_MODE)return;
  await request<{status:string}>(`/v1/edge/devices/${encodeURIComponent(id)}/revoke`,{method:"POST"},session.token);
}
