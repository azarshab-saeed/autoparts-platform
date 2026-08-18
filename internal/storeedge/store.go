package storeedge

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Store struct {
	mu      sync.Mutex
	dataDir string
	config  Config
	state   State
}

func Open(dataDir string) (*Store, error) {
	if strings.TrimSpace(dataDir) == "" {
		return nil, errors.New("edge data directory is required")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	s := &Store{dataDir: dataDir}
	s.config = Config{Listen: "127.0.0.1:17624", SyncSeconds: 15, AllowedOrigins: []string{"http://localhost:3000"}}
	_ = readJSON(filepath.Join(dataDir, "config.json"), &s.config)
	_ = readJSON(filepath.Join(dataDir, "state.json"), &s.state)
	if s.config.Listen == "" {
		s.config.Listen = "127.0.0.1:17624"
	}
	if s.config.SyncSeconds < 5 {
		s.config.SyncSeconds = 15
	}
	if s.state.Sales == nil {
		s.state.Sales = []LocalSale{}
	}
	if s.state.Snapshot.Products == nil {
		s.state.Snapshot.Products = []Product{}
	}
	return s, nil
}

func (s *Store) Config() Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config
}

func (s *Store) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneState(s.state)
}

func (s *Store) IsPaired() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config.DeviceID != "" && s.config.DeviceSecret != "" && s.config.CloudURL != ""
}

func (s *Store) SavePairing(cloudURL, deviceName string, p PairResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.CloudURL = strings.TrimRight(strings.TrimSpace(cloudURL), "/")
	s.config.DeviceName = strings.TrimSpace(deviceName)
	s.config.DeviceID = p.DeviceID
	s.config.DeviceSecret = p.DeviceSecret
	s.config.StoreID = p.StoreID
	s.config.StoreName = p.StoreName
	s.config.WarehouseID = p.WarehouseID
	if p.WebOrigin != "" && !contains(s.config.AllowedOrigins, p.WebOrigin) {
		s.config.AllowedOrigins = append(s.config.AllowedOrigins, p.WebOrigin)
	}
	return writeJSONAtomic(filepath.Join(s.dataDir, "config.json"), s.config, 0o600)
}

func (s *Store) ReplaceSnapshot(snapshot Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pending := map[string]float64{}
	for _, sale := range s.state.Sales {
		if sale.Status == "synced" {
			continue
		}
		for _, item := range sale.Items {
			pending[item.ProductID] += item.Qty
		}
	}
	for i := range snapshot.Products {
		if q := pending[snapshot.Products[i].ProductID]; q > 0 {
			snapshot.Products[i].OnHand = math.Max(0, snapshot.Products[i].OnHand-q)
			snapshot.Products[i].Available = math.Max(0, snapshot.Products[i].Available-q)
		}
	}
	s.state.Snapshot = snapshot
	return s.persistStateLocked()
}

func (s *Store) SearchCatalog(q string, limit int) []Product {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	q = normalize(q)
	out := make([]Product, 0, limit)
	for _, p := range s.state.Snapshot.Products {
		hay := normalize(strings.Join([]string{p.Title, p.SKU, p.Brand, p.OEMCode, p.Barcode}, " "))
		if q == "" || strings.Contains(hay, q) {
			out = append(out, p)
			if len(out) == limit {
				break
			}
		}
	}
	return out
}

func priceForQuantity(p Product, qty float64) int64 {
	price := p.SellingPrice
	bestMin := float64(-1)
	for _, b := range p.PriceBreaks {
		if b.MinQty <= qty && b.MinQty >= 0 && b.MinQty > bestMin {
			price = b.UnitPrice
			bestMin = b.MinQty
		}
	}
	return price
}

