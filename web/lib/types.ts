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
  last_updated_at: string;
  freshness: "live" | "recent" | "stale";
  distance_km?: number;
};

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
