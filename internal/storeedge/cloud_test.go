package storeedge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPushSaleCarriesCustomerAndStripsLocalPricingFlags(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/edge/sales" {
			t.Fatalf("path=%q want /v1/edge/sales", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"sale-1","total_amount":510,"paid_amount":510,"due_amount":0,"status":"paid"}`))
	}))
	defer server.Close()

	cloud := NewCloud()
	sale := LocalSale{
		LocalOperationID: "local-1",
		CreatedAt:        time.Now(),
		PaymentMethod:    "cash",
		CustomerID:       "customer-123",
		Items: []LocalSaleItem{{
			ProductID: "p1", ProductUnitID: "unit-carton", Title: "Brake", Qty: 6, UnitPrice: 85,
			ManualPrice: true, PreservePrice: true,
		}},
	}
	out, status, err := cloud.PushSale(context.Background(), Config{CloudURL: server.URL, DeviceID: "d1", DeviceSecret: "s1"}, sale)
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusCreated || out.ID != "sale-1" {
		t.Fatalf("unexpected response status=%d out=%+v", status, out)
	}
	if got, _ := body["customer_id"].(string); got != "customer-123" {
		t.Fatalf("customer_id=%q want customer-123", got)
	}
	items, ok := body["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("unexpected items payload: %#v", body["items"])
	}
	item, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected item payload: %#v", items[0])
	}
	if _, exists := item["manual_price"]; exists {
		t.Fatal("manual_price leaked into cloud sale API payload")
	}
	if _, exists := item["preserve_price"]; exists {
		t.Fatal("preserve_price leaked into cloud sale API payload")
	}
	if got, _ := item["unit_price"].(float64); got != 85 {
		t.Fatalf("unit_price=%v want 85", got)
	}
	if got, _ := item["product_unit_id"].(string); got != "unit-carton" {
		t.Fatalf("product_unit_id=%q want unit-carton", got)
	}
	if _, exists := item["base_qty"]; exists {
		t.Fatal("base_qty leaked into cloud sale API payload")
	}
}
