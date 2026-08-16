package auth

import "testing"

func TestPasswordRoundTrip(t *testing.T) {
	h, err := HashPassword("a-strong-password")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(h, "a-strong-password") {
		t.Fatal("expected password to verify")
	}
	if VerifyPassword(h, "wrong-password") {
		t.Fatal("wrong password verified")
	}
}
