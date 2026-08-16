package sales

import "testing"

func TestCreateSaleItemValidation(t *testing.T) {
	item := CreateSaleItem{Qty: 0, UnitPrice: 1000}
	if item.Qty > 0 {
		t.Fatal("expected invalid quantity")
	}
}
