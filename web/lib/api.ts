import { addMockExpense, addMockPartyBalance, applyMockAdjustment, applyMockPurchase, applyMockPurchaseReturn, applyMockReorder, applyMockSaleReturn, applyMockSettlement, getMockCustomerBalances, getMockExpenseCategories, getMockExpenses, getMockInventory, getMockPartyStatement, getMockProfitLoss, getMockSupplierBalances, mockCustomers, mockProducts, mockPurchaseDetail, mockSaleDetail, mockSuppliers, mockNetworkResults, getMockNetworkProfile, setMockNetworkProfile, getMockStoreOffers, setMockStoreOffer, createMockReservation, getMockBuyerReservations, getMockStoreReservations, cancelMockReservation, transitionMockReservation, fulfillMockReservation, getMockDashboardSummary, getMockSalesHistory, getMockPurchasesHistory, getMockInventoryInsights, getMockCashReport, closeMockBusinessDay } from "./mock";
import type {
  Customer,
  InventoryAdjustmentResult,
  InventoryStock,
  MeResponse,
  Product,
  ProductUnit,
  ProductUnitInput,
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
  UserSession, NetworkSearchResult, NetworkStoreOffer, StoreNetworkProfile, NetworkReservation, NetworkReservationStatus, ReservationFulfillmentResult, Expense, ExpenseCategory, PartyStatement, ProfitLoss, CashReport, DailyClosing, DashboardSummary, InventoryInsightReport, PagedResult, PurchaseHistoryItem, SaleHistoryItem, NetworkProcurement, ProcurementReceiveResult, VehicleMake, ProductSearchMetadata, ProductSearchTerm, ProductFitmentInput, AuditLogEntry, ProductImportRow, ProductImportResult, EdgePairing, EdgeDevice, BankAccount, BankLedger, StoreCheck, CheckSummary, CheckDirection, CheckAction, MaturityAverageResult, FinanceIntelligenceDashboard, BankStatementInput, BankStatementImportResult, BankStatementLine, ReconciliationCandidate, ReconciliationMatch, PriceList, PricingSettings, PriceBreak, ProductPricing, PricingQuote
} from "./types";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
export const MOCK_MODE = process.env.NEXT_PUBLIC_MOCK_MODE !== "false";

export class ApiError extends Error {
  status: number;
  constructor(status:number,message:string){super(message);this.status=status;this.name="ApiError";}
}

let mockPriceLists: PriceList[] = [
  {id:"mock-retail",code:"retail",name:"خرده / مصرف‌کننده",is_default:true,active:true},
  {id:"mock-mechanic",code:"mechanic",name:"مکانیک",is_default:false,active:true},
  {id:"mock-wholesale",code:"wholesale",name:"عمده",is_default:false,active:true},
];
let mockPricingSettings:PricingSettings={min_margin_bps:1000,cashier_may_override:true};
const baseUnitFor=(p:Product):ProductUnit=>p.units?.find(u=>u.is_base)??{id:`mock-base-${p.id}`,code:p.unit||"pcs",name:p.unit==="pair"?"جفت":"عدد",factor_to_base:1,barcode:p.barcode,is_base:true,allow_sale:true,allow_purchase:true,active:true};
const unitsFor=(p:Product):ProductUnit[]=>p.units?.length?p.units:[baseUnitFor(p)];
const priceBreakKey=(productId:string,unitId?:string)=>unitId?`${productId}:${unitId}`:productId;
if(mockCustomers[0]){mockCustomers[0].price_list_id="mock-mechanic";mockCustomers[0].price_list_name="مکانیک";}
if(mockCustomers[1]){mockCustomers[1].price_list_id="mock-wholesale";mockCustomers[1].price_list_name="عمده";}
const mockPriceBreaks:Record<string,Record<string,PriceBreak[]>>={};
for(const list of mockPriceLists){
  mockPriceBreaks[list.id]={};
  for(const p of mockProducts){
    const base=p.mockPrice||0;
    const multiplier=list.code==="mechanic"?.96:list.code==="wholesale"?.92:1;
    const breaks=[{min_qty:1,unit_price:Math.round(base*multiplier)}];
    mockPriceBreaks[list.id][p.id]=breaks.map(x=>({...x}));
    mockPriceBreaks[list.id][priceBreakKey(p.id,baseUnitFor(p).id)]=breaks.map(x=>({...x}));
  }
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
    return mockProducts.filter(p => !s || [p.title,p.sku,p.brand,p.oem_code,p.barcode,...unitsFor(p).map(u=>u.barcode)].filter(Boolean).some(v => String(v).toLowerCase().includes(s))).map(p=>({...p,units:unitsFor(p)}));
  }
  const out = await request<{items: Product[]}>(`/v1/products?q=${encodeURIComponent(q)}&limit=30`, {}, session.token);
  return out.items;
}

