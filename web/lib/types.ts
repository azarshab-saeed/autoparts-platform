export type UserSession = {
  token: string;
  displayName: string;
  email: string;
  role: string;
  roles: string[];
  storeName: string;
  storeId: string;
  warehouseId: string;
};

export type MeResponse = {
  user_id: string;
  tenant_id?: string;
  store_id?: string;
  role: string;
  roles?: string[];
  display_name?: string;
  email?: string;
  store_name?: string;
  default_warehouse_id?: string;
};

export type Product = {
  id: string;
  sku?: string;
  title: string;
  brand?: string;
  oem_code?: string;
  barcode?: string;
  unit: string;
  active: boolean;
  mockPrice?: number;
  mockQty?: number;
  mockUnitCost?: number;
};

export type ProductImportRow = {
  row_number: number;
  sku?: string;
  title: string;
  brand?: string;
  oem_code?: string;
  barcode?: string;
  unit?: string;
  on_hand: number;
  avg_unit_cost: number;
  selling_price: number;
  visible: boolean;
  allow_reservation: boolean;
  allow_procurement: boolean;
};

export type ProductImportResult = {
  batch_id: string;
  row_count: number;
  created_count: number;
  updated_count: number;
  inventory_initialized_count: number;
  inventory_preserved_count: number;
  offers_upserted_count: number;
  opening_inventory_value: number;
  rows: { row_number: number; product_id: string; product_action: "created"|"updated"; inventory_action: "initialized"|"preserved"|"none"; offer_action: "upserted"|"none"; note?: string }[];
};

export type Customer = { id: string; name: string; phone?: string; code?: string };
export type Supplier = { id: string; name: string; phone?: string; code?: string; notes?: string };

export type InventoryStock = {
  product_id: string;
  title: string;
  sku?: string;
  on_hand: number;
  reserved: number;
  available: number;
  avg_unit_cost: number;
  min_qty: number;
  target_qty: number;
  low_stock: boolean;
};

export type SaleItem = { product: Product; qty: number; unitPrice: number };
export type PurchaseItem = { product: Product; qty: number; unitCost: number };

export type PurchaseResult = {
  id: string;
  total_amount: number;
  paid_amount?: number;
  due_amount?: number;
  status: string;
};

export type InventoryAdjustmentResult = {
  id: string;
  status: string;
};

export type PaymentPart = { method: "cash"|"card"; amount: number };
export type PartyBalance = { id: string; code?: string; name: string; phone?: string; balance: number };
export type SettlementResult = { id: string; party_type: "customer"|"supplier"; method: "cash"|"card"; amount: number; balance: number; status: string };
export type ReturnableLine = {
  id: string;
  product_id: string;
  title: string;
  qty: number;
  returned_qty: number;
  returnable_qty: number;
  unit_price?: number;
  unit_cost: number;
  line_total: number;
};
export type SaleDetail = {
  id: string; customer_id?: string; customer_name?: string; warehouse_id: string;
  total_amount: number; paid_amount: number; due_amount: number; status: string; created_at: string; items: ReturnableLine[];
};
export type PurchaseDetail = {
  id: string; supplier_id: string; supplier_name: string; warehouse_id: string;
  total_amount: number; paid_amount: number; due_amount: number; status: string; created_at: string; items: ReturnableLine[];
};
export type ReturnResult = { id: string; total_amount: number; status: string };

export type NetworkSearchResult = {
  offer_id: string;
  product_id: string;
  title: string;
  sku?: string;
  brand?: string;
  oem_code?: string;
  store_id: string;
  store_name: string;
  city?: string;
  address?: string;
  phone?: string;
  selling_price: number;
  available: number;
  allow_reservation: boolean;
  allow_procurement?: boolean;
  last_updated_at: string;
  freshness: "live" | "recent" | "stale";
  distance_km?: number;
  fitment_match?: boolean;
  fitment_summary?: string;
  match_reason?: "exact_code" | "exact_alias" | "title" | "keyword" | "vehicle_fitment";
  score?: number;
};


export type VehicleVariant = { id: string; name: string; engine_code?: string; year_from?: number; year_to?: number };
export type VehicleModel = { id: string; name: string; variants: VehicleVariant[] };
export type VehicleMake = { id: string; name: string; models: VehicleModel[] };
export type ProductSearchTerm = { kind: "alias" | "oem" | "equivalent"; term: string };
export type ProductFitment = { vehicle_variant_id: string; make_name: string; model_name: string; variant_name: string; engine_code?: string; year_from?: number; year_to?: number; notes?: string };
export type ProductSearchMetadata = { product_id: string; terms: ProductSearchTerm[]; fitments: ProductFitment[] };
export type ProductFitmentInput = { vehicle_variant_id: string; year_from?: number; year_to?: number; notes?: string };

export type NetworkStoreOffer = {
  product_id: string;
  title: string;
  sku?: string;
  brand?: string;
  on_hand: number;
  reserved: number;
  available: number;
  selling_price: number;
  visible: boolean;
  allow_reservation: boolean;
  allow_procurement?: boolean;
  last_verified_at: string;
};

export type StoreNetworkProfile = {
  store_id: string;
  store_name: string;
  network_enabled: boolean;
  address?: string;
  phone?: string;
  city?: string;
  latitude?: number;
  longitude?: number;
};


