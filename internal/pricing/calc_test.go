package pricing

import "testing"

func TestSelectBreakUsesHighestEligibleQuantity(t *testing.T) {
	got, ok := SelectBreak([]Break{{MinQty: 10, UnitPrice: 790}, {MinQty: 1, UnitPrice: 850}, {MinQty: 5, UnitPrice: 820}}, 7)
	if !ok || got.UnitPrice != 820 {
		t.Fatalf("expected 820 break, got %+v ok=%v", got, ok)
	}
}

func TestMinimumPriceForMargin(t *testing.T) {
	got := MinimumPriceForMargin(700000, 1000)
	if got != 777778 {
		t.Fatalf("expected 777778, got %d", got)
	}
	if MarginBPS(got, 700000) < 1000 {
		t.Fatalf("minimum price must preserve requested margin, got %d bps", MarginBPS(got, 700000))
	}
}
