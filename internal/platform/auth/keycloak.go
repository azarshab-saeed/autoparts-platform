package auth

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// KeycloakVerifier verifies RS256 access tokens issued by the configured
// Keycloak realm. JWKS are fetched from Keycloak and cached in memory.
type KeycloakVerifier struct {
	issuer   string
	audience string
	jwksURL  string
	client   *http.Client

	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
	cacheTTL  time.Duration
}

func NewKeycloakVerifier(issuer, audience, jwksURL string) (*KeycloakVerifier, error) {
	issuer = strings.TrimRight(strings.TrimSpace(issuer), "/")
	audience = strings.TrimSpace(audience)
	jwksURL = strings.TrimSpace(jwksURL)
	if issuer == "" || audience == "" {
		return nil, errors.New("KEYCLOAK_ISSUER and KEYCLOAK_AUDIENCE are required")
	}
	if jwksURL == "" {
		jwksURL = issuer + "/protocol/openid-connect/certs"
	}
	return &KeycloakVerifier{
		issuer: issuer, audience: audience, jwksURL: jwksURL,
		client: &http.Client{Timeout: 5 * time.Second},
		keys:   make(map[string]*rsa.PublicKey), cacheTTL: 10 * time.Minute,
	}, nil
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Typ string `json:"typ"`
}

type realmAccess struct {
	Roles []string `json:"roles"`
}

type jwtPayload struct {
	Sub               string          `json:"sub"`
	Iss               string          `json:"iss"`
	Aud               json.RawMessage `json:"aud"`
	Exp               int64           `json:"exp"`
	Nbf               int64           `json:"nbf"`
	Email             string          `json:"email"`
	Name              string          `json:"name"`
	PreferredUsername string          `json:"preferred_username"`
	TenantID          string          `json:"tenant_id"`
	StoreID           string          `json:"store_id"`
	RealmAccess       realmAccess     `json:"realm_access"`
}

func (v *KeycloakVerifier) VerifyToken(ctx context.Context, raw string) (Claims, error) {
	var out Claims
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return out, errors.New("invalid jwt")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return out, errors.New("invalid jwt header")
	}
	var header jwtHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return out, errors.New("invalid jwt header")
	}
	if header.Alg != "RS256" || header.Kid == "" {
		return out, errors.New("unsupported jwt signing algorithm")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return out, errors.New("invalid jwt payload")
	}
	var payload jwtPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return out, errors.New("invalid jwt payload")
	}
	if payload.Iss != v.issuer {
		return out, errors.New("invalid token issuer")
	}
	if !audienceContains(payload.Aud, v.audience) {
		return out, errors.New("invalid token audience")
	}
	now := time.Now().Unix()
	if payload.Exp == 0 || payload.Exp <= now-30 {
		return out, errors.New("token expired")
	}
	if payload.Nbf != 0 && payload.Nbf > now+30 {
		return out, errors.New("token not active yet")
	}

	key, err := v.keyFor(ctx, header.Kid, false)
	if err != nil {
		return out, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return out, errors.New("invalid jwt signature")
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], sig); err != nil {
		// A rotation can reuse a request while our cache still contains old keys.
		if refreshed, refreshErr := v.keyFor(ctx, header.Kid, true); refreshErr == nil {
			if verifyErr := rsa.VerifyPKCS1v15(refreshed, crypto.SHA256, digest[:], sig); verifyErr == nil {
				key = refreshed
			} else {
				return out, errors.New("invalid jwt signature")
			}
		} else {
			return out, errors.New("invalid jwt signature")
		}
	}
	_ = key

	userID, err := uuid.Parse(payload.Sub)
	if err != nil {
		return out, errors.New("token subject must be a UUID")
	}
	role, roles := selectAppRoles(payload.RealmAccess.Roles)
	if role == "" {
		return out, errors.New("token has no application role")
	}

	// Store roles are tenant-bound. External roles (mechanic/consumer) are
	// intentionally allowed without tenant_id/store_id so they can search the
	// cross-store network without pretending to belong to a shop.
	tenantID, storeID := uuid.Nil, uuid.Nil
	if role != "mechanic" && role != "consumer" {
		tenantID, err = uuid.Parse(payload.TenantID)
		if err != nil {
			return out, errors.New("missing or invalid tenant_id claim")
		}
		storeID, err = uuid.Parse(payload.StoreID)
		if err != nil {
			return out, errors.New("missing or invalid store_id claim")
		}
	}
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		name = strings.TrimSpace(payload.PreferredUsername)
	}

	return Claims{
		UserID: userID, TenantID: tenantID, StoreID: storeID,
		Role: role, Roles: roles, Email: payload.Email, Name: name, Exp: payload.Exp,
	}, nil
}

func audienceContains(raw json.RawMessage, expected string) bool {
	if len(raw) == 0 {
		return false
	}
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return one == expected
	}
	var many []string
	if json.Unmarshal(raw, &many) == nil {
		for _, a := range many {
			if a == expected {
				return true
			}
		}
	}
	return false
}

func selectAppRoles(in []string) (string, []string) {
	allowed := map[string]bool{
		"owner": true, "admin": true, "cashier": true, "warehouse": true,
		"accountant": true, "mechanic": true, "consumer": true,
	}
	roles := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, r := range in {
		if allowed[r] && !seen[r] {
			roles = append(roles, r)
			seen[r] = true
		}
	}
	priority := []string{"owner", "admin", "cashier", "warehouse", "accountant", "mechanic", "consumer"}
	for _, p := range priority {
		if seen[p] {
			return p, roles
		}
	}
	return "", roles
}

type jwksDocument struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func (v *KeycloakVerifier) keyFor(ctx context.Context, kid string, force bool) (*rsa.PublicKey, error) {
	v.mu.RLock()
	key := v.keys[kid]
	fresh := time.Since(v.fetchedAt) < v.cacheTTL
	v.mu.RUnlock()
	if key != nil && fresh && !force {
		return key, nil
	}

	if err := v.refreshKeys(ctx); err != nil {
		// Prefer a stale known key over an outage of the JWKS endpoint.
		if key != nil && !force {
			return key, nil
		}
		return nil, err
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	key = v.keys[kid]
	if key == nil {
		return nil, errors.New("jwt signing key not found")
	}
	return key, nil
}

func (v *KeycloakVerifier) refreshKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch jwks: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	var doc jwksDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return fmt.Errorf("parse jwks: %w", err)
	}
	keys := make(map[string]*rsa.PublicKey)
	for _, item := range doc.Keys {
		if item.Kty != "RSA" || item.Kid == "" || item.N == "" || item.E == "" {
			continue
		}
		if item.Alg != "" && item.Alg != "RS256" {
			continue
		}
		if item.Use != "" && item.Use != "sig" {
			continue
		}
		pub, err := rsaPublicKey(item.N, item.E)
		if err != nil {
			continue
		}
		keys[item.Kid] = pub
	}
	if len(keys) == 0 {
		return errors.New("jwks contains no usable RSA keys")
	}
	v.mu.Lock()
	v.keys = keys
	v.fetchedAt = time.Now()
	v.mu.Unlock()
	return nil
}

func rsaPublicKey(nEncoded, eEncoded string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nEncoded)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eEncoded)
	if err != nil {
		return nil, err
	}
	e := 0
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}
	if e < 3 {
		return nil, errors.New("invalid RSA exponent")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}, nil
}
