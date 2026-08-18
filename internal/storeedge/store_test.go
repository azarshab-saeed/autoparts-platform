package storeedge

import (
	"testing"
	"time"
)

func TestQueueSaleIsDurableAndReducesLocalAvailability(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SavePairing("http://example.invalid", "cashier", PairResponse{DeviceID: "d1", DeviceSecret: "s1", StoreID: "st", WarehouseID: "wh", StoreName: "store"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceSnapshot(Snapshot{GeneratedAt: time.Now(), StoreID: "st", StoreName: "store", WarehouseID: "wh", Products: []Product{{ProductID: "p1", Title: "Brake", Available: 5, OnHand: 5, SellingPrice: 100}}}); err != nil {
		t.Fatal(err)
	}
	sale, err := s.QueueSale("cash", "", []LocalSaleItem{{ProductID: "p1", Qty: 2, UnitPrice: 100}})
	if err != nil {
		t.Fatal(err)
	}
	if sale.Status != "pending" || sale.TotalAmount != 200 {
		t.Fatalf("unexpected sale: %+v", sale)
	}
	if got := s.SearchCatalog("Brake", 10)[0].Available; got != 3 {
		t.Fatalf("available=%v want 3", got)
	}
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.PendingSales()) != 1 {
		t.Fatalf("pending sales were not durable")
	}
	if got := s2.SearchCatalog("Brake", 10)[0].Available; got != 3 {
		t.Fatalf("reloaded available=%v want 3", got)
	}
}

func TestOfflineSaleRejectsCredit(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_ = s.SavePairing("http://example.invalid", "cashier", PairResponse{DeviceID: "d1", DeviceSecret: "s1", StoreID: "st", WarehouseID: "wh", StoreName: "store"})
	_ = s.ReplaceSnapshot(Snapshot{Products: []Product{{ProductID: "p1", Title: "Brake", Available: 1, OnHand: 1}}})
	if _, err := s.QueueSale("credit", "", []LocalSaleItem{{ProductID: "p1", Qty: 1, UnitPrice: 10}}); err == nil {
		t.Fatal("expected credit sale to be rejected")
	}
}

func TestOfflineSaleUsesQuantityBreakUnlessPriceIsManual(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SavePairing("http://example.invalid", "cashier", PairResponse{DeviceID: "d1", DeviceSecret: "s1", StoreID: "st", WarehouseID: "wh", StoreName: "store"}); err != nil {
		t.Fatal(err)
	}
	p := Product{
		ProductID: "p1", Title: "Brake", Available: 20, OnHand: 20, SellingPrice: 100,
		PriceBreaks: []PriceBreak{{MinQty: 1, UnitPrice: 100}, {MinQty: 5, UnitPrice: 90}, {MinQty: 10, UnitPrice: 80}},
	}
	if err := s.ReplaceSnapshot(Snapshot{Products: []Product{p}}); err != nil {
		t.Fatal(err)
	}

	sale, err := s.QueueSale("cash", "", []LocalSaleItem{{ProductID: "p1", Qty: 6, UnitPrice: 100}})
	if err != nil {
		t.Fatal(err)
	}
	if got := sale.Items[0].UnitPrice; got != 90 {
		t.Fatalf("tier unit price=%d want 90", got)
	}
	if sale.TotalAmount != 540 {
		t.Fatalf("tier total=%d want 540", sale.TotalAmount)
	}

	manual, err := s.QueueSale("cash", "", []LocalSaleItem{{ProductID: "p1", Qty: 2, UnitPrice: 77, ManualPrice: true}})
	if err != nil {
		t.Fatal(err)
	}
	if got := manual.Items[0].UnitPrice; got != 77 {
		t.Fatalf("manual unit price=%d want 77", got)
	}
}

func TestOfflineSaleRejectsManualPriceWhenCashierOverrideDisabled(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SavePairing("http://example.invalid", "cashier", PairResponse{DeviceID: "d1", DeviceSecret: "s1", StoreID: "st", WarehouseID: "wh", StoreName: "store"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceSnapshot(Snapshot{
		PricingPolicy: &PricingPolicy{CashierMayOverride: false},
		Products:      []Product{{ProductID: "p1", Title: "Brake", Available: 5, OnHand: 5, SellingPrice: 100}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.QueueSale("cash", "", []LocalSaleItem{{ProductID: "p1", Qty: 1, UnitPrice: 77, ManualPrice: true}}); err == nil {
		t.Fatal("expected manual price to be rejected when cashier override is disabled")
	}
}

func TestOfflineSalePreservesResolvedCustomerTierAndCustomerID(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SavePairing("http://example.invalid", "cashier", PairResponse{DeviceID: "d1", DeviceSecret: "s1", StoreID: "st", WarehouseID: "wh", StoreName: "store"}); err != nil {
		t.Fatal(err)
	}
	p := Product{
		ProductID: "p1", Title: "Brake", Available: 20, OnHand: 20, SellingPrice: 100,
		PriceBreaks: []PriceBreak{{MinQty: 1, UnitPrice: 100}, {MinQty: 5, UnitPrice: 90}},
	}
	if err := s.ReplaceSnapshot(Snapshot{PricingPolicy: &PricingPolicy{CashierMayOverride: true}, Products: []Product{p}}); err != nil {
		t.Fatal(err)
	}

	sale, err := s.QueueSale("cash", "customer-123", []LocalSaleItem{{ProductID: "p1", Qty: 6, UnitPrice: 85, PreservePrice: true}})
	if err != nil {
		t.Fatal(err)
	}
	if sale.CustomerID != "customer-123" {
		t.Fatalf("customer id=%q want customer-123", sale.CustomerID)
	}
	if got := sale.Items[0].UnitPrice; got != 85 {
		t.Fatalf("preserved customer-tier unit price=%d want 85", got)
	}
}

func TestOfflineSaleSupportsPackageBarcodeAndBaseInventory(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SavePairing("http://example.invalid", "cashier", PairResponse{DeviceID: "d1", DeviceSecret: "s1", StoreID: "st", WarehouseID: "wh", StoreName: "store"}); err != nil {
		t.Fatal(err)
	}
	p := Product{
		ProductID: "p1", Title: "Oil Filter", Barcode: "111", Available: 24, OnHand: 24, SellingPrice: 100,
		PriceBreaks: []PriceBreak{{MinQty: 1, UnitPrice: 100}},
		Units: []ProductUnit{
			{ProductUnitID: "u-base", Code: "pcs", Name: "عدد", FactorToBase: 1, Barcode: "111", IsBase: true, PriceBreaks: []PriceBreak{{MinQty: 1, UnitPrice: 100}}},
			{ProductUnitID: "u-carton", Code: "carton", Name: "کارتن", FactorToBase: 12, Barcode: "222"},
		},
	}
	if err := s.ReplaceSnapshot(Snapshot{Products: []Product{p}}); err != nil {
		t.Fatal(err)
	}
	search := s.SearchCatalog("222", 10)
	if len(search) != 1 || search[0].ProductID != "p1" {
		t.Fatalf("package barcode did not find product: %+v", search)
	}
	sale, err := s.QueueSale("cash", "", []LocalSaleItem{{ProductID: "p1", ProductUnitID: "u-carton", Qty: 1, UnitPrice: 0}})
	if err != nil {
		t.Fatal(err)
	}
	item := sale.Items[0]
	if item.UnitPrice != 1200 || item.BaseQty != 12 || item.UnitName != "کارتن" || item.ConversionFactor != 12 {
		t.Fatalf("unexpected package line: %+v", item)
	}
	if sale.TotalAmount != 1200 {
		t.Fatalf("total=%d want 1200", sale.TotalAmount)
	}
	got := s.SearchCatalog("Oil", 10)[0]
	if got.Available != 12 || got.OnHand != 12 {
		t.Fatalf("base stock after carton sale on_hand=%v available=%v want 12", got.OnHand, got.Available)
	}
}

func TestOfflineSaleUsesPackageSpecificPriceBreak(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_ = s.SavePairing("http://example.invalid", "cashier", PairResponse{DeviceID: "d1", DeviceSecret: "s1", StoreID: "st", WarehouseID: "wh", StoreName: "store"})
	p := Product{
		ProductID: "p1", Title: "Oil Filter", Available: 60, OnHand: 60, SellingPrice: 100,
		PriceBreaks: []PriceBreak{{MinQty: 1, UnitPrice: 100}, {MinQty: 24, UnitPrice: 90}},
		Units: []ProductUnit{
			{ProductUnitID: "u-base", Code: "pcs", Name: "عدد", FactorToBase: 1, IsBase: true, PriceBreaks: []PriceBreak{{MinQty: 1, UnitPrice: 100}, {MinQty: 24, UnitPrice: 90}}},
			{ProductUnitID: "u-carton", Code: "carton", Name: "کارتن", FactorToBase: 12, PriceBreaks: []PriceBreak{{MinQty: 1, UnitPrice: 1100}, {MinQty: 2, UnitPrice: 1050}}},
		},
	}
	if err := s.ReplaceSnapshot(Snapshot{Products: []Product{p}}); err != nil {
		t.Fatal(err)
	}
	sale, err := s.QueueSale("cash", "", []LocalSaleItem{{ProductID: "p1", ProductUnitID: "u-carton", Qty: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if sale.Items[0].UnitPrice != 1050 || sale.Items[0].BaseQty != 24 || sale.TotalAmount != 2100 {
		t.Fatalf("unexpected package tier sale: %+v", sale)
	}
}

func TestReplaceSnapshotSubtractsPendingBaseQty(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.SavePairing("http://example.invalid", "cashier", PairResponse{DeviceID: "d1", DeviceSecret: "s1", StoreID: "st", WarehouseID: "wh", StoreName: "store"})
	p := Product{ProductID: "p1", Title: "Filter", Available: 24, OnHand: 24, SellingPrice: 100, Units: []ProductUnit{{ProductUnitID: "base", IsBase: true, FactorToBase: 1}, {ProductUnitID: "carton", FactorToBase: 12}}}
	if err := s.ReplaceSnapshot(Snapshot{Products: []Product{p}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.QueueSale("cash", "", []LocalSaleItem{{ProductID: "p1", ProductUnitID: "carton", Qty: 1}}); err != nil {
		t.Fatal(err)
	}
	// A fresh cloud snapshot still reports 24 base units because the local sale is not synced yet.
	if err := s.ReplaceSnapshot(Snapshot{Products: []Product{p}}); err != nil {
		t.Fatal(err)
	}
	if got := s.SearchCatalog("Filter", 10)[0].Available; got != 12 {
		t.Fatalf("available=%v want 12 after replaying pending base qty", got)
	}
}
