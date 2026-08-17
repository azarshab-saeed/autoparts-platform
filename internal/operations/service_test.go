package operations

import "testing"

func TestCalculateClosing(t *testing.T) {
	expected, variance := calculateClosing(1_000_000, 3_500_000, 1_250_000, 3_240_000)
	if expected != 3_250_000 {
		t.Fatalf("expected cash %d, got %d", 3_250_000, expected)
	}
	if variance != -10_000 {
		t.Fatalf("expected variance %d, got %d", -10_000, variance)
	}
}
