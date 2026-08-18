import type { CashReport, Customer, DailyClosing, DashboardSummary, Expense, ExpenseCategory, InventoryInsightReport, InventoryStock, NetworkReservation, NetworkReservationStatus, PartyBalance, PartyStatement, Product, ProfitLoss, PurchaseDetail, PurchaseHistoryItem, SaleDetail, SaleHistoryItem, Supplier } from "./types";

export const mockProducts: Product[] = [
  { id: "11111111-1111-1111-1111-111111111111", sku: "BRK-206-TXT", title: "لنت جلو پژو ۲۰۶ تیپ ۵", brand: "Textar", oem_code: "4254.89", unit: "pcs", active: true, mockPrice: 1780000, mockQty: 8, mockUnitCost: 1350000 },
  { id: "22222222-2222-2222-2222-222222222222", sku: "FLT-OIL-TU5", title: "فیلتر روغن TU5", brand: "سرکان", oem_code: "1109.AH", unit: "pcs", active: true, mockPrice: 285000, mockQty: 24, mockUnitCost: 210000 },
  { id: "33333333-3333-3333-3333-333333333333", sku: "PMP-EF7", title: "واترپمپ دنا EF7", brand: "ایساکو", oem_code: "IK-32011", unit: "pcs", active: true, mockPrice: 1650000, mockQty: 3, mockUnitCost: 1180000 },
  { id: "44444444-4444-4444-4444-444444444444", sku: "DSK-206", title: "دیسک ترمز جلو پژو ۲۰۶", brand: "عظام", oem_code: "4249.66", unit: "pcs", active: true, mockPrice: 3150000, mockQty: 2, mockUnitCost: 2400000 },
  { id: "55555555-5555-5555-5555-555555555555", sku: "SPK-TU5", title: "شمع موتور TU5", brand: "Bosch", unit: "pcs", active: true, mockPrice: 420000, mockQty: 16, mockUnitCost: 310000 }
];

