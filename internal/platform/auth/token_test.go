package auth

import (
	"github.com/google/uuid"
	"testing"
	"time"
)

func TestTokenRoundTrip(t *testing.T) {
	m, err := NewTokenManager("12345678901234567890123456789012")
	if err != nil {
		t.Fatal(err)
	}
	want := Claims{UserID: uuid.New(), TenantID: uuid.New(), StoreID: uuid.New(), Role: "owner"}
	tok, err := m.Sign(want, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.Verify(tok)
	if err != nil {
		t.Fatal(err)
	}
	if got.UserID != want.UserID || got.TenantID != want.TenantID || got.StoreID != want.StoreID || got.Role != want.Role {
		t.Fatal("claims mismatch")
	}
}
