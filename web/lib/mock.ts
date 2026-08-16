import type { Customer, InventoryStock, NetworkReservation, NetworkReservationStatus, PartyBalance, Product, PurchaseDetail, SaleDetail, Supplier } from "./types";

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
  warehouse_id: "33333333-3333-3333-3333-333333333333", total_amount: 3560000, paid_amount: 2000000, due_amount: 1560000, status: "posted", created_at: new Date().toISOString(),
  items: [{ id:"91919191-9191-9191-9191-919191919191", product_id:mockProducts[0].id, title:mockProducts[0].title, qty:2, returned_qty:0, returnable_qty:2, unit_price:1780000, unit_cost:1350000, line_total:3560000 }]
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