func (s *Store) QueueSale(paymentMethod, customerID string, items []LocalSaleItem) (LocalSale, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.config.DeviceID == "" {
		return LocalSale{}, errors.New("Store Edge is not paired with a store")
	}
	if paymentMethod != "cash" && paymentMethod != "card" {
		return LocalSale{}, errors.New("offline checkout supports cash or card only")
	}
	if len(items) == 0 || len(items) > 100 {
		return LocalSale{}, errors.New("offline sale requires 1 to 100 items")
	}
	byID := make(map[string]int, len(s.state.Snapshot.Products))
	for i := range s.state.Snapshot.Products {
		byID[s.state.Snapshot.Products[i].ProductID] = i
	}
	var total int64
	for i := range items {
		idx, ok := byID[items[i].ProductID]
		if !ok {
			return LocalSale{}, fmt.Errorf("product %s is not in the local catalog", items[i].ProductID)
		}
		if items[i].Qty <= 0 || items[i].UnitPrice < 0 || s.state.Snapshot.Products[idx].Available+1e-9 < items[i].Qty {
			return LocalSale{}, fmt.Errorf("insufficient local stock for %s", s.state.Snapshot.Products[idx].Title)
		}
		manualAllowed := s.state.Snapshot.PricingPolicy == nil || s.state.Snapshot.PricingPolicy.CashierMayOverride
		if items[i].ManualPrice && !manualAllowed {
			return LocalSale{}, errors.New("cashier price override is disabled")
		}
		if !items[i].ManualPrice && !items[i].PreservePrice {
			items[i].UnitPrice = priceForQuantity(s.state.Snapshot.Products[idx], items[i].Qty)
		}
		items[i].Title = s.state.Snapshot.Products[idx].Title
		line := int64(math.Round(items[i].Qty * float64(items[i].UnitPrice)))
		if line < 0 || total > math.MaxInt64-line {
			return LocalSale{}, errors.New("offline sale total is invalid")
		}
		total += line
	}
	for _, item := range items {
		idx := byID[item.ProductID]
		s.state.Snapshot.Products[idx].OnHand -= item.Qty
		s.state.Snapshot.Products[idx].Available -= item.Qty
	}
	s.state.Sequence++
	now := time.Now()
	id, err := randomID()
	if err != nil {
		return LocalSale{}, err
	}
	sale := LocalSale{
		LocalOperationID: id,
		LocalNumber:      fmt.Sprintf("LOCAL-%s-%06d", now.Format("20060102"), s.state.Sequence),
		CreatedAt:        now,
		PaymentMethod:    paymentMethod,
		CustomerID:       strings.TrimSpace(customerID),
		Items:            items,
		TotalAmount:      total,
		Status:           "pending",
	}
	s.state.Sales = append(s.state.Sales, sale)
	if err := s.persistStateLocked(); err != nil {
		return LocalSale{}, err
	}
	return sale, nil
}

func (s *Store) PendingSales() []LocalSale {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]LocalSale, 0)
	for _, sale := range s.state.Sales {
		if sale.Status == "pending" {
			out = append(out, sale)
		}
	}
	return out
}

func (s *Store) Sales(status string) []LocalSale {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]LocalSale, 0)
	for i := len(s.state.Sales) - 1; i >= 0; i-- {
		if status == "" || status == "all" || s.state.Sales[i].Status == status {
			out = append(out, s.state.Sales[i])
		}
	}
	return out
}

func (s *Store) MarkAttempt(localID string, serverID, status, detail string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for i := range s.state.Sales {
		if s.state.Sales[i].LocalOperationID != localID {
			continue
		}
		s.state.Sales[i].Attempts++
		s.state.Sales[i].LastAttemptAt = &now
		s.state.Sales[i].Status = status
		s.state.Sales[i].LastError = detail
		if serverID != "" {
			s.state.Sales[i].ServerSaleID = serverID
		}
		return s.persistStateLocked()
	}
	return errors.New("local sale not found")
}

func (s *Store) Retry(localID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Sales {
		if s.state.Sales[i].LocalOperationID == localID {
			if s.state.Sales[i].Status != "conflict" {
				return errors.New("only conflicted sales can be retried")
			}
			s.state.Sales[i].Status = "pending"
			s.state.Sales[i].LastError = ""
			return s.persistStateLocked()
		}
	}
	return errors.New("local sale not found")
}

func (s *Store) SetSyncResult(errText string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if errText == "" {
		s.state.LastSyncAt = &now
	}
	s.state.LastSyncError = errText
	return s.persistStateLocked()
}

func (s *Store) persistStateLocked() error {
	return writeJSONAtomic(filepath.Join(s.dataDir, "state.json"), s.state, 0o600)
}

func readJSON(path string, dst any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}

func writeJSONAtomic(path string, v any, mode os.FileMode) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func cloneState(in State) State {
	b, _ := json.Marshal(in)
	var out State
	_ = json.Unmarshal(b, &out)
	return out
}

func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func normalize(v string) string {
	r := strings.NewReplacer("ي", "ی", "ى", "ی", "ك", "ک", "۰", "0", "۱", "1", "۲", "2", "۳", "3", "۴", "4", "۵", "5", "۶", "6", "۷", "7", "۸", "8", "۹", "9", "٠", "0", "١", "1", "٢", "2", "٣", "3", "٤", "4", "٥", "5", "٦", "6", "٧", "7", "٨", "8", "٩", "9")
	return strings.ToLower(strings.Join(strings.Fields(r.Replace(strings.TrimSpace(v))), " "))
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