export type NetworkProcurementStatus = "requested" | "accepted" | "ready" | "received" | "rejected" | "cancelled" | "expired";
export type NetworkProcurement = {
  id: string;
  buyer_store_id: string;
  buyer_store_name: string;
  buyer_warehouse_id: string;
  buyer_product_id: string;
  buyer_product_title: string;
  seller_store_id: string;
  seller_store_name: string;
  seller_warehouse_id: string;
  seller_product_id: string;
  seller_product_title: string;
  offer_id: string;
  qty: number;
  unit_price: number;
  total_amount: number;
  status: NetworkProcurementStatus;
  expires_at: string;
  seller_sale_id?: string;
  buyer_purchase_id?: string;
  created_at: string;
  updated_at: string;
};
export type ProcurementReceiveResult = {
  procurement_id: string;
  seller_sale_id: string;
  buyer_purchase_id: string;
  total_amount: number;
  status: NetworkProcurementStatus;
};

export type NetworkReservationStatus = "pending" | "accepted" | "ready" | "fulfilled" | "rejected" | "cancelled" | "expired";
export type ReservationFulfillmentResult = {
  reservation_id: string;
  sale_id: string;
  total_amount: number;
  paid_amount: number;
  due_amount: number;
  status: string;
};
export type NetworkReservation = {
  id: string;
  offer_id: string;
  product_id: string;
  product_title: string;
  store_id: string;
  store_name: string;
  address?: string;
  phone?: string;
  buyer_user_id?: string;
  buyer_name?: string;
  buyer_email?: string;
  qty: number;
  unit_price: number;
  total_amount: number;
  status: NetworkReservationStatus;
  expires_at: string;
  created_at: string;
  updated_at: string;
};

export type ExpenseCategory = { id: string; code: string; name: string };
export type Expense = {
  id: string;
  category_id: string;
  category_code: string;
  category_name: string;
  method: "cash" | "card";
  amount: number;
  note?: string;
  occurred_on: string;
  created_at: string;
  status: string;
};
export type ExpenseCategoryTotal = {
  category_id: string;
  category_code: string;
  category_name: string;
  amount: number;
};
export type ProfitLoss = {
  from: string;
  to: string;
  gross_sales: number;
  sales_returns: number;
  net_sales: number;
  cogs: number;
  cogs_reversed: number;
  net_cogs: number;
  gross_profit: number;
  operating_expenses: number;
  net_profit: number;
  expense_breakdown: ExpenseCategoryTotal[];
};
export type PartyStatementLine = {
  id: string;
  entry_type: string;
  reference_id: string;
  debit: number;
  credit: number;
  change: number;
  balance: number;
  created_at: string;
};
export type PartyStatement = {
  party_type: "customer" | "supplier";
  party_id: string;
  party_name: string;
  closing_balance: number;
  items: PartyStatementLine[];
};

export type PagedResult<T> = {
  items: T[];
  total: number;
  next_cursor: string;
};

export type SaleHistoryItem = {
  id: string;
  customer_id?: string;
  customer_name?: string;
  total_amount: number;
  paid_amount: number;
  due_amount: number;
  status: string;
  created_at: string;
  line_count: number;
  total_qty: number;
  network_source: boolean;
};

export type PurchaseHistoryItem = {
  id: string;
  supplier_id: string;
  supplier_name: string;
  total_amount: number;
  paid_amount: number;
  due_amount: number;
  status: string;
  created_at: string;
  line_count: number;
  total_qty: number;
};

export type DashboardDailyAmount = { date: string; amount: number };
export type DashboardSummary = {
  sales_today: number;
  sales_returns_today: number;
  net_sales_today: number;
  gross_profit_today: number;
  purchases_today: number;
  receivables: number;
  payables: number;
  inventory_value: number;
  open_reservations: number;
  low_stock_count: number;
  open_buying_procurements: number;
  open_selling_procurements: number;
  network_enabled: boolean;
  published_offers: number;
  network_requests_30d: number;
  network_sales_count_30d: number;
  network_sales_30d: number;
  recent_sales: SaleHistoryItem[];
  sales_last_seven_days: DashboardDailyAmount[];
};

export type InventoryInsightItem = {
  product_id: string;
  title: string;
  sku?: string;
  brand?: string;
  on_hand: number;
  reserved: number;
  available: number;
  avg_unit_cost: number;
  inventory_value: number;
  min_qty: number;
  target_qty: number;
  low_stock: boolean;
  sold_qty_30d: number;
  last_sale_at?: string;
  days_since_sale?: number;
  dead_stock: boolean;
};

export type InventoryInsightSummary = {
  sku_count: number;
  on_hand: number;
  reserved: number;
  inventory_value: number;
  low_stock_count: number;
  dead_stock_count: number;
};

export type InventoryInsightReport = {
  summary: InventoryInsightSummary;
  items: InventoryInsightItem[];
  total: number;
  next_cursor: string;
};

export type DailyClosing = {
  id: string;
  business_date: string;
  opening_cash: number;
  cash_in: number;
  cash_out: number;
  expected_cash: number;
  actual_cash: number;
  variance: number;
  closed_by_user_id: string;
  note?: string;
  created_at: string;
};

export type CashReport = {
  business_date: string;
  sale_cash_in: number;
  customer_receipt_cash_in: number;
  purchase_return_cash_in: number;
  cash_in: number;
  purchase_cash_out: number;
  supplier_payment_cash_out: number;
  expense_cash_out: number;
  sale_return_cash_out: number;
  cash_out: number;
  net_cash_movement: number;
  card_in: number;
  card_out: number;
  net_card_movement: number;
  closing?: DailyClosing;
  changed_after_close: boolean;
};


export type AuditLogEntry = {
  id: number;
  occurred_at: string;
  request_id: string;
  actor_user_id: string;
  role: string;
  method: string;
  path: string;
  route: string;
  status: number;
  remote_ip: string;
  metadata?: Record<string, unknown>;
};

export type EdgePairing = { pair_code: string; expires_at: string };
export type EdgeDevice = {
  id: string;
  store_id: string;
  warehouse_id: string;
  name: string;
  active: boolean;
  last_seen_at?: string;
  created_at: string;
};
