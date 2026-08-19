package finance

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCalculateMaturityAverageWeightedByAmount(t *testing.T) {
	base := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	out, err := calculateMaturityAverage(base, []maturityInput{
		{ID: uuid.New(), Number: "A", Direction: "receivable", Amount: 100, Due: base.AddDate(0, 0, 10)},
		{ID: uuid.New(), Number: "B", Direction: "receivable", Amount: 300, Due: base.AddDate(0, 0, 30)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.WeightedDays != 25 {
		t.Fatalf("want 25 weighted days, got %d", out.WeightedDays)
	}
	if out.MaturityDate != "2026-09-12" {
		t.Fatalf("unexpected maturity date %s", out.MaturityDate)
	}
}

func TestCalculateMaturityAverageRejectsMixedDirection(t *testing.T) {
	base := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	_, err := calculateMaturityAverage(base, []maturityInput{
		{ID: uuid.New(), Number: "A", Direction: "receivable", Amount: 100, Due: base},
		{ID: uuid.New(), Number: "B", Direction: "payable", Amount: 100, Due: base},
	})
	if err == nil {
		t.Fatal("expected mixed direction error")
	}
}

func TestMaturityBucketFor(t *testing.T) {
	base := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	cases := map[int]string{-1: "overdue", 0: "today", 3: "1_7", 20: "8_30", 45: "31_60", 61: "61_plus"}
	for days, want := range cases {
		if got := maturityBucketFor(base.AddDate(0, 0, days), base); got != want {
			t.Fatalf("%d: want %s got %s", days, want, got)
		}
	}
}

func TestClassifyCustomerRisk(t *testing.T) {
	_, _, level := classifyCustomerRisk(1_000_000, 400_000, 10, 0, 10)
	if level != "high" {
		t.Fatalf("expected high, got %s", level)
	}
	_, _, level = classifyCustomerRisk(1_000_000, 0, 10, 1, 0)
	if level != "medium" {
		t.Fatalf("expected medium, got %s", level)
	}
	_, _, level = classifyCustomerRisk(1_000_000, 0, 10, 0, 0)
	if level != "low" {
		t.Fatalf("expected low, got %s", level)
	}
}

func TestBankStatementFingerprintNormalizesText(t *testing.T) {
	id := uuid.New()
	d := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	a := bankStatementFingerprint(id, d, 1000, "POS", "REF", "")
	b := bankStatementFingerprint(id, d, 1000, " pos ", " ref ", "")
	if a != b {
		t.Fatal("fingerprint should normalize text")
	}
	if a == bankStatementFingerprint(id, d, 2000, "POS", "REF", "") {
		t.Fatal("fingerprint must include amount")
	}
}