export async function createProduct(session:UserSession,input:{sku?:string;title:string;brand?:string;oem_code?:string;barcode?:string;unit:string;base_unit_name?:string;allow_fractional_base_qty:boolean;retail_price?:number;units:ProductUnitInput[]}):Promise<Product>{
  if(MOCK_MODE){
    const id=crypto.randomUUID();
    const base:ProductUnit={id:crypto.randomUUID(),code:input.unit||"pcs",name:input.base_unit_name||"عدد",factor_to_base:1,barcode:input.barcode||undefined,is_base:true,allow_sale:true,allow_purchase:true,active:true};
    const alternates=input.units.map(u=>({id:crypto.randomUUID(),code:u.code,name:u.name,factor_to_base:u.factor_to_base,barcode:u.barcode||undefined,is_base:false,allow_sale:u.allow_sale,allow_purchase:u.allow_purchase,active:true}));
    const row:Product={id,sku:input.sku||undefined,title:input.title,brand:input.brand||undefined,oem_code:input.oem_code||undefined,barcode:input.barcode||undefined,unit:base.code,allow_fractional_base_qty:input.allow_fractional_base_qty,units:[base,...alternates],active:true,mockPrice:input.retail_price||0,mockQty:0,mockUnitCost:0};
    mockProducts.push(row);
    for(const list of mockPriceLists){mockPriceBreaks[list.id]??={};const mult=list.code==="mechanic"?.96:list.code==="wholesale"?.92:1;const basePrice=Math.round((input.retail_price||0)*mult);if(basePrice>0){mockPriceBreaks[list.id][priceBreakKey(id,base.id)]=[{min_qty:1,unit_price:basePrice}];mockPriceBreaks[list.id][id]=[{min_qty:1,unit_price:basePrice}];} input.units.forEach((u,i)=>{if((u.retail_price||0)>0)mockPriceBreaks[list.id][priceBreakKey(id,alternates[i].id)]=[{min_qty:1,unit_price:Math.round((u.retail_price||0)*mult)}]});}
    return row;
  }
  return request<Product>("/v1/products",{method:"POST",body:JSON.stringify(input)},session.token);
}

export async function getProduct(session:UserSession,productId:string):Promise<Product>{
  if(MOCK_MODE){const p=mockProducts.find(x=>x.id===productId);if(!p)throw new Error("کالا پیدا نشد");return{...p,units:unitsFor(p).map(x=>({...x}))};}
  return request<Product>(`/v1/products/${productId}`,{},session.token);
}

export async function getProductUnits(session:UserSession,productId:string):Promise<ProductUnit[]>{
  if(MOCK_MODE){const p=mockProducts.find(x=>x.id===productId);if(!p)throw new Error("کالا پیدا نشد");return unitsFor(p).map(x=>({...x}));}
  const out=await request<{items:ProductUnit[]}>(`/v1/products/${productId}/units`,{},session.token);return out.items;
}

export async function updateProductUnits(session:UserSession,productId:string,input:{allow_fractional_base_qty:boolean;base_unit_name:string;base_barcode?:string;units:ProductUnitInput[]}):Promise<ProductUnit[]>{
  if(MOCK_MODE){const p=mockProducts.find(x=>x.id===productId);if(!p)throw new Error("کالا پیدا نشد");const oldBase=baseUnitFor(p);const base={...oldBase,name:input.base_unit_name||oldBase.name,barcode:input.base_barcode||undefined};const alt=input.units.map(u=>{const old=p.units?.find(x=>!x.is_base&&x.code===u.code);return{id:old?.id||crypto.randomUUID(),code:u.code,name:u.name,factor_to_base:u.factor_to_base,barcode:u.barcode||undefined,is_base:false,allow_sale:u.allow_sale,allow_purchase:u.allow_purchase,active:true}});p.barcode=base.barcode;p.allow_fractional_base_qty=input.allow_fractional_base_qty;p.units=[base,...alt];return p.units.map(x=>({...x}));}
  const out=await request<{items:ProductUnit[]}>(`/v1/products/${productId}/units`,{method:"PUT",body:JSON.stringify(input)},session.token);return out.items;
}

