package storeedge

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type PrinterConfig struct {
	Enabled   bool   `json:"enabled"`
	Name      string `json:"name"`
	Transport string `json:"transport"` // tcp9100, file, windows_share, windows_spool_text, system_file
	Address   string `json:"address"`
	Language  string `json:"language"` // text, escpos, zpl
}

type CashDrawerConfig struct {
	Enabled  bool `json:"enabled"`
	AutoOpen bool `json:"auto_open"`
}

type POSConfig struct {
	Provider       string `json:"provider"` // manual, mock, tcp_json
	Address        string `json:"address"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type HardwareConfig struct {
	ReceiptPrinter   PrinterConfig    `json:"receipt_printer"`
	LabelPrinter     PrinterConfig    `json:"label_printer"`
	A4Printer        PrinterConfig    `json:"a4_printer"`
	CashDrawer       CashDrawerConfig `json:"cash_drawer"`
	POS              POSConfig        `json:"pos"`
	AutoPrintReceipt bool             `json:"auto_print_receipt"`
}

type ReceiptLine struct {
	Title     string  `json:"title"`
	Qty       float64 `json:"qty"`
	UnitPrice int64   `json:"unit_price"`
}

type Receipt struct {
	Number        string        `json:"number"`
	StoreName     string        `json:"store_name"`
	CreatedAt     time.Time     `json:"created_at"`
	PaymentMethod string        `json:"payment_method"`
	TotalAmount   int64         `json:"total_amount"`
	Lines         []ReceiptLine `json:"lines"`
}

type Label struct {
	Title   string `json:"title"`
	SKU     string `json:"sku,omitempty"`
	Barcode string `json:"barcode,omitempty"`
	Price   int64  `json:"price,omitempty"`
	Copies  int    `json:"copies,omitempty"`
}

type POSCharge struct {
	Amount    int64  `json:"amount"`
	Reference string `json:"reference"`
}

type POSResult struct {
	Approved         bool   `json:"approved"`
	RequiresOperator bool   `json:"requires_operator"`
	Provider         string `json:"provider"`
	RRN              string `json:"rrn,omitempty"`
	Trace            string `json:"trace,omitempty"`
	Message          string `json:"message,omitempty"`
}

func defaultHardwareConfig() HardwareConfig {
	return HardwareConfig{
		ReceiptPrinter: PrinterConfig{Language: "text"},
		LabelPrinter:   PrinterConfig{Language: "zpl"},
		A4Printer:      PrinterConfig{Language: "pdf", Transport: "system_file"},
		POS:            POSConfig{Provider: "manual", TimeoutSeconds: 30},
	}
}

func (s *Store) HardwareConfig() HardwareConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	var cfg HardwareConfig
	if err := readJSON(filepath.Join(s.dataDir, "hardware.json"), &cfg); err != nil {
		return defaultHardwareConfig()
	}
	if cfg.POS.Provider == "" {
		cfg.POS.Provider = "manual"
	}
	if cfg.POS.TimeoutSeconds <= 0 {
		cfg.POS.TimeoutSeconds = 30
	}
	return cfg
}

func (s *Store) SaveHardwareConfig(cfg HardwareConfig) error {
	if cfg.POS.Provider == "" {
		cfg.POS.Provider = "manual"
	}
	if cfg.POS.TimeoutSeconds <= 0 {
		cfg.POS.TimeoutSeconds = 30
	}
	if err := validateHardwareConfig(cfg); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return writeJSONAtomic(filepath.Join(s.dataDir, "hardware.json"), cfg, 0o600)
}

func validateHardwareConfig(cfg HardwareConfig) error {
	for name, p := range map[string]PrinterConfig{"receipt": cfg.ReceiptPrinter, "label": cfg.LabelPrinter, "a4": cfg.A4Printer} {
		if err := validatePrinterConfig(name, p); err != nil {
			return err
		}
	}
	if cfg.AutoPrintReceipt && !cfg.ReceiptPrinter.Enabled {
		return errors.New("auto print receipt requires an enabled receipt printer")
	}
	if cfg.CashDrawer.Enabled {
		if !cfg.ReceiptPrinter.Enabled {
			return errors.New("cash drawer requires an enabled receipt printer")
		}
		switch cfg.ReceiptPrinter.Transport {
		case "tcp9100", "windows_share", "file":
		default:
			return errors.New("cash drawer requires a raw receipt-printer transport")
		}
	}
	if cfg.POS.TimeoutSeconds < 1 || cfg.POS.TimeoutSeconds > 120 {
		return errors.New("POS timeout_seconds must be between 1 and 120")
	}
	switch cfg.POS.Provider {
	case "manual", "mock":
	case "tcp_json":
		if err := validateHostPort("tcp_json POS", cfg.POS.Address); err != nil {
			return err
		}
	default:
		return errors.New("POS provider must be manual, mock, or tcp_json")
	}
	return nil
}

func validatePrinterConfig(name string, p PrinterConfig) error {
	if !p.Enabled {
		return nil
	}
	p.Transport = strings.TrimSpace(p.Transport)
	p.Address = strings.TrimSpace(p.Address)
	if p.Transport == "" {
		return fmt.Errorf("%s printer transport is required", name)
	}
	switch p.Transport {
	case "tcp9100":
		return validateHostPort(name+" printer", p.Address)
	case "file":
		if !strings.EqualFold(strings.TrimSpace(os.Getenv("AUTOPARTS_EDGE_ALLOW_FILE_TRANSPORT")), "true") {
			return fmt.Errorf("%s printer file transport is disabled; enable it only for QA", name)
		}
		if p.Address == "" {
			return fmt.Errorf("%s printer file path is required", name)
		}
	case "windows_share":
		if err := validateWindowsShare(p.Address); err != nil {
			return fmt.Errorf("%s printer: %w", name, err)
		}
	case "windows_spool_text":
		if p.Address == "" {
			return fmt.Errorf("%s printer Windows name is required", name)
		}
		if strings.ContainsAny(p.Address, "\r\n") {
			return fmt.Errorf("%s printer Windows name contains control characters", name)
		}
	case "system_file":
		// Address is optional: blank means the operating-system default printer.
	default:
		return fmt.Errorf("unsupported %s printer transport", name)
	}
	return nil
}

func validateHostPort(label, address string) error {
	address = strings.TrimSpace(address)
	host, portText, err := net.SplitHostPort(address)
	if err != nil || strings.TrimSpace(host) == "" {
		return fmt.Errorf("%s address must be host:port", label)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("%s port is invalid", label)
	}
	return nil
}

func validateWindowsShare(address string) error {
	address = strings.TrimSpace(address)
	if !strings.HasPrefix(address, `\\`) {
		return errors.New(`Windows share must use \\server\printer format`)
	}
	if strings.ContainsAny(address, "\r\n&|<>^\"%'`") {
		return errors.New("Windows share contains unsafe characters")
	}
	rest := strings.TrimPrefix(address, `\\`)
	parts := strings.Split(rest, `\`)
	if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return errors.New(`Windows share must include server and printer names`)
	}
	return nil
}

type HardwareBridge struct{ store *Store }

func NewHardwareBridge(store *Store) *HardwareBridge { return &HardwareBridge{store: store} }

func (h *HardwareBridge) Status() map[string]any {
	cfg := h.store.HardwareConfig()
	return map[string]any{
		"receipt_configured":     cfg.ReceiptPrinter.Enabled,
		"label_configured":       cfg.LabelPrinter.Enabled,
		"a4_configured":          cfg.A4Printer.Enabled,
		"cash_drawer_configured": cfg.CashDrawer.Enabled,
		"pos_provider":           cfg.POS.Provider,
		"auto_print_receipt":     cfg.AutoPrintReceipt,
		"supported_transports":   []string{"tcp9100", "file", "windows_share", "windows_spool_text", "system_file"},
		"platform":               runtime.GOOS,
	}
}

func (h *HardwareBridge) PrintReceipt(ctx context.Context, r Receipt) error {
	cfg := h.store.HardwareConfig()
	p := cfg.ReceiptPrinter
	if !p.Enabled {
		return errors.New("receipt printer is not configured")
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	plain := renderReceiptText(r)
	var data []byte
	if p.Language == "escpos" {
		data = append([]byte{0x1b, 0x40}, []byte(plain)...)
		data = append(data, []byte{0x0a, 0x0a, 0x1d, 0x56, 0x00}...)
	} else {
		data = []byte(plain)
	}
	if err := sendPrinter(ctx, p, data); err != nil {
		return err
	}
	if cfg.CashDrawer.Enabled && cfg.CashDrawer.AutoOpen && r.PaymentMethod == "cash" {
		_ = h.OpenCashDrawer(ctx)
	}
	return nil
}

func (h *HardwareBridge) PrintLabel(ctx context.Context, l Label) error {
	cfg := h.store.HardwareConfig()
	p := cfg.LabelPrinter
	if !p.Enabled {
		return errors.New("label printer is not configured")
	}
	if l.Copies <= 0 {
		l.Copies = 1
	}
	if l.Copies > 100 {
		return errors.New("label copies must be at most 100")
	}
	var data []byte
	if p.Language == "zpl" {
		data = []byte(renderZPL(l))
	} else {
		data = []byte(renderLabelText(l))
	}
	for i := 0; i < l.Copies; i++ {
		if err := sendPrinter(ctx, p, data); err != nil {
			return err
		}
	}
	return nil
}

func (h *HardwareBridge) OpenCashDrawer(ctx context.Context) error {
	cfg := h.store.HardwareConfig()
	if !cfg.CashDrawer.Enabled {
		return errors.New("cash drawer is not enabled")
	}
	p := cfg.ReceiptPrinter
	if !p.Enabled {
		return errors.New("cash drawer requires receipt printer")
	}
	if p.Transport == "windows_spool_text" || p.Transport == "system_file" {
		return errors.New("cash drawer pulse requires a raw printer transport")
	}
	return sendPrinter(ctx, p, []byte{0x1b, 0x70, 0x00, 0x32, 0xfa})
}

func (h *HardwareBridge) PrintA4PDF(ctx context.Context, encoded string) error {
	cfg := h.store.HardwareConfig()
	p := cfg.A4Printer
	if !p.Enabled {
		return errors.New("A4 printer is not configured")
	}
	b, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return errors.New("document_base64 is invalid")
	}
	if len(b) == 0 || len(b) > 8<<20 || !strings.HasPrefix(string(b[:minInt(len(b), 4)]), "%PDF") {
		return errors.New("A4 document must be a PDF up to 8 MB")
	}
	f, err := os.CreateTemp("", "autoparts-a4-*.pdf")
	if err != nil {
		return err
	}
	path := f.Name()
	defer os.Remove(path)
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	_ = f.Close()
	if err != nil {
		return err
	}
	return printSystemFile(ctx, p, path)
}

func (h *HardwareBridge) ChargePOS(ctx context.Context, in POSCharge) (POSResult, error) {
	cfg := h.store.HardwareConfig().POS
	if in.Amount <= 0 {
		return POSResult{}, errors.New("POS amount must be greater than zero")
	}
	switch cfg.Provider {
	case "", "manual":
		return POSResult{Provider: "manual", RequiresOperator: true, Message: "مبلغ روی کارتخوان وارد شود و پس از تأیید اپراتور فروش نهایی شود."}, nil
	case "mock":
		if !strings.EqualFold(strings.TrimSpace(os.Getenv("AUTOPARTS_EDGE_ALLOW_MOCK_POS")), "true") {
			return POSResult{}, errors.New("mock POS is disabled; set AUTOPARTS_EDGE_ALLOW_MOCK_POS=true only for QA")
		}
		return POSResult{Provider: "mock", Approved: true, RRN: fmt.Sprintf("MOCK-%d", time.Now().Unix()), Trace: "QA", Message: "mock approval"}, nil
	case "tcp_json":
		return chargeTCPJSON(ctx, cfg, in)
	default:
		return POSResult{}, errors.New("unsupported POS provider")
	}
}

func chargeTCPJSON(ctx context.Context, cfg POSConfig, in POSCharge) (POSResult, error) {
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout < 3*time.Second {
		timeout = 30 * time.Second
	}
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", cfg.Address)
	if err != nil {
		return POSResult{}, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	payload := map[string]any{"amount": in.Amount, "reference": in.Reference, "currency": "IRT"}
	if err := json.NewEncoder(conn).Encode(payload); err != nil {
		return POSResult{}, err
	}
	var out POSResult
	if err := json.NewDecoder(bufio.NewReader(io.LimitReader(conn, 64<<10))).Decode(&out); err != nil {
		return POSResult{}, err
	}
	out.Provider = "tcp_json"
	return out, nil
}

func sendPrinter(ctx context.Context, p PrinterConfig, data []byte) error {
	switch p.Transport {
	case "tcp9100":
		d := net.Dialer{Timeout: 4 * time.Second}
		conn, err := d.DialContext(ctx, "tcp", p.Address)
		if err != nil {
			return err
		}
		defer conn.Close()
		_ = conn.SetWriteDeadline(time.Now().Add(8 * time.Second))
		_, err = conn.Write(data)
		return err
	case "file":
		f, err := os.OpenFile(p.Address, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err = f.Write(data); err == nil {
			err = f.Sync()
		}
		return err
	case "windows_share":
		if runtime.GOOS != "windows" {
			return errors.New("windows_share transport requires Windows")
		}
		f, err := os.CreateTemp("", "autoparts-print-*.bin")
		if err != nil {
			return err
		}
		path := f.Name()
		defer os.Remove(path)
		if _, err = f.Write(data); err == nil {
			err = f.Close()
		}
		if err != nil {
			return err
		}
		return exec.CommandContext(ctx, "cmd.exe", "/c", "copy", "/b", path, p.Address).Run()
	case "windows_spool_text":
		if runtime.GOOS != "windows" {
			return errors.New("windows_spool_text transport requires Windows")
		}
		f, err := os.CreateTemp("", "autoparts-receipt-*.txt")
		if err != nil {
			return err
		}
		path := f.Name()
		defer os.Remove(path)
		if _, err = f.Write(data); err == nil {
			err = f.Close()
		}
		if err != nil {
			return err
		}
		script := `Get-Content -Raw -Encoding UTF8 -LiteralPath $args[1] | Out-Printer -Name $args[0]`
		return exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-Command", script, p.Address, path).Run()
	default:
		return errors.New("printer transport does not support raw/text output")
	}
}

func printSystemFile(ctx context.Context, p PrinterConfig, path string) error {
	if p.Transport != "system_file" {
		return errors.New("A4 PDF requires system_file transport")
	}
	if runtime.GOOS == "windows" {
		script := `Start-Process -FilePath $args[0] -Verb Print -WindowStyle Hidden`
		return exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-Command", script, path).Run()
	}
	args := []string{}
	if strings.TrimSpace(p.Address) != "" {
		args = append(args, "-d", p.Address)
	}
	args = append(args, path)
	return exec.CommandContext(ctx, "lp", args...).Run()
}

func renderReceiptText(r Receipt) string {
	var b strings.Builder
	if r.StoreName != "" {
		fmt.Fprintf(&b, "%s\n", r.StoreName)
	}
	fmt.Fprintf(&b, "فاکتور: %s\n", r.Number)
	fmt.Fprintf(&b, "زمان: %s\n", r.CreatedAt.Format("2006-01-02 15:04"))
	b.WriteString(strings.Repeat("-", 38) + "\n")
	for _, x := range r.Lines {
		fmt.Fprintf(&b, "%s\n%.3g x %d = %d\n", x.Title, x.Qty, x.UnitPrice, int64(x.Qty*float64(x.UnitPrice)))
	}
	b.WriteString(strings.Repeat("-", 38) + "\n")
	fmt.Fprintf(&b, "جمع: %d تومان\n", r.TotalAmount)
	fmt.Fprintf(&b, "پرداخت: %s\n", paymentLabel(r.PaymentMethod))
	b.WriteString("سپاس از خرید شما\n")
	return b.String()
}
func renderLabelText(l Label) string {
	return fmt.Sprintf("%s\nSKU: %s\nBarcode: %s\nPrice: %d\n", l.Title, l.SKU, l.Barcode, l.Price)
}
func renderZPL(l Label) string {
	title := zplEscape(l.Title)
	sku := zplEscape(l.SKU)
	barcode := zplEscape(l.Barcode)
	return fmt.Sprintf("^XA^CI28^PW600^LL320^FO30,30^A0N,32,32^FD%s^FS^FO30,80^A0N,26,26^FDSKU: %s^FS^FO30,125^A0N,28,28^FDPrice: %d^FS^FO30,170^BY2^BCN,80,Y,N,N^FD%s^FS^XZ", title, sku, l.Price, barcode)
}
func zplEscape(v string) string { return strings.NewReplacer("^", " ", "~", " ").Replace(v) }
func paymentLabel(v string) string {
	if v == "cash" {
		return "نقد"
	}
	if v == "card" {
		return "کارت"
	}
	return v
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
