package tax

import "testing"

func TestCalculateLineExclusive(t *testing.T) {
	base, tax, total, err := CalculateLine(1_000_000, 1000, "taxable", "exclusive")
	if err != nil || base != 1_000_000 || tax != 100_000 || total != 1_100_000 {
		t.Fatalf("exclusive got base=%d tax=%d total=%d err=%v", base, tax, total, err)
	}
}

func TestCalculateLineInclusive(t *testing.T) {
	base, tax, total, err := CalculateLine(1_100_000, 1000, "taxable", "inclusive")
	if err != nil || base != 1_000_000 || tax != 100_000 || total != 1_100_000 {
		t.Fatalf("inclusive got base=%d tax=%d total=%d err=%v", base, tax, total, err)
	}
}

func TestCalculateLineExempt(t *testing.T) {
	base, tax, total, err := CalculateLine(550_000, 1000, "exempt", "exclusive")
	if err != nil || base != 550_000 || tax != 0 || total != 550_000 {
		t.Fatalf("exempt got base=%d tax=%d total=%d err=%v", base, tax, total, err)
	}
}

func TestCalculateLineLargeAmountKeepsIntegerPrecision(t *testing.T) {
	const amount int64 = 9_007_199_254_740_993
	base, taxAmount, total, err := CalculateLine(amount, 1, "taxable", "exclusive")
	if err != nil {
		t.Fatal(err)
	}
	if base != amount || taxAmount != 900_719_925_474 || total != 9_008_099_974_666_467 {
		t.Fatalf("unexpected exact result: base=%d tax=%d total=%d", base, taxAmount, total)
	}
}

func TestCalculateLineRoundsHalfUp(t *testing.T) {
	_, taxAmount, _, err := CalculateLine(5_000, 1, "taxable", "exclusive")
	if err != nil {
		t.Fatal(err)
	}
	if taxAmount != 1 {
		t.Fatalf("expected half-up tax 1, got %d", taxAmount)
	}
}
