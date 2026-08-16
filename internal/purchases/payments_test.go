package purchases

import "testing"

func TestNormalizePurchasePayments(t *testing.T) {
	_, paid, due, err := normalizePayments(2000, "credit", []PaymentPart{{Method: "card", Amount: 800}})
	if err != nil {
		t.Fatal(err)
	}
	if paid != 800 || due != 1200 {
		t.Fatalf("paid=%d due=%d", paid, due)
	}
}
