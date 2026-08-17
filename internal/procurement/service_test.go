package procurement

import "testing"

func TestProductsCompatible(t *testing.T) {
	cases := []struct {
		name string
		args [6]string
		want bool
	}{
		{"sku", [6]string{"BRK-206", "", "a", "brk-206", "", "b"}, true},
		{"oem", [6]string{"", "206-FB-01", "a", "", "206-fb-01", "b"}, true},
		{"title", [6]string{"", "", "لنت جلو 206", "", "", "لنت جلو 206"}, true},
		{"mismatch", [6]string{"A", "B", "C", "X", "Y", "Z"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := productsCompatible(tc.args[0], tc.args[1], tc.args[2], tc.args[3], tc.args[4], tc.args[5]); got != tc.want {
				t.Fatalf("productsCompatible=%v want %v", got, tc.want)
			}
		})
	}
}

func TestProcurementTransitions(t *testing.T) {
	if !canTransition(StatusRequested, StatusAccepted, "seller") {
		t.Fatal("seller must accept requested")
	}
	if !canTransition(StatusAccepted, StatusReady, "seller") {
		t.Fatal("seller must mark accepted ready")
	}
	if canTransition(StatusReady, StatusRejected, "seller") {
		t.Fatal("seller must not reject ready")
	}
	if !canTransition(StatusReady, StatusCancelled, "buyer") {
		t.Fatal("buyer may cancel ready before receive")
	}
	if canTransition(StatusReceived, StatusCancelled, "buyer") {
		t.Fatal("received order is closed")
	}
}

func TestWeightedAverage(t *testing.T) {
	if got := weightedAverage(10, 100, 2, 200); got != 117 {
		t.Fatalf("got %d want 117", got)
	}
}
