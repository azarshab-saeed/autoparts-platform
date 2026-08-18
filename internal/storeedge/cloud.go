package storeedge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Cloud struct {
	client *http.Client
}

func NewCloud() *Cloud {
	return &Cloud{client: &http.Client{Timeout: 12 * time.Second}}
}

func (c *Cloud) Pair(ctx context.Context, cloudURL, code, deviceName string) (PairResponse, error) {
	var out PairResponse
	err := c.do(ctx, strings.TrimRight(cloudURL, "/")+"/v1/edge/pair", "POST", "", "", map[string]any{"pair_code": code, "device_name": deviceName}, &out)
	return out, err
}

func (c *Cloud) Snapshot(ctx context.Context, cfg Config) (Snapshot, error) {
	var out Snapshot
	err := c.do(ctx, cfg.CloudURL+"/v1/edge/snapshot", "GET", cfg.DeviceID, cfg.DeviceSecret, nil, &out)
	return out, err
}

func (c *Cloud) PushSale(ctx context.Context, cfg Config, sale LocalSale) (CloudSaleResponse, int, error) {
	var out CloudSaleResponse
	items := make([]map[string]any, 0, len(sale.Items))
	for _, item := range sale.Items {
		items = append(items, map[string]any{
			"product_id": item.ProductID,
			"title":      item.Title,
			"qty":        item.Qty,
			"unit_price": item.UnitPrice,
		})
	}
	body := map[string]any{
		"local_operation_id": sale.LocalOperationID,
		"occurred_at":        sale.CreatedAt,
		"payment_method":     sale.PaymentMethod,
		"items":              items,
	}
	if sale.CustomerID != "" {
		body["customer_id"] = sale.CustomerID
	}
	status, err := c.doStatus(ctx, cfg.CloudURL+"/v1/edge/sales", "POST", cfg.DeviceID, cfg.DeviceSecret, body, &out)
	return out, status, err
}

func (c *Cloud) Heartbeat(ctx context.Context, cfg Config) error {
	return c.do(ctx, cfg.CloudURL+"/v1/edge/heartbeat", "POST", cfg.DeviceID, cfg.DeviceSecret, map[string]any{"pending_sales": 0}, &struct{}{})
}

func (c *Cloud) do(ctx context.Context, url, method, deviceID, secret string, body any, out any) error {
	_, err := c.doStatus(ctx, url, method, deviceID, secret, body, out)
	return err
}

func (c *Cloud) doStatus(ctx context.Context, url, method, deviceID, secret string, body any, out any) (int, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, r)
	if err != nil {
		return 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if deviceID != "" {
		req.Header.Set("X-Edge-Device-ID", deviceID)
		req.Header.Set("X-Edge-Secret", secret)
	}
	res, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return res.StatusCode, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(b, &apiErr)
		message := strings.TrimSpace(apiErr.Error.Message)
		if message == "" {
			message = strings.TrimSpace(string(b))
		}
		if message == "" {
			message = res.Status
		}
		return res.StatusCode, errors.New(message)
	}
	if out != nil && len(b) > 0 {
		if err := json.Unmarshal(b, out); err != nil {
			return res.StatusCode, fmt.Errorf("decode cloud response: %w", err)
		}
	}
	return res.StatusCode, nil
}