export const mockCustomers: Customer[] = [
  { id: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", name: "تعمیرگاه احمدی", phone: "09121234567" },
  { id: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", name: "مکانیکی امیر", phone: "09123334455" },
  { id: "cccccccc-cccc-cccc-cccc-cccccccccccc", name: "مشتری نقدی" }
];

export const mockSuppliers: Supplier[] = [
  { id: "dddddddd-dddd-dddd-dddd-dddddddddddd", name: "پخش قطعات ایران", phone: "02155551010", code: "SUP-001" },
  { id: "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee", name: "بازرگانی پارس یدک", phone: "02155552020", code: "SUP-002" },
  { id: "ffffffff-ffff-ffff-ffff-ffffffffffff", name: "تأمین قطعه مرکزی", phone: "02155553030", code: "SUP-003" }
];

let mockInventory: InventoryStock[] = mockProducts.map((p, index) => ({
  product_id: p.id,
  title: p.title,
  sku: p.sku,
  on_hand: p.mockQty || 0,
  reserved: index === 0 ? 1 : 0,
  available: Math.max(0, (p.mockQty || 0) - (index === 0 ? 1 : 0)),
  avg_unit_cost: p.mockUnitCost || 0,
  min_qty: index === 2 ? 5 : index === 3 ? 3 : index === 0 ? 4 : 0,
  target_qty: index === 2 ? 12 : index === 3 ? 8 : index === 0 ? 10 : 0,
  low_stock: index === 2 || index === 3
}));

export function getMockInventory(lowOnly = false): InventoryStock[] {
  return mockInventory.filter(x => !lowOnly || x.low_stock).map(x => ({...x}));
}

export function applyMockPurchase(items: {productId: string; qty: number; unitCost: number}[]) {
  for (const item of items) {
    const stock = mockInventory.find(x => x.product_id === item.productId);
    const product = mockProducts.find(x => x.id === item.productId);
    if (stock) {
      const oldValue = stock.on_hand * stock.avg_unit_cost;
      const newValue = item.qty * item.unitCost;
      const newOnHand = stock.on_hand + item.qty;
      stock.avg_unit_cost = newOnHand > 0 ? Math.round((oldValue + newValue) / newOnHand) : item.unitCost;
      stock.on_hand = newOnHand;
      stock.available = stock.on_hand - stock.reserved;
      stock.low_stock = stock.min_qty > 0 && stock.available <= stock.min_qty;
    } else if (product) {
      mockInventory.push({
        product_id: product.id,
        title: product.title,
        sku: product.sku,
        on_hand: item.qty,
        reserved: 0,
        available: item.qty,
        avg_unit_cost: item.unitCost,
        min_qty: 0,
        target_qty: 0,
        low_stock: false
      });
    }
  }
}

export function applyMockAdjustment(productId: string, qtyDelta: number) {
  const stock = mockInventory.find(x => x.product_id === productId);
  if (!stock) throw new Error("کالا در موجودی یافت نشد.");
  const next = stock.on_hand + qtyDelta;
  if (next < stock.reserved) throw new Error("موجودی جدید نمی‌تواند از تعداد رزرو شده کمتر باشد.");
  stock.on_hand = next;
  stock.available = stock.on_hand - stock.reserved;
  stock.low_stock = stock.min_qty > 0 && stock.available <= stock.min_qty;
}

export function applyMockReorder(productId: string, minQty: number, targetQty: number) {
  const stock = mockInventory.find(x => x.product_id === productId);
  if (!stock) throw new Error("کالا در موجودی یافت نشد.");
  stock.min_qty = minQty;
  stock.target_qty = targetQty;
  stock.low_stock = minQty > 0 && stock.available <= minQty;
}

export const mockDashboard = {
  sales: 87400000,
  purchases: 38100000,
  profit: 14200000,
  receivables: 124000000,
  payables: 82000000,
  lowStock: [
    { title: "دیسک ترمز جلو ۲۰۶", qty: 2 },
    { title: "واترپمپ EF7", qty: 3 },
    { title: "لنت جلو ۲۰۶", qty: 4 }
  ],
  recentSales: [
    { no: "S-1042", customer: "تعمیرگاه احمدی", amount: 3560000, method: "کارت", time: "۱۰:۳۲" },
    { no: "S-1041", customer: "مشتری نقدی", amount: 840000, method: "نقد", time: "۱۰:۱۴" },
    { no: "S-1040", customer: "مکانیکی امیر", amount: 3150000, method: "نسیه", time: "۰۹:۵۸" }
  ]
};


let mockCustomerBalances: PartyBalance[] = [
  { id: mockCustomers[0].id, name: mockCustomers[0].name, phone: mockCustomers[0].phone, balance: 12400000 },
  { id: mockCustomers[1].id, name: mockCustomers[1].name, phone: mockCustomers[1].phone, balance: 6850000 },
  { id: mockCustomers[2].id, name: mockCustomers[2].name, balance: 0 }
];
let mockSupplierBalances: PartyBalance[] = [
  { id: mockSuppliers[0].id, code: mockSuppliers[0].code, name: mockSuppliers[0].name, phone: mockSuppliers[0].phone, balance: 28600000 },
  { id: mockSuppliers[1].id, code: mockSuppliers[1].code, name: mockSuppliers[1].name, phone: mockSuppliers[1].phone, balance: 17400000 },
  { id: mockSuppliers[2].id, code: mockSuppliers[2].code, name: mockSuppliers[2].name, phone: mockSuppliers[2].phone, balance: 0 }
];
export function getMockCustomerBalances(){ return mockCustomerBalances.map(x=>({...x})); }
export function getMockSupplierBalances(){ return mockSupplierBalances.map(x=>({...x})); }
export function applyMockSettlement(type: "customer"|"supplier", id: string, amount: number){
  const list=type==="customer"?mockCustomerBalances:mockSupplierBalances; const row=list.find(x=>x.id===id);
  if(!row) throw new Error("حساب پیدا نشد."); if(amount<=0||amount>row.balance) throw new Error("مبلغ تسویه معتبر نیست."); row.balance-=amount; return row.balance;
}


export const mockSaleDetail: SaleDetail = {
  id: "99999999-9999-9999-9999-999999999991", customer_id: mockCustomers[0].id, customer_name: mockCustomers[0].name,
  warehouse_id: "33333333-3333-3333-3333-333333333333", gross_amount: 3800000, discount_amount: 240000, total_amount: 3560000, paid_amount: 2000000, due_amount: 1560000, status: "posted", created_at: new Date().toISOString(),
  items: [{ id:"91919191-9191-9191-9191-919191919191", product_id:mockProducts[0].id, title:mockProducts[0].title, qty:2, returned_qty:0, returnable_qty:2, unit_price:1780000, unit_cost:1350000, line_total:3560000, gross_line_total:3800000, discount_amount:240000, list_unit_price:1900000, price_source:"price_list", price_override:true, override_reason:"تخفیف مشتری" }]
};
export const mockPurchaseDetail: PurchaseDetail = {
  id: "99999999-9999-9999-9999-999999999992", supplier_id:mockSuppliers[0].id, supplier_name:mockSuppliers[0].name,
  warehouse_id: "33333333-3333-3333-3333-333333333333", total_amount: 4200000, paid_amount: 1200000, due_amount: 3000000, status: "posted", created_at:new Date().toISOString(),
  items:[{ id:"92929292-9292-9292-9292-929292929292", product_id:mockProducts[1].id, title:mockProducts[1].title, qty:20, returned_qty:0, returnable_qty:20, unit_cost:210000, line_total:4200000 }]
};
export function applyMockSaleReturn(qty:number){ const line=mockSaleDetail.items[0]; if(qty<=0||qty>line.returnable_qty) throw new Error("تعداد مرجوعی معتبر نیست."); line.returned_qty+=qty; line.returnable_qty-=qty; applyMockPurchase([{productId:line.product_id,qty,unitCost:line.unit_cost}]); return Math.round(qty*(line.unit_price||0)); }
export function applyMockPurchaseReturn(qty:number){ const line=mockPurchaseDetail.items[0]; if(qty<=0||qty>line.returnable_qty) throw new Error("تعداد مرجوعی معتبر نیست."); const stock=mockInventory.find(x=>x.product_id===line.product_id); if(!stock||stock.available<qty) throw new Error("موجودی کافی برای مرجوعی خرید نیست."); line.returned_qty+=qty; line.returnable_qty-=qty; applyMockAdjustment(line.product_id,-qty); return Math.round(qty*line.unit_cost); }
export function addMockPartyBalance(type:"customer"|"supplier", id:string, delta:number){
  const list=type==="customer"?mockCustomerBalances:mockSupplierBalances;
  const row=list.find(x=>x.id===id); if(!row) return; row.balance+=delta;
}

export const mockNetworkResults = [
  { offer_id:"n1", product_id:mockProducts[0].id, title:"لنت جلو پژو ۲۰۶ تیپ ۵", sku:"BRK-206-TXT", brand:"Textar", oem_code:"206-FB-01", store_id:"s1", store_name:"یدکی رضایی", city:"تهران", address:"خیابان آزادی، نبش بهبودی", phone:"02166000001", selling_price:1780000, available:11, allow_reservation:true, last_updated_at:new Date().toISOString(), freshness:"live" as const, distance_km:0.8 },
  { offer_id:"n2", product_id:"n-product-2", title:"لنت جلو پژو ۲۰۶ تیپ ۵", sku:"BRK-206-TXT", brand:"Textar", oem_code:"206-FB-01", store_id:"s2", store_name:"یدکی مرکزی", city:"تهران", address:"خیابان جمهوری، بعد از اسکندری", phone:"02166990002", selling_price:1720000, available:7, allow_reservation:false, last_updated_at:new Date().toISOString(), freshness:"live" as const, distance_km:2.4 },
  { offer_id:"n3", product_id:"n-product-3", title:"لنت جلو پژو ۲۰۶ تیپ ۵", sku:"BRK-206-TXT", brand:"Textar", oem_code:"206-FB-01", store_id:"s3", store_name:"پارس یدک", city:"تهران", address:"ستارخان، خیابان حبیب‌الله", phone:"02166550003", selling_price:1750000, available:16, allow_reservation:true, last_updated_at:new Date().toISOString(), freshness:"live" as const, distance_km:1.6 },
  { offer_id:"n4", product_id:mockProducts[2].id, title:"واترپمپ EF7", sku:"WP-EF7-ISK", brand:"ایساکو", oem_code:"EF7-WP-03", store_id:"s1", store_name:"یدکی رضایی", city:"تهران", address:"خیابان آزادی، نبش بهبودی", phone:"02166000001", selling_price:1590000, available:7, allow_reservation:true, last_updated_at:new Date().toISOString(), freshness:"live" as const, distance_km:0.8 },
];

let mockNetworkProfile = { store_id:"s1", store_name:"یدکی رضایی", network_enabled:true, address:"خیابان آزادی، نبش بهبودی", phone:"02166000001", city:"تهران", latitude:35.7007, longitude:51.3716 };
let mockStoreOffers = mockInventory.map((x,i)=>({ product_id:x.product_id,title:x.title,sku:x.sku,brand:mockProducts.find(p=>p.id===x.product_id)?.brand,on_hand:x.on_hand,reserved:x.reserved,available:x.available,selling_price:mockProducts.find(p=>p.id===x.product_id)?.mockPrice||0,visible:i<3,allow_reservation:i===0,last_verified_at:new Date().toISOString() }));
export function getMockNetworkProfile(){ return {...mockNetworkProfile}; }
export function setMockNetworkProfile(v:typeof mockNetworkProfile){ mockNetworkProfile={...v}; }
export function getMockStoreOffers(){ return mockStoreOffers.map(x=>({...x})); }
export function setMockStoreOffer(productId:string, sellingPrice:number, visible:boolean, allowReservation:boolean){ const row=mockStoreOffers.find(x=>x.product_id===productId); if(!row)throw new Error("کالا پیدا نشد"); row.selling_price=sellingPrice;row.visible=visible;row.allow_reservation=allowReservation;row.last_verified_at=new Date().toISOString(); }


let mockReservations: NetworkReservation[] = [];
export function createMockReservation(offerId:string,qty:number):NetworkReservation{
  const offer=mockNetworkResults.find(x=>x.offer_id===offerId); if(!offer)throw new Error("پیشنهاد پیدا نشد.");
  if(!offer.allow_reservation)throw new Error("این فروشگاه رزرو را فعال نکرده است.");
  if(qty<=0||qty>offer.available)throw new Error("موجودی کافی برای رزرو وجود ندارد.");
  offer.available-=qty;
  const r:NetworkReservation={id:crypto.randomUUID(),offer_id:offer.offer_id,product_id:offer.product_id,product_title:offer.title,store_id:offer.store_id,store_name:offer.store_name,address:offer.address,phone:offer.phone,buyer_user_id:"88888888-8888-8888-8888-888888888888",buyer_name:"مهدی مکانیک",buyer_email:"mechanic@example.com",qty,unit_price:offer.selling_price,total_amount:Math.round(qty*offer.selling_price),status:"pending",expires_at:new Date(Date.now()+30*60*1000).toISOString(),created_at:new Date().toISOString(),updated_at:new Date().toISOString()};
  mockReservations.unshift(r); return {...r};
}
export function getMockBuyerReservations(){return mockReservations.map(x=>({...x}));}
export function getMockStoreReservations(){return mockReservations.map(x=>({...x}));}
function releaseMockReservation(r:NetworkReservation){const offer=mockNetworkResults.find(x=>x.offer_id===r.offer_id);if(offer)offer.available+=r.qty;}
export function cancelMockReservation(id:string){const r=mockReservations.find(x=>x.id===id);if(!r)throw new Error("رزرو پیدا نشد.");if(!["pending","accepted"].includes(r.status))throw new Error("این رزرو قابل لغو نیست.");releaseMockReservation(r);r.status="cancelled";r.updated_at=new Date().toISOString();return {...r};}
export function transitionMockReservation(id:string,status:NetworkReservationStatus){const r=mockReservations.find(x=>x.id===id);if(!r)throw new Error("رزرو پیدا نشد.");const ok=(status==="accepted"&&r.status==="pending")||(status==="ready"&&r.status==="accepted")||(status==="rejected"&&["pending","accepted"].includes(r.status));if(!ok)throw new Error("تغییر وضعیت مجاز نیست.");if(status==="rejected")releaseMockReservation(r);r.status=status;r.updated_at=new Date().toISOString();return {...r};}
export function fulfillMockReservation(id:string){const r=mockReservations.find(x=>x.id===id);if(!r)throw new Error("رزرو پیدا نشد.");if(r.status==="fulfilled")return{reservation_id:r.id,sale_id:`mock-sale-${r.id}`,total_amount:r.total_amount,paid_amount:r.total_amount,due_amount:0,status:"posted"};if(r.status!=="ready")throw new Error("رزرو باید آماده تحویل باشد.");r.status="fulfilled";r.updated_at=new Date().toISOString();return{reservation_id:r.id,sale_id:`mock-sale-${r.id}`,total_amount:r.total_amount,paid_amount:r.total_amount,due_amount:0,status:"posted"};}

export const mockExpenseCategories: ExpenseCategory[] = [
  {id:"e1000000-0000-0000-0000-000000000001",code:"RENT",name:"اجاره"},
  {id:"e1000000-0000-0000-0000-000000000002",code:"PAYROLL",name:"حقوق و دستمزد"},
  {id:"e1000000-0000-0000-0000-000000000003",code:"UTILITIES",name:"آب، برق، گاز و اینترنت"},
  {id:"e1000000-0000-0000-0000-000000000004",code:"TRANSPORT",name:"حمل و رفت‌وآمد"},
  {id:"e1000000-0000-0000-0000-000000000005",code:"SUPPLIES",name:"ملزومات فروشگاه"},
  {id:"e1000000-0000-0000-0000-000000000006",code:"MARKETING",name:"تبلیغات و بازاریابی"},
  {id:"e1000000-0000-0000-0000-000000000007",code:"OTHER",name:"سایر هزینه‌ها"}
];
let mockExpenses: Expense[] = [
  {id:"e2000000-0000-0000-0000-000000000001",category_id:mockExpenseCategories[0].id,category_code:"RENT",category_name:"اجاره",method:"card",amount:8500000,note:"اجاره ماه جاری",occurred_on:new Date().toISOString().slice(0,10),created_at:new Date().toISOString(),status:"posted"},
  {id:"e2000000-0000-0000-0000-000000000002",category_id:mockExpenseCategories[4].id,category_code:"SUPPLIES",category_name:"ملزومات فروشگاه",method:"cash",amount:980000,note:"بسته‌بندی و مصرفی",occurred_on:new Date().toISOString().slice(0,10),created_at:new Date().toISOString(),status:"posted"}
];
export function getMockExpenseCategories(){return mockExpenseCategories.map(x=>({...x}));}
export function getMockExpenses(){return mockExpenses.map(x=>({...x}));}
export function addMockExpense(categoryId:string,amount:number,method:"cash"|"card",note:string,occurredOn:string){
  const category=mockExpenseCategories.find(x=>x.id===categoryId);if(!category)throw new Error("دسته هزینه پیدا نشد.");if(amount<=0)throw new Error("مبلغ هزینه معتبر نیست.");
  const x:Expense={id:crypto.randomUUID(),category_id:category.id,category_code:category.code,category_name:category.name,method,amount,note,occurred_on:occurredOn,created_at:new Date().toISOString(),status:"posted"};mockExpenses.unshift(x);return {...x};
}
export function getMockProfitLoss(from:string,to:string):ProfitLoss{
  const operating=mockExpenses.reduce((sum,x)=>sum+x.amount,0);const grossSales=124500000;const salesReturns=4200000;const cogs=88700000;const cogsReversed=2950000;const netSales=grossSales-salesReturns;const netCogs=cogs-cogsReversed;const grossProfit=netSales-netCogs;
  const expense_breakdown=mockExpenseCategories.map(c=>({category_id:c.id,category_code:c.code,category_name:c.name,amount:mockExpenses.filter(x=>x.category_id===c.id).reduce((sum,x)=>sum+x.amount,0)})).filter(x=>x.amount>0).sort((a,b)=>b.amount-a.amount);
  return{from,to,gross_sales:grossSales,sales_returns:salesReturns,net_sales:netSales,cogs,cogs_reversed:cogsReversed,net_cogs:netCogs,gross_profit:grossProfit,operating_expenses:operating,net_profit:grossProfit-operating,expense_breakdown};
}
export function getMockPartyStatement(type:"customer"|"supplier",id:string):PartyStatement{
  const src=type==="customer"?mockCustomerBalances:mockSupplierBalances;const row=src.find(x=>x.id===id);if(!row)throw new Error("حساب پیدا نشد.");
  const first=Math.max(0,Math.round(row.balance*.65));const second=Math.max(0,row.balance-first);const now=Date.now();
  return{party_type:type,party_id:id,party_name:row.name,closing_balance:row.balance,items:row.balance===0?[]:[
    {id:crypto.randomUUID(),entry_type:type==="customer"?"sale":"purchase",reference_id:crypto.randomUUID(),debit:type==="customer"?first:0,credit:type==="supplier"?first:0,change:first,balance:first,created_at:new Date(now-86400000*8).toISOString()},
    {id:crypto.randomUUID(),entry_type:type==="customer"?"sale":"purchase",reference_id:crypto.randomUUID(),debit:type==="customer"?second:0,credit:type==="supplier"?second:0,change:second,balance:row.balance,created_at:new Date(now-86400000*2).toISOString()}
  ]};
}

const todayISO=()=>new Date().toISOString().slice(0,10);
const mockSalesHistory:SaleHistoryItem[]=[
  {id:mockSaleDetail.id,customer_id:mockCustomers[0].id,customer_name:mockCustomers[0].name,gross_amount:3800000,discount_amount:240000,total_amount:3560000,paid_amount:2000000,due_amount:1560000,status:"posted",created_at:new Date().toISOString(),line_count:1,total_qty:2,below_margin_count:1,network_source:false},
  {id:"99999999-9999-9999-9999-999999999993",customer_id:mockCustomers[2].id,customer_name:mockCustomers[2].name,gross_amount:900000,discount_amount:60000,total_amount:840000,paid_amount:840000,due_amount:0,status:"posted",created_at:new Date(Date.now()-3600_000).toISOString(),line_count:1,total_qty:2,below_margin_count:0,network_source:false},
  {id:"99999999-9999-9999-9999-999999999994",customer_id:mockCustomers[1].id,customer_name:mockCustomers[1].name,gross_amount:3150000,discount_amount:0,total_amount:3150000,paid_amount:3150000,due_amount:0,status:"posted",created_at:new Date(Date.now()-7200_000).toISOString(),line_count:1,total_qty:1,below_margin_count:0,network_source:true}
];
const mockPurchasesHistory:PurchaseHistoryItem[]=[
  {id:mockPurchaseDetail.id,supplier_id:mockSuppliers[0].id,supplier_name:mockSuppliers[0].name,total_amount:4200000,paid_amount:1200000,due_amount:3000000,status:"posted",created_at:new Date().toISOString(),line_count:1,total_qty:20},
  {id:"99999999-9999-9999-9999-999999999995",supplier_id:mockSuppliers[1].id,supplier_name:mockSuppliers[1].name,total_amount:12800000,paid_amount:12800000,due_amount:0,status:"posted",created_at:new Date(Date.now()-86400_000).toISOString(),line_count:3,total_qty:14}
];
let mockDailyClosing:DailyClosing|undefined;

export function getMockSalesHistory(q="",paymentState="all"){const needle=q.trim().toLowerCase();return mockSalesHistory.filter(x=>(!needle||x.id.toLowerCase().includes(needle)||(x.customer_name||"").toLowerCase().includes(needle))&&(paymentState==="all"||!paymentState||(paymentState==="paid"?x.due_amount===0:x.due_amount>0))).map(x=>({...x}));}
export function getMockPurchasesHistory(q="",paymentState="all"){const needle=q.trim().toLowerCase();return mockPurchasesHistory.filter(x=>(!needle||x.id.toLowerCase().includes(needle)||x.supplier_name.toLowerCase().includes(needle))&&(paymentState==="all"||!paymentState||(paymentState==="paid"?x.due_amount===0:x.due_amount>0))).map(x=>({...x}));}
export function getMockDashboardSummary():DashboardSummary{
  const salesLastSevenDays=Array.from({length:7},(_,i)=>{const d=new Date();d.setDate(d.getDate()-(6-i));return{date:d.toISOString().slice(0,10),amount:[4200000,6800000,5100000,9200000,7400000,11200000,8740000][i]};});
  return{sales_today:8740000,sales_returns_today:420000,net_sales_today:8320000,gross_profit_today:2140000,purchases_today:3810000,receivables:mockCustomerBalances.reduce((s,x)=>s+Math.max(0,x.balance),0),payables:mockSupplierBalances.reduce((s,x)=>s+Math.max(0,x.balance),0),inventory_value:mockInventory.reduce((s,x)=>s+x.on_hand*x.avg_unit_cost,0),open_reservations:mockReservations.filter(x=>["pending","accepted","ready"].includes(x.status)).length,low_stock_count:mockInventory.filter(x=>x.low_stock).length,open_buying_procurements:1,open_selling_procurements:2,network_enabled:true,published_offers:4,network_requests_30d:17,network_sales_count_30d:6,network_sales_30d:10460000,recent_sales:getMockSalesHistory().slice(0,5),sales_last_seven_days:salesLastSevenDays};
}
export function getMockInventoryInsights(q="",sort="title"):InventoryInsightReport{
  const needle=q.trim().toLowerCase();let items=mockInventory.map((x,i)=>({product_id:x.product_id,title:x.title,sku:x.sku,brand:mockProducts.find(p=>p.id===x.product_id)?.brand,on_hand:x.on_hand,reserved:x.reserved,available:x.available,avg_unit_cost:x.avg_unit_cost,inventory_value:x.on_hand*x.avg_unit_cost,min_qty:x.min_qty,target_qty:x.target_qty,low_stock:x.low_stock,sold_qty_30d:[16,31,2,0,23][i]||0,last_sale_at:i===3?undefined:new Date(Date.now()-86400_000*(i+1)*4).toISOString(),days_since_sale:i===3?undefined:(i+1)*4,dead_stock:i===3})).filter(x=>!needle||x.title.toLowerCase().includes(needle)||(x.sku||"").toLowerCase().includes(needle)||(x.brand||"").toLowerCase().includes(needle));
  if(sort==="value")items=[...items].sort((a,b)=>b.inventory_value-a.inventory_value);else if(sort==="low_stock")items=[...items].sort((a,b)=>Number(b.low_stock)-Number(a.low_stock)||a.available-b.available);else if(sort==="sold_qty")items=[...items].sort((a,b)=>b.sold_qty_30d-a.sold_qty_30d);else if(sort==="dead_stock")items=[...items].sort((a,b)=>Number(b.dead_stock)-Number(a.dead_stock)||b.inventory_value-a.inventory_value);else items=[...items].sort((a,b)=>a.title.localeCompare(b.title,"fa"));
  const all=mockInventory.map((x,i)=>({x,dead:i===3}));return{summary:{sku_count:mockInventory.length,on_hand:mockInventory.reduce((s,x)=>s+x.on_hand,0),reserved:mockInventory.reduce((s,x)=>s+x.reserved,0),inventory_value:mockInventory.reduce((s,x)=>s+x.on_hand*x.avg_unit_cost,0),low_stock_count:mockInventory.filter(x=>x.low_stock).length,dead_stock_count:all.filter(x=>x.dead).length},items,total:items.length,next_cursor:""};
}
export function getMockCashReport(date=todayISO()):CashReport{const sale=3400000,receipt=1600000,purchaseReturn=250000,purchase=1250000,supplier=900000,expense=980000,saleReturn=180000;const cashIn=sale+receipt+purchaseReturn,cashOut=purchase+supplier+expense+saleReturn;return{business_date:date,sale_cash_in:sale,customer_receipt_cash_in:receipt,purchase_return_cash_in:purchaseReturn,cash_in:cashIn,purchase_cash_out:purchase,supplier_payment_cash_out:supplier,expense_cash_out:expense,sale_return_cash_out:saleReturn,cash_out:cashOut,net_cash_movement:cashIn-cashOut,card_in:7800000,card_out:3600000,net_card_movement:4200000,closing:mockDailyClosing?.business_date===date?{...mockDailyClosing}:undefined,changed_after_close:false};}
export function closeMockBusinessDay(input:{businessDate:string;openingCash:number;actualCash:number;note:string}):DailyClosing{if(mockDailyClosing?.business_date===input.businessDate)return{...mockDailyClosing};const flow=getMockCashReport(input.businessDate);const expected=input.openingCash+flow.cash_in-flow.cash_out;mockDailyClosing={id:crypto.randomUUID(),business_date:input.businessDate,opening_cash:input.openingCash,cash_in:flow.cash_in,cash_out:flow.cash_out,expected_cash:expected,actual_cash:input.actualCash,variance:input.actualCash-expected,closed_by_user_id:"11111111-aaaa-bbbb-cccc-111111111111",note:input.note,created_at:new Date().toISOString()};return{...mockDailyClosing};}
