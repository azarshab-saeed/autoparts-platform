package purchases

import "testing"

func TestWeightedAverage(t *testing.T) {
	if got := weightedAverage(10, 100, 10, 200); got != 150 {
		t.Fatalf("got %d want 150", got)
	}
	if got := weightedAverage(0, 0, 5, 123); got != 123 {
		t.Fatalf("got %d want 123", got)
	}
	if got := weightedAverage(3, 100, 1, 200); got != 125 {
		t.Fatalf("got %d want 125", got)
	}
}
