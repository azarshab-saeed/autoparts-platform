package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	ownerWarehouseID  = "33333333-3333-3333-3333-333333333333"
	ownerBrakeProduct = "55555555-5555-5555-5555-555555555551"
	parsWarehouseID   = "33333333-3333-3333-3333-333333333335"
	parsProductID     = "55555555-5555-5555-5555-555555555571"
	parsStoreID       = "22222222-2222-2222-2222-222222222224"
)

type config struct {
	apiURL      string
	keycloakURL string
	realm       string
	clientID    string
	password    string
	mutating    bool
}

type runner struct {
	cfg    config
	client *http.Client
	passed int
	failed int
}

type searchItem struct {
	OfferID          string  `json:"offer_id"`
	ProductID        string  `json:"product_id"`
	StoreID          string  `json:"store_id"`
	StoreName        string  `json:"store_name"`
	Available        float64 `json:"available"`
	AllowReservation bool    `json:"allow_reservation"`
	AllowProcurement bool    `json:"allow_procurement"`
}

type searchResponse struct {
	Items []searchItem `json:"items"`
}

func main() {
	cfg := config{
		apiURL:      strings.TrimRight(env("RC_API_URL", "http://localhost:8080"), "/"),
		keycloakURL: strings.TrimRight(env("RC_KEYCLOAK_URL", "http://localhost:8081"), "/"),
		realm:       env("RC_KEYCLOAK_REALM", "autoparts"),
		clientID:    env("RC_QA_CLIENT_ID", "autoparts-qa"),
		password:    env("RC_DEV_PASSWORD", "ChangeMe123!"),
		mutating:    env("RC_SMOKE_MUTATING", "true") != "false",
	}
	r := &runner{cfg: cfg, client: &http.Client{Timeout: 15 * time.Second}}

	fmt.Printf("AutoParts RC smoke: api=%s mutating=%t\n", cfg.apiURL, cfg.mutating)
	if err := r.run(); err != nil {
		fmt.Fprintf(os.Stderr, "\nRC smoke FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\nRC smoke PASSED: %d checks\n", r.passed)
}

func (r *runner) run() error {
	if err := r.checkPublic(); err != nil {
		return err
	}

	ownerToken, err := r.token("owner@example.com")
	if err != nil {
		return r.fail("owner token", err)
	}
	parsToken, err := r.token("pars@example.com")
	if err != nil {
		return r.fail("pars token", err)
	}
	mechanicToken, err := r.token("mechanic@example.com")
	if err != nil {
		return r.fail("mechanic token", err)
	}
	r.pass("Keycloak QA password-grant identities")

	if err := r.checkMe(ownerToken, "owner", "owner@example.com"); err != nil {
		return err
	}
	if err := r.checkMe(parsToken, "owner", "pars@example.com"); err != nil {
		return err
	}
	if err := r.checkMe(mechanicToken, "mechanic", "mechanic@example.com"); err != nil {
		return err
	}
	if err := r.checkTenantIsolation(ownerToken); err != nil {
		return err
	}

	search, err := r.networkSearch("4254.97")
	if err != nil {
		return err
	}
	parsOffer, err := pickParsOffer(search.Items)
	if err != nil {
		return r.fail("network search cross-tenant result", err)
	}
	r.pass("OEM/alias network search crosses tenants")

	if !r.cfg.mutating {
		fmt.Println("SKIP self-reverting mutation checks (RC_SMOKE_MUTATING=false)")
		return nil
	}

	if err := r.checkCatalogImport(ownerToken); err != nil {
		return err
	}
	if err := r.checkInventoryIdempotency(ownerToken); err != nil {
		return err
	}
	if err := r.checkReservationHoldRelease(mechanicToken, parsOffer); err != nil {
		return err
	}
	if err := r.checkProcurementHoldRelease(ownerToken, parsOffer); err != nil {
		return err
	}
	return nil
}

func (r *runner) checkPublic() error {
	for _, path := range []string{"/healthz", "/readyz", "/version"} {
		status, body, err := r.do(http.MethodGet, r.cfg.apiURL+path, "", nil, "")
		if err != nil {
			return r.fail(path, err)
		}
		if status != http.StatusOK {
			return r.fail(path, fmt.Errorf("status=%d body=%s", status, body))
		}
		r.pass(path)
	}
	return nil
}

func (r *runner) token(username string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", r.cfg.clientID)
	form.Set("username", username)
	form.Set("password", r.cfg.password)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", r.cfg.keycloakURL, r.cfg.realm), strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Keycloak status=%d body=%s", resp.StatusCode, body)
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", errors.New("access_token missing")
	}
	return out.AccessToken, nil
}

