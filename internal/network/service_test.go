package network

import "testing"

func TestNormalizeSearchTextDigits(t *testing.T) {
	got := normalizeSearchText("  لنت ۲۰۶ تيپ ۵  ")
	want := "لنت 206 تیپ 5"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestHaversineZero(t *testing.T) {
	if d := haversine(35.7, 51.37, 35.7, 51.37); d != 0 {
		t.Fatalf("expected zero distance, got %v", d)
	}
}