export async function exportProducts(session:UserSession):Promise<Product[]>{
  if(MOCK_MODE)return mockProducts.map(p=>({...p,units:unitsFor(p).map(x=>({...x}))}));
  const out=await request<{items:Product[]}>("/v1/products/export",{},session.token);return out.items;
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

export async function getPriceLists(session:UserSession):Promise<PriceList[]>{
  if(MOCK_MODE)return mockPriceLists.map(x=>({...x}));
  const out=await request<{items:PriceList[]}>("/v1/pricing/price-lists",{},session.token);return out.items;
}
export async function createPriceList(session:UserSession,input:{code:string;name:string;is_default?:boolean}):Promise<PriceList>{
  if(MOCK_MODE){const row:PriceList={id:crypto.randomUUID(),code:input.code.trim().toLowerCase(),name:input.name.trim(),is_default:Boolean(input.is_default),active:true};if(row.is_default)mockPriceLists=mockPriceLists.map(x=>({...x,is_default:false}));mockPriceLists.push(row);mockPriceBreaks[row.id]={};return{...row};}
  return request<PriceList>("/v1/pricing/price-lists",{method:"POST",body:JSON.stringify(input)},session.token);
}
export async function updatePriceList(session:UserSession,id:string,input:{name:string;is_default:boolean;active:boolean}):Promise<PriceList>{
  if(MOCK_MODE){const i=mockPriceLists.findIndex(x=>x.id===id);if(i<0)throw new Error("لیست قیمت پیدا نشد");if(input.is_default)mockPriceLists=mockPriceLists.map(x=>({...x,is_default:false}));mockPriceLists[i]={...mockPriceLists[i],...input};return{...mockPriceLists[i]};}
  return request<PriceList>(`/v1/pricing/price-lists/${id}`,{method:"PUT",body:JSON.stringify(input)},session.token);
}
export async function getPricingSettings(session:UserSession):Promise<PricingSettings>{
  if(MOCK_MODE)return{...mockPricingSettings};return request<PricingSettings>("/v1/pricing/settings",{},session.token);
}
export async function savePricingSettings(session:UserSession,input:PricingSettings):Promise<PricingSettings>{
  if(MOCK_MODE){mockPricingSettings={...input};return{...mockPricingSettings};}return request<PricingSettings>("/v1/pricing/settings",{method:"PUT",body:JSON.stringify(input)},session.token);
}
export async function getProductPricing(session:UserSession,priceListId:string,q=""):Promise<ProductPricing[]>{
  if(MOCK_MODE){const needle=q.trim().toLowerCase();return mockProducts.filter(p=>!needle||[p.title,p.sku,p.brand,p.oem_code,p.barcode,...unitsFor(p).map(u=>u.barcode)].filter(Boolean).some(v=>String(v).toLowerCase().includes(needle))).slice(0,30).map(p=>{const units=unitsFor(p).filter(u=>u.active&&u.allow_sale).map(u=>({product_unit_id:u.id,code:u.code,name:u.name,factor_to_base:u.factor_to_base,is_base:u.is_base,breaks:(mockPriceBreaks[priceListId]?.[priceBreakKey(p.id,u.id)]||[]).map(x=>({...x}))}));return{product_id:p.id,title:p.title,sku:p.sku,brand:p.brand,breaks:units.find(u=>u.is_base)?.breaks||[],units};});}
  const out=await request<{items:ProductPricing[]}>(`/v1/pricing/products?price_list_id=${encodeURIComponent(priceListId)}&q=${encodeURIComponent(q)}&limit=30`,{},session.token);return out.items;
}
export async function replaceProductPriceBreaks(session:UserSession,priceListId:string,productId:string,breaks:PriceBreak[],productUnitId?:string):Promise<PriceBreak[]>{
  if(MOCK_MODE){mockPriceBreaks[priceListId]??={};const key=priceBreakKey(productId,productUnitId);mockPriceBreaks[priceListId][key]=breaks.map(x=>({...x})).sort((a,b)=>a.min_qty-b.min_qty);const p=mockProducts.find(x=>x.id===productId);if(p&&baseUnitFor(p).id===productUnitId)mockPriceBreaks[priceListId][productId]=mockPriceBreaks[priceListId][key].map(x=>({...x}));return mockPriceBreaks[priceListId][key].map(x=>({...x}));}
  const out=await request<{breaks:PriceBreak[]}>(`/v1/pricing/products/${productId}/breaks`,{method:"PUT",body:JSON.stringify({price_list_id:priceListId,product_unit_id:productUnitId||null,breaks})},session.token);return out.breaks;
}
export async function assignCustomerPriceList(session:UserSession,customerId:string,priceListId:string|null):Promise<Customer>{
  if(MOCK_MODE){const c=mockCustomers.find(x=>x.id===customerId);if(!c)throw new Error("مشتری پیدا نشد");const pl=mockPriceLists.find(x=>x.id===priceListId);c.price_list_id=pl?.id;c.price_list_name=pl?.name;return{...c};}
  return request<Customer>(`/v1/customers/${customerId}/price-list`,{method:"PUT",body:JSON.stringify({price_list_id:priceListId})},session.token);
}
export async function quotePricing(session:UserSession,items:{product_id:string;product_unit_id?:string;qty:number}[],customerId?:string|null):Promise<PricingQuote>{
  if(MOCK_MODE){const customer=customerId?mockCustomers.find(x=>x.id===customerId):undefined;const selected=mockPriceLists.find(x=>x.id===customer?.price_list_id&&x.active)||mockPriceLists.find(x=>x.is_default&&x.active)||mockPriceLists[0];const fallback=mockPriceLists.find(x=>x.is_default&&x.active)||selected;const rows=items.map(i=>{const p=mockProducts.find(x=>x.id===i.product_id);const unit=p?(unitsFor(p).find(u=>u.id===i.product_unit_id)||baseUnitFor(p)):undefined;const factor=unit?.factor_to_base||1;const exact=(listId:string)=>mockPriceBreaks[listId]?.[priceBreakKey(i.product_id,unit?.id)]||[];const eligible=(bs:PriceBreak[],q:number)=>bs.filter(b=>b.min_qty<=q).sort((a,b)=>b.min_qty-a.min_qty)[0];let selectedEligible=eligible(exact(selected.id),i.qty);let fallbackEligible=selected.id===fallback.id?undefined:eligible(exact(fallback.id),i.qty);let derived=false;if(!selectedEligible&&!fallbackEligible&&unit&&!unit.is_base&&p){const base=baseUnitFor(p);selectedEligible=eligible(mockPriceBreaks[selected.id]?.[priceBreakKey(i.product_id,base.id)]||[],i.qty*factor);fallbackEligible=selected.id===fallback.id?undefined:eligible(mockPriceBreaks[fallback.id]?.[priceBreakKey(i.product_id,base.id)]||[],i.qty*factor);derived=Boolean(selectedEligible||fallbackEligible);}const raw=selectedEligible?.unit_price??fallbackEligible?.unit_price??p?.mockPrice??0;const packageFallback=!selectedEligible&&!fallbackEligible&&Boolean(unit&&!unit.is_base);const price=(derived||packageFallback)?Math.round(raw*factor):raw;const source=selectedEligible?(derived?"base_unit_derived":"price_list"):fallbackEligible?(derived?"base_unit_default_derived":"default_fallback"):"network_offer_fallback";const applied=selectedEligible?selected:fallbackEligible?fallback:undefined;const cost=(p?.mockUnitCost||Math.round((p?.mockPrice||price)*.75))*factor;const min=Math.ceil(cost*10000/(10000-mockPricingSettings.min_margin_bps));return{product_id:i.product_id,product_unit_id:unit?.id||"",unit_code:unit?.code||p?.unit||"pcs",unit_name:unit?.name||"عدد",factor_to_base:factor,qty:i.qty,price_list_id:applied?.id,price_list_name:applied?.name,unit_price:price,price_source:source,min_allowed_price:min};});return{price_list_id:selected.id,price_list_name:selected.name,min_margin_bps:mockPricingSettings.min_margin_bps,cashier_may_override:mockPricingSettings.cashier_may_override,items:rows};}
  return request<PricingQuote>("/v1/pricing/quote",{method:"POST",body:JSON.stringify({warehouse_id:session.warehouseId,customer_id:customerId||null,items})},session.token);
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
      items: items.map(i => ({ product_id: i.product.id, product_unit_id: i.unit.id, qty: i.qty, unit_price: i.unitPrice, ...(i.overrideReason ? { override_reason: i.overrideReason } : {}) }))
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
    applyMockPurchase(items.map(i => ({ productId: i.product.id, qty: i.qty * i.unit.factor_to_base, unitCost: i.unit.factor_to_base > 0 ? i.unitCost / i.unit.factor_to_base : i.unitCost })));
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
      items: items.map(i => ({ product_id: i.product.id, product_unit_id: i.unit.id, qty: i.qty, unit_cost: i.unitCost }))
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
let mockStatementLines:BankStatementLine[]=[];
let mockReconciliationMatches:ReconciliationMatch[]=[];
export async function getFinanceIntelligence(session:UserSession,days=90):Promise<FinanceIntelligenceDashboard>{
  if(MOCK_MODE){const unmatched=mockStatementLines.filter(x=>x.status!=="matched");const bank=mockBankAccounts.reduce((s,x)=>s+x.balance,0);return{generated_at:new Date().toISOString(),window_days:days,bank_balance:bank,receivable_open_amount:0,payable_open_amount:0,overdue_receivable_amount:0,overdue_payable_amount:0,next_30_net:0,projected_bank_balance_30:bank,unreconciled_bank_lines:unmatched.length,unreconciled_bank_amount:unmatched.reduce((s,x)=>s+x.remaining_amount,0),maturity_buckets:[],cash_calendar:[],customer_risks:[]};}
  return request<FinanceIntelligenceDashboard>(`/v1/finance/check-intelligence?days=${encodeURIComponent(String(days))}`,{},session.token);
}
export async function getCheckMaturityAverage(session:UserSession,checkIds:string[],referenceDate?:string):Promise<MaturityAverageResult>{
  if(MOCK_MODE){const selected=mockChecks.filter(x=>checkIds.includes(x.id));if(!selected.length)throw new Error("حداقل یک چک انتخاب کن");const dir=selected[0].direction;if(selected.some(x=>x.direction!==dir))throw new Error("چک‌های دریافتی و پرداختی را جداگانه راس‌گیری کن");const base=new Date(`${referenceDate||new Date().toISOString().slice(0,10)}T00:00:00Z`);const total=selected.reduce((s,x)=>s+x.amount,0);const weighted=Math.round(selected.reduce((s,x)=>s+x.amount*Math.round((new Date(`${x.due_date}T00:00:00Z`).getTime()-base.getTime())/86400000),0)/total);const maturity=new Date(base);maturity.setUTCDate(maturity.getUTCDate()+weighted);return{direction:dir,count:selected.length,total_amount:total,reference_date:base.toISOString().slice(0,10),weighted_days:weighted,maturity_date:maturity.toISOString().slice(0,10),items:selected.map(x=>({check_id:x.id,check_number:x.check_number,amount:x.amount,due_date:x.due_date,days_from_reference:Math.round((new Date(`${x.due_date}T00:00:00Z`).getTime()-base.getTime())/86400000),weight_bps:Math.round(x.amount*10000/total)}))};}
  return request<MaturityAverageResult>("/v1/checks/maturity-average",{method:"POST",body:JSON.stringify({check_ids:checkIds,reference_date:referenceDate||""})},session.token);
}
export async function getBankStatementLines(session:UserSession,bankAccountId:string):Promise<BankStatementLine[]>{
  if(MOCK_MODE)return mockStatementLines.filter(x=>x.bank_account_id===bankAccountId).map(x=>({...x}));
  const out=await request<{items:BankStatementLine[]}>(`/v1/banking/accounts/${encodeURIComponent(bankAccountId)}/statement-lines`,{},session.token);return out.items??[];
}
export async function importBankStatement(session:UserSession,bankAccountId:string,lines:BankStatementInput[]):Promise<BankStatementImportResult>{
  if(MOCK_MODE){let imported=0,duplicates=0;for(const line of lines){if(mockStatementLines.some(x=>x.bank_account_id===bankAccountId&&x.date===line.date&&x.amount===line.amount&&x.reference===line.reference)){duplicates++;continue}mockStatementLines.unshift({id:crypto.randomUUID(),bank_account_id:bankAccountId,date:line.date,amount:line.amount,description:line.description,reference:line.reference,external_id:line.external_id,matched_amount:0,remaining_amount:Math.abs(line.amount),status:"unmatched",duplicate_suspected:false,created_at:new Date().toISOString()});imported++;}return{imported,duplicates};}
  return request<BankStatementImportResult>(`/v1/banking/accounts/${encodeURIComponent(bankAccountId)}/statement-lines/import`,{method:"POST",body:JSON.stringify({lines})},session.token);
}
export async function getReconciliationCandidates(session:UserSession,bankAccountId:string,statementLineId:string):Promise<ReconciliationCandidate[]>{
  if(MOCK_MODE){const line=mockStatementLines.find(x=>x.id===statementLineId);if(!line)return[];return[{journal_entry_id:`mock-je-${line.id}`,journal_id:`mock-j-${line.id}`,reference_type:line.amount>0?"customer_receipt":"supplier_payment",reference_id:line.id,posted_at:`${line.date}T12:00:00Z`,change:line.amount,matched_amount:0,remaining_amount:Math.abs(line.amount),exact_amount:true}];}
  const out=await request<{items:ReconciliationCandidate[]}>(`/v1/banking/accounts/${encodeURIComponent(bankAccountId)}/reconciliation-candidates?statement_line_id=${encodeURIComponent(statementLineId)}`,{},session.token);return out.items??[];
}
export async function matchBankStatementLine(session:UserSession,statementLineId:string,journalEntryId:string,amount=0,note=""):Promise<ReconciliationMatch>{
  if(MOCK_MODE){const line=mockStatementLines.find(x=>x.id===statementLineId);if(!line)throw new Error("تراکنش بانک پیدا نشد");const matched=Math.min(amount||line.remaining_amount,line.remaining_amount);line.matched_amount+=matched;line.remaining_amount=Math.max(0,Math.abs(line.amount)-line.matched_amount);line.status=line.remaining_amount===0?"matched":"partial";const row:ReconciliationMatch={id:crypto.randomUUID(),statement_line_id:line.id,journal_entry_id:journalEntryId,matched_amount:matched,note,reference_type:line.amount>0?"customer_receipt":"supplier_payment",reference_id:line.id,posted_at:`${line.date}T12:00:00Z`,created_at:new Date().toISOString()};mockReconciliationMatches.push(row);return row;}
  return request<ReconciliationMatch>(`/v1/banking/statement-lines/${encodeURIComponent(statementLineId)}/match`,{method:"POST",body:JSON.stringify({journal_entry_id:journalEntryId,amount,note})},session.token);
}
export async function getReconciliationMatches(session:UserSession,statementLineId:string):Promise<ReconciliationMatch[]>{
  if(MOCK_MODE)return mockReconciliationMatches.filter(x=>x.statement_line_id===statementLineId).map(x=>({...x}));
  const out=await request<{items:ReconciliationMatch[]}>(`/v1/banking/statement-lines/${encodeURIComponent(statementLineId)}/matches`,{},session.token);return out.items??[];
}
export async function unmatchBankStatementLine(session:UserSession,statementLineId:string,matchId:string):Promise<void>{
  if(MOCK_MODE){const i=mockReconciliationMatches.findIndex(x=>x.id===matchId&&x.statement_line_id===statementLineId);if(i<0)throw new Error("تطبیق پیدا نشد");const [m]=mockReconciliationMatches.splice(i,1);const line=mockStatementLines.find(x=>x.id===statementLineId);if(line){line.matched_amount=Math.max(0,line.matched_amount-m.matched_amount);line.remaining_amount=Math.max(0,Math.abs(line.amount)-line.matched_amount);line.status=line.matched_amount===0?"unmatched":line.remaining_amount===0?"matched":"partial";}return;}
  await request<Record<string,never>>(`/v1/banking/statement-lines/${encodeURIComponent(statementLineId)}/matches/${encodeURIComponent(matchId)}`,{method:"DELETE"},session.token);
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
