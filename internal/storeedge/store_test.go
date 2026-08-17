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
	sale, err := s.QueueSale("cash", []LocalSaleItem{{ProductID: "p1", Qty: 2, UnitPrice: 100}})
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
	if _, err := s.QueueSale("credit", []LocalSaleItem{{ProductID: "p1", Qty: 1, UnitPrice: 10}}); err == nil {
		t.Fatal("expected credit sale to be rejected")
	}
}
