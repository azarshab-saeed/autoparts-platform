package fitment

import "testing"

func TestNormalizePersianAndArabicDigits(t *testing.T) {
	if got, want := normalize(" لنت ۲۰۶ تيپ ٥ "), "لنت 206 تیپ 5"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
