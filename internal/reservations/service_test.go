package reservations

import "testing"

func TestReservationTransitions(t *testing.T) {
	tests := []struct {
		current, next string
		buyer         bool
		want          bool
	}{
		{"pending", "accepted", false, true},
		{"accepted", "ready", false, true},
		{"pending", "rejected", false, true},
		{"accepted", "rejected", false, true},
		{"ready", "rejected", false, false},
		{"pending", "cancelled", true, true},
		{"accepted", "cancelled", true, true},
		{"ready", "cancelled", true, true},
		{"rejected", "cancelled", true, false},
		{"pending", "ready", false, false},
	}
	for _, tt := range tests {
		if got := canTransition(tt.current, tt.next, tt.buyer); got != tt.want {
			t.Fatalf("canTransition(%q,%q,buyer=%v)=%v want %v", tt.current, tt.next, tt.buyer, got, tt.want)
		}
	}
}