func (r *runner) checkMe(token, role, email string) error {
	status, body, err := r.do(http.MethodGet, r.cfg.apiURL+"/v1/me", token, nil, "")
	if err != nil {
		return r.fail("/v1/me "+email, err)
	}
	if status != http.StatusOK {
		return r.fail("/v1/me "+email, fmt.Errorf("status=%d body=%s", status, body))
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return r.fail("/v1/me "+email, err)
	}
	if fmt.Sprint(out["role"]) != role || fmt.Sprint(out["email"]) != email {
		return r.fail("/v1/me "+email, fmt.Errorf("unexpected identity: %s", body))
	}
	r.pass("identity " + email)
	return nil
}

func (r *runner) checkTenantIsolation(token string) error {
	status, body, err := r.do(http.MethodGet, r.cfg.apiURL+"/v1/inventory?warehouse_id="+ownerWarehouseID+"&limit=100&offset=0", token, nil, "")
	if err != nil {
		return r.fail("tenant isolation", err)
	}
	if status != http.StatusOK {
		return r.fail("tenant isolation", fmt.Errorf("status=%d body=%s", status, body))
	}
	if bytes.Contains(body, []byte(parsProductID)) {
		return r.fail("tenant isolation", errors.New("owner inventory leaked pars tenant product"))
	}

	status, deniedBody, err := r.do(http.MethodGet, r.cfg.apiURL+"/v1/inventory?warehouse_id="+parsWarehouseID+"&limit=10&offset=0", token, nil, "")
	if err != nil {
		return r.fail("cross-tenant warehouse denial", err)
	}
	if status != http.StatusConflict {
		return r.fail("cross-tenant warehouse denial", fmt.Errorf("expected 409, got status=%d body=%s", status, deniedBody))
	}
	r.pass("store inventory tenant isolation + foreign warehouse denial")
	return nil
}

func (r *runner) networkSearch(q string) (searchResponse, error) {
	u := r.cfg.apiURL + "/v1/network/search?q=" + url.QueryEscape(q) + "&limit=50"
	status, body, err := r.do(http.MethodGet, u, "", nil, "")
	if err != nil {
		return searchResponse{}, r.fail("network search", err)
	}
	if status != http.StatusOK {
		return searchResponse{}, r.fail("network search", fmt.Errorf("status=%d body=%s", status, body))
	}
	var out searchResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return out, r.fail("network search", err)
	}
	if len(out.Items) == 0 {
		return out, r.fail("network search", errors.New("no results for seeded OEM 4254.97"))
	}
	return out, nil
}

func pickParsOffer(items []searchItem) (searchItem, error) {
	for _, item := range items {
		if item.StoreID == parsStoreID && item.Available >= 1 && item.AllowReservation && item.AllowProcurement {
			return item, nil
		}
	}
	return searchItem{}, errors.New("pars offer with reservation/procurement availability not found")
}

func (r *runner) checkCatalogImport(token string) error {
	const key = "rc15-6-import-v1"
	payload := map[string]any{
		"warehouse_id": ownerWarehouseID,
		"rows": []map[string]any{{
			"row_number": 2, "sku": "RC-IMPORT-001", "title": "کالای تست ورود RC", "brand": "RC",
			"on_hand": 1, "avg_unit_cost": 1000, "selling_price": 1500,
			"visible": false, "allow_reservation": true, "allow_procurement": true,
		}},
	}
	status, first, err := r.do(http.MethodPost, r.cfg.apiURL+"/v1/products/import", token, payload, key)
	if err != nil || status != http.StatusCreated {
		return r.fail("catalog import first write", fmt.Errorf("status=%d err=%v body=%s", status, err, first))
	}
	var a struct {
		BatchID string `json:"batch_id"`
	}
	if err := json.Unmarshal(first, &a); err != nil || a.BatchID == "" {
		return r.fail("catalog import first write", fmt.Errorf("invalid response: %s", first))
	}
	status, second, err := r.do(http.MethodPost, r.cfg.apiURL+"/v1/products/import", token, payload, key)
	if err != nil || status != http.StatusCreated {
		return r.fail("catalog import replay", fmt.Errorf("status=%d err=%v body=%s", status, err, second))
	}
	var b struct {
		BatchID string `json:"batch_id"`
	}
	if err := json.Unmarshal(second, &b); err != nil || b.BatchID != a.BatchID {
		return r.fail("catalog import replay", fmt.Errorf("idempotency mismatch first=%s second=%s", a.BatchID, b.BatchID))
	}
	r.pass("catalog import idempotency + opening inventory")
	return nil
}

