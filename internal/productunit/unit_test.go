package productunit

import "testing"

func TestBaseQtyConvertsCommercialPackaging(t *testing.T) {
	u := Unit{FactorToBase: 12, AllowFractionalBaseQty: false}
	got, err := BaseQty(2, u)
	if err != nil {
		t.Fatalf("unexpected conversion error: %v", err)
	}
	if got != 24 {
		t.Fatalf("got %.3f base units, want 24", got)
	}
}

func TestBaseQtyRejectsFractionalCommercialPackageWhenPieceStock(t *testing.T) {
	u := Unit{FactorToBase: 12, AllowFractionalBaseQty: false}
	if _, err := BaseQty(0.5, u); err == nil {
		t.Fatal("expected fractional commercial package quantity to be rejected")
	}
}

func TestBaseQtyRejectsFractionalBaseWhenDisabled(t *testing.T) {
	u := Unit{FactorToBase: 2.5, AllowFractionalBaseQty: false}
	if _, err := BaseQty(1, u); err == nil {
		t.Fatal("expected fractional base quantity to be rejected")
	}
}

func TestBaseQtyAllowsFractionalBaseWhenEnabled(t *testing.T) {
	u := Unit{FactorToBase: 0.5, AllowFractionalBaseQty: true}
	got, err := BaseQty(3, u)
	if err != nil {
		t.Fatalf("unexpected conversion error: %v", err)
	}
	if got != 1.5 {
		t.Fatalf("got %.3f base units, want 1.5", got)
	}
}

func TestMoneyConversionUsesPackagingFactor(t *testing.T) {
	u := Unit{FactorToBase: 12}
	if got := CommercialMoney(100, u); got != 1200 {
		t.Fatalf("commercial money got %d want 1200", got)
	}
	if got := BaseMoney(1200, u); got != 100 {
		t.Fatalf("base money got %d want 100", got)
	}
}
