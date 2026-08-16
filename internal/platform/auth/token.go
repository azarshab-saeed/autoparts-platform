package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Claims struct {
	UserID   uuid.UUID `json:"sub"`
	TenantID uuid.UUID `json:"tenant_id"`
	StoreID  uuid.UUID `json:"store_id"`
	Role     string    `json:"role"`
	Roles    []string  `json:"roles,omitempty"`
	Email    string    `json:"email,omitempty"`
	Name     string    `json:"name,omitempty"`
	Exp      int64     `json:"exp"`
}

type TokenManager struct{ secret []byte }

func NewTokenManager(secret string) (*TokenManager, error) {
	if len(secret) < 32 {
		return nil, errors.New("AUTH_SECRET must be at least 32 characters")
	}
	return &TokenManager{secret: []byte(secret)}, nil
}

func (m *TokenManager) Sign(c Claims, ttl time.Duration) (string, error) {
	c.Exp = time.Now().Add(ttl).Unix()
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(body))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return body + "." + sig, nil
}

func (m *TokenManager) Verify(token string) (Claims, error) {
	var c Claims
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return c, errors.New("invalid token")
	}
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(parts[0]))
	supplied, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return c, errors.New("invalid token")
	}
	if !hmac.Equal(supplied, mac.Sum(nil)) {
		return c, errors.New("invalid token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return c, errors.New("invalid token")
	}
	if err := json.Unmarshal(payload, &c); err != nil {
		return c, errors.New("invalid token")
	}
	if c.Exp <= time.Now().Unix() {
		return c, errors.New("token expired")
	}
	if c.UserID == uuid.Nil || c.TenantID == uuid.Nil || c.StoreID == uuid.Nil || c.Role == "" {
		return c, errors.New("invalid token claims")
	}
	return c, nil
}

// VerifyToken adapts the legacy token manager to the middleware verifier interface.
func (m *TokenManager) VerifyToken(_ context.Context, token string) (Claims, error) {
	return m.Verify(token)
}