func (r *runner) checkInventoryIdempotency(token string) error {
	key := fmt.Sprintf("rc15-adjust-%d", time.Now().UnixNano())
	payload := map[string]any{
		"warehouse_id": ownerWarehouseID,
		"product_id":   ownerBrakeProduct,
		"qty_delta":    1,
		"reason":       "RC15 self-reverting idempotency check",
	}
	status, first, err := r.do(http.MethodPost, r.cfg.apiURL+"/v1/inventory/adjustments", token, payload, key)
	if err != nil || status != http.StatusCreated {
		return r.fail("inventory idempotency first write", fmt.Errorf("status=%d err=%v body=%s", status, err, first))
	}
	cleanupKey := fmt.Sprintf("rc15-adjust-rollback-%d", time.Now().UnixNano())
	defer func() {
		_, _, _ = r.do(http.MethodPost, r.cfg.apiURL+"/v1/inventory/adjustments", token, map[string]any{
			"warehouse_id": ownerWarehouseID, "product_id": ownerBrakeProduct, "qty_delta": -1, "reason": "RC15 rollback",
		}, cleanupKey)
	}()

	status, second, err := r.do(http.MethodPost, r.cfg.apiURL+"/v1/inventory/adjustments", token, payload, key)
	if err != nil || status != http.StatusCreated {
		return r.fail("inventory idempotency replay", fmt.Errorf("status=%d err=%v body=%s", status, err, second))
	}
	var a, b map[string]any
	_ = json.Unmarshal(first, &a)
	_ = json.Unmarshal(second, &b)
	if fmt.Sprint(a["id"]) == "" || fmt.Sprint(a["id"]) != fmt.Sprint(b["id"]) {
		return r.fail("inventory idempotency replay", fmt.Errorf("ids differ first=%s second=%s", first, second))
	}
	r.pass("inventory idempotency replay")
	return nil
}

func (r *runner) checkReservationHoldRelease(token string, offer searchItem) error {
	key := fmt.Sprintf("rc15-reservation-%d", time.Now().UnixNano())
	status, body, err := r.do(http.MethodPost, r.cfg.apiURL+"/v1/network/reservations", token, map[string]any{"offer_id": offer.OfferID, "qty": 1}, key)
	if err != nil || status != http.StatusCreated {
		return r.fail("reservation create", fmt.Errorf("status=%d err=%v body=%s", status, err, body))
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return r.fail("reservation create", err)
	}
	id := fmt.Sprint(out["id"])
	if id == "" || id == "<nil>" {
		return r.fail("reservation create", fmt.Errorf("id missing: %s", body))
	}
	cancel := func() error {
		status, body, err := r.do(http.MethodPost, r.cfg.apiURL+"/v1/me/reservations/"+id+"/cancel", token, nil, "")
		if err != nil || status != http.StatusOK {
			return fmt.Errorf("status=%d err=%v body=%s", status, err, body)
		}
		return nil
	}
	if err := cancel(); err != nil {
		return r.fail("reservation cancel/release", err)
	}
	r.pass("reservation hold then buyer cancellation/release")
	return nil
}

func (r *runner) checkProcurementHoldRelease(token string, offer searchItem) error {
	key := fmt.Sprintf("rc15-procurement-%d", time.Now().UnixNano())
	payload := map[string]any{
		"offer_id":         offer.OfferID,
		"buyer_product_id": ownerBrakeProduct,
		"warehouse_id":     ownerWarehouseID,
		"qty":              1,
	}
	status, body, err := r.do(http.MethodPost, r.cfg.apiURL+"/v1/network/procurements", token, payload, key)
	if err != nil || status != http.StatusCreated {
		return r.fail("procurement create", fmt.Errorf("status=%d err=%v body=%s", status, err, body))
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return r.fail("procurement create", err)
	}
	id := fmt.Sprint(out["id"])
	if id == "" || id == "<nil>" {
		return r.fail("procurement create", fmt.Errorf("id missing: %s", body))
	}
	status, cancelBody, err := r.do(http.MethodPost, r.cfg.apiURL+"/v1/network/procurements/"+id+"/cancel", token, nil, "")
	if err != nil || status != http.StatusOK {
		return r.fail("procurement cancel/release", fmt.Errorf("status=%d err=%v body=%s", status, err, cancelBody))
	}
	r.pass("procurement hold then buyer cancellation/release")
	return nil
}

func (r *runner) do(method, endpoint, token string, payload any, idempotencyKey string) (int, []byte, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return 0, nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	return resp.StatusCode, b, err
}

func (r *runner) pass(name string) {
	r.passed++
	fmt.Printf("PASS %s\n", name)
}

func (r *runner) fail(name string, err error) error {
	r.failed++
	return fmt.Errorf("%s: %w", name, err)
}

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
