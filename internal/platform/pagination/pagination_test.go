package pagination

import "testing"

func TestCursorRoundTrip(t *testing.T) {
	c := EncodeOffset(42)
	n, err := DecodeOffset(c)
	if err != nil || n != 42 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}
