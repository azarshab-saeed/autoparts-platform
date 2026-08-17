package storeedge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReceiptPrinterFileTransport(t *testing.T) {
	t.Setenv("AUTOPARTS_EDGE_ALLOW_FILE_TRANSPORT", "true")
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "receipt.txt")
	cfg := defaultHardwareConfig()
	cfg.ReceiptPrinter = PrinterConfig{Enabled: true, Name: "qa", Transport: "file", Address: out, Language: "text"}
	if err := s.SaveHardwareConfig(cfg); err != nil {
		t.Fatal(err)
	}
	b := NewHardwareBridge(s)
	err = b.PrintReceipt(context.Background(), Receipt{Number: "LOCAL-1", StoreName: "Demo", CreatedAt: time.Now(), PaymentMethod: "cash", TotalAmount: 1000, Lines: []ReceiptLine{{Title: "Brake", Qty: 1, UnitPrice: 1000}}})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "LOCAL-1") || !strings.Contains(string(data), "Brake") {
		t.Fatalf("unexpected receipt: %s", data)
	}
}

func TestLabelZPLContainsBarcode(t *testing.T) {
	z := renderZPL(Label{Title: "Brake", SKU: "BRK", Barcode: "626123", Price: 100})
	if !strings.Contains(z, "^BC") || !strings.Contains(z, "626123") {
		t.Fatalf("unexpected zpl: %s", z)
	}
}

func TestPOSManualRequiresOperator(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	out, err := NewHardwareBridge(s).ChargePOS(context.Background(), POSCharge{Amount: 100, Reference: "s1"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Approved || !out.RequiresOperator || out.Provider != "manual" {
		t.Fatalf("unexpected result: %+v", out)
	}
}

func TestHardwareConfigRejectsUnknownTransport(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := defaultHardwareConfig()
	cfg.ReceiptPrinter = PrinterConfig{Enabled: true, Transport: "magic", Address: "x"}
	if err := s.SaveHardwareConfig(cfg); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestPOSMockRequiresExplicitQAFlag(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := defaultHardwareConfig()
	cfg.POS.Provider = "mock"
	if err := s.SaveHardwareConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := NewHardwareBridge(s).ChargePOS(context.Background(), POSCharge{Amount: 100}); err == nil {
		t.Fatal("expected mock provider to be disabled")
	}
	t.Setenv("AUTOPARTS_EDGE_ALLOW_MOCK_POS", "true")
	out, err := NewHardwareBridge(s).ChargePOS(context.Background(), POSCharge{Amount: 100})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Approved {
		t.Fatal("mock should approve when explicitly enabled")
	}
}

func TestFileTransportRequiresExplicitQAFlag(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := defaultHardwareConfig()
	cfg.ReceiptPrinter = PrinterConfig{Enabled: true, Transport: "file", Address: filepath.Join(t.TempDir(), "receipt.txt"), Language: "text"}
	if err := s.SaveHardwareConfig(cfg); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("expected file transport to be gated, got %v", err)
	}
}

func TestSystemFilePrinterMayUseDefaultPrinter(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := defaultHardwareConfig()
	cfg.A4Printer = PrinterConfig{Enabled: true, Transport: "system_file", Language: "pdf"}
	if err := s.SaveHardwareConfig(cfg); err != nil {
		t.Fatalf("default system printer should be valid: %v", err)
	}
}

func TestCashDrawerRejectsNonRawReceiptTransport(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := defaultHardwareConfig()
	cfg.ReceiptPrinter = PrinterConfig{Enabled: true, Transport: "windows_spool_text", Address: "Receipt Printer", Language: "text"}
	cfg.CashDrawer.Enabled = true
	if err := s.SaveHardwareConfig(cfg); err == nil || !strings.Contains(err.Error(), "raw") {
		t.Fatalf("expected cash drawer/raw transport validation, got %v", err)
	}
}

func TestWindowsShareRejectsShellMetacharacters(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := defaultHardwareConfig()
	cfg.ReceiptPrinter = PrinterConfig{Enabled: true, Transport: "windows_share", Address: `\\server\printer&whoami`, Language: "text"}
	if err := s.SaveHardwareConfig(cfg); err == nil || !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("expected unsafe share to be rejected, got %v", err)
	}
}

func TestTCPPrinterRequiresValidHostPort(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := defaultHardwareConfig()
	cfg.ReceiptPrinter = PrinterConfig{Enabled: true, Transport: "tcp9100", Address: "192.168.1.20", Language: "escpos"}
	if err := s.SaveHardwareConfig(cfg); err == nil || !strings.Contains(err.Error(), "host:port") {
		t.Fatalf("expected host:port validation, got %v", err)
	}
}
