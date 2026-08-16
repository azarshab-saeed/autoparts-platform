package sales

import "testing"

func TestNormalizePayments(t *testing.T) {
	parts, paid, due, err := normalizePayments(1000, "credit", []PaymentPart{{Method: "cash", Amount: 250}, {Method: "card", Amount: 500}})
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 2 || paid != 750 || due != 250 {
		t.Fatalf("parts=%v paid=%d due=%d", parts, paid, due)
	}
	_, _, _, err = normalizePayments(1000, "credit", []PaymentPart{{Method: "cash", Amount: 1200}})
	if err == nil {
		t.Fatal("expected overpayment rejection")
	}
}
