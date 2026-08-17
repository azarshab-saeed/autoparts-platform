package catalog

import "testing"

func TestNormalizeImportRowsRejectsDuplicateSKU(t *testing.T) {
	_, err := normalizeImportRows([]ImportRow{
		{RowNumber: 2, SKU: "BRK-206", Title: "لنت 206", OnHand: 1, AvgUnitCost: 100},
		{RowNumber: 3, SKU: "brk-206", Title: "لنت 206 دوم", OnHand: 1, AvgUnitCost: 100},
	})
	if err == nil {
		t.Fatal("expected duplicate SKU validation error")
	}
}

func TestNormalizeImportRowsRequiresCostForOpeningStock(t *testing.T) {
	_, err := normalizeImportRows([]ImportRow{{RowNumber: 2, Title: "فیلتر روغن", OnHand: 3}})
	if err == nil {
		t.Fatal("expected opening cost validation error")
	}
}

func TestUnitOrPCSDefaultsUnit(t *testing.T) {
	if got := unitOrPCS(""); got != "pcs" {
		t.Fatalf("expected pcs, got %q", got)
	}
}
