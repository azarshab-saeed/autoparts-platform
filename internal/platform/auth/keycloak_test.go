package auth

import (
	"encoding/json"
	"testing"
)

func TestAudienceContainsStringAndArray(t *testing.T) {
	if !audienceContains(json.RawMessage(`"autoparts-api"`), "autoparts-api") {
		t.Fatal("expected string audience to match")
	}
	if !audienceContains(json.RawMessage(`["account","autoparts-api"]`), "autoparts-api") {
		t.Fatal("expected array audience to match")
	}
	if audienceContains(json.RawMessage(`["account"]`), "autoparts-api") {
		t.Fatal("unexpected audience match")
	}
}

func TestSelectAppRoles(t *testing.T) {
	primary, roles := selectAppRoles([]string{"offline_access", "cashier", "owner", "uma_authorization"})
	if primary != "owner" {
		t.Fatalf("expected owner primary role, got %q", primary)
	}
	if len(roles) != 2 {
		t.Fatalf("expected two application roles, got %v", roles)
	}
}
