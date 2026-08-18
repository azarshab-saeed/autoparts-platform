package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/example/autoparts-core/internal/storeedge"
)

var version = "0.15.8.2"

const windowsServiceName = "AutoPartsStoreEdge"

//go:embed offline.html
var assets embed.FS

func main() {
	if len(os.Args) > 1 {
		switch strings.ToLower(strings.TrimSpace(os.Args[1])) {
		case "version", "--version", "-version":
			fmt.Println(version)
			return
		case "open", "--open":
			if err := openLocalUI(); err != nil {
				log.Fatal(err)
			}
			return
		case "service":
			if runtime.GOOS != "windows" {
				log.Fatal("service mode is only available on Windows")
			}
			if err := runWindowsService(windowsServiceName, runAgent); err != nil {
				log.Fatal(err)
			}
			return
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := runAgent(ctx, false); err != nil {
		log.Fatal(err)
	}
}

func runAgent(ctx context.Context, serviceMode bool) error {
	agentCtx, cancelAgent := context.WithCancel(ctx)
	defer cancelAgent()
	managerToken := strings.TrimSpace(os.Getenv("AUTOPARTS_EDGE_MANAGER_TOKEN"))
	dataDir, err := edgeDataDir(serviceMode)
	if err != nil {
		return err
	}
	closeLog, err := configureAgentLogging(dataDir, serviceMode)
	if err != nil {
		return err
	}
	defer closeLog()

	store, err := storeedge.Open(dataDir)
	if err != nil {
		return err
	}
	cloud := storeedge.NewCloud()
	syncer := storeedge.NewSyncer(store, cloud)
	bridge := storeedge.NewHardwareBridge(store)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		b, _ := assets.ReadFile("offline.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "autoparts-store-edge", "version": version, "mode": map[bool]string{true: "service", false: "console"}[serviceMode]})
	})
	mux.HandleFunc("POST /v1/admin/shutdown", func(w http.ResponseWriter, r *http.Request) {
		if managerToken == "" || r.Header.Get("X-AutoParts-Manager-Token") != managerToken {
			writeError(w, http.StatusForbidden, errors.New("manager authorization failed"))
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "shutting_down"})
		go func() {
			time.Sleep(80 * time.Millisecond)
			cancelAgent()
		}()
	})
	mux.HandleFunc("GET /v1/status", func(w http.ResponseWriter, r *http.Request) {
		st := store.State()
		cfg := store.Config()
		pending, conflicts := 0, 0
		for _, sale := range st.Sales {
			if sale.Status == "pending" {
				pending++
			}
			if sale.Status == "conflict" {
				conflicts++
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"paired": store.IsPaired(), "store_name": cfg.StoreName, "device_name": cfg.DeviceName,
			"pending_sales": pending, "conflicts": conflicts, "last_sync_at": st.LastSyncAt,
			"last_sync_error": st.LastSyncError, "catalog_items": len(st.Snapshot.Products), "snapshot_at": st.Snapshot.GeneratedAt,
		})
	})
	mux.HandleFunc("POST /v1/pair", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			CloudURL   string `json:"cloud_url"`
			PairCode   string `json:"pair_code"`
			DeviceName string `json:"device_name"`
		}
		if err := decodeJSON(r, &in); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if strings.TrimSpace(in.DeviceName) == "" {
			in.DeviceName = hostname()
		}
		requestCtx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		paired, err := cloud.Pair(requestCtx, in.CloudURL, in.PairCode, in.DeviceName)
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		if err := store.SavePairing(in.CloudURL, in.DeviceName, paired); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if err := syncer.Run(requestCtx); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"paired": true, "store_name": paired.StoreName, "sync_warning": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"paired": true, "store_name": paired.StoreName})
	})
	mux.HandleFunc("GET /v1/catalog", func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		writeJSON(w, http.StatusOK, map[string]any{"items": store.SearchCatalog(r.URL.Query().Get("q"), limit)})
	})
	mux.HandleFunc("GET /v1/offline-sales", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"items": store.Sales(r.URL.Query().Get("status"))})
	})
	mux.HandleFunc("POST /v1/offline-sales", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			PaymentMethod string                    `json:"payment_method"`
			Items         []storeedge.LocalSaleItem `json:"items"`
		}
		if err := decodeJSON(r, &in); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		out, err := store.QueueSale(in.PaymentMethod, in.Items)
		if err != nil {
			writeError(w, http.StatusConflict, err)
			return
		}
		hw := store.HardwareConfig()
		if hw.AutoPrintReceipt && hw.ReceiptPrinter.Enabled {
			lines := make([]storeedge.ReceiptLine, 0, len(out.Items))
			for _, x := range out.Items {
				lines = append(lines, storeedge.ReceiptLine{Title: x.Title, Qty: x.Qty, UnitPrice: x.UnitPrice})
			}
			if err := bridge.PrintReceipt(r.Context(), storeedge.Receipt{Number: out.LocalNumber, StoreName: store.Config().StoreName, CreatedAt: out.CreatedAt, PaymentMethod: out.PaymentMethod, TotalAmount: out.TotalAmount, Lines: lines}); err != nil {
				log.Printf("hardware receipt print warning: %v", err)
			}
		}
		writeJSON(w, http.StatusCreated, out)
		go func() {
			syncCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			_ = syncer.Run(syncCtx)
		}()
	})
	mux.HandleFunc("POST /v1/offline-sales/{id}/retry", func(w http.ResponseWriter, r *http.Request) {
		if err := store.Retry(r.PathValue("id")); err != nil {
			writeError(w, http.StatusConflict, err)
			return
		}
		requestCtx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		if err := syncer.Run(requestCtx); err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "retry_started"})
	})
	mux.HandleFunc("GET /v1/hardware/config", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, store.HardwareConfig())
	})
	mux.HandleFunc("PUT /v1/hardware/config", func(w http.ResponseWriter, r *http.Request) {
		var cfg storeedge.HardwareConfig
		if err := decodeJSON(r, &cfg); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := store.SaveHardwareConfig(cfg); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, cfg)
	})
	mux.HandleFunc("GET /v1/hardware/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, bridge.Status())
	})
	mux.HandleFunc("POST /v1/hardware/receipt/print", func(w http.ResponseWriter, r *http.Request) {
		var in storeedge.Receipt
		if err := decodeJSON(r, &in); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if in.StoreName == "" {
			in.StoreName = store.Config().StoreName
		}
		if err := bridge.PrintReceipt(r.Context(), in); err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "printed"})
	})
	mux.HandleFunc("POST /v1/hardware/label/print", func(w http.ResponseWriter, r *http.Request) {
		var in storeedge.Label
		if err := decodeJSON(r, &in); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := bridge.PrintLabel(r.Context(), in); err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "printed"})
	})
	mux.HandleFunc("POST /v1/hardware/cash-drawer/open", func(w http.ResponseWriter, r *http.Request) {
		if err := bridge.OpenCashDrawer(r.Context()); err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "opened"})
	})
	mux.HandleFunc("POST /v1/hardware/a4/print", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			DocumentBase64 string `json:"document_base64"`
		}
		if err := decodeJSONLimit(r, &in, 12<<20); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := bridge.PrintA4PDF(r.Context(), in.DocumentBase64); err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "queued"})
	})
	mux.HandleFunc("POST /v1/hardware/pos/charge", func(w http.ResponseWriter, r *http.Request) {
		var in storeedge.POSCharge
		if err := decodeJSON(r, &in); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		out, err := bridge.ChargePOS(r.Context(), in)
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})

	mux.HandleFunc("POST /v1/sync", func(w http.ResponseWriter, r *http.Request) {
		requestCtx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		if err := syncer.Run(requestCtx); err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "synced"})
	})

	handler := localCORS(store, mux)
	cfg := store.Config()
	srv := &http.Server{Addr: cfg.Listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second}

	go func() {
		ticker := time.NewTicker(time.Duration(cfg.SyncSeconds) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-agentCtx.Done():
				return
			case <-ticker.C:
				syncCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				_ = syncer.Run(syncCtx)
				cancel()
			}
		}
	}()

	serveErr := make(chan error, 1)
	go func() {
		log.Printf("AutoParts Store Edge %s listening on http://%s data=%s os=%s service=%t", version, cfg.Listen, dataDir, runtime.GOOS, serviceMode)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case <-agentCtx.Done():
	case err := <-serveErr:
		if err != nil {
			return err
		}
		return nil
	}

	shutdown, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return srv.Shutdown(shutdown)
}

func edgeDataDir(serviceMode bool) (string, error) {
	if v := strings.TrimSpace(os.Getenv("AUTOPARTS_EDGE_DATA_DIR")); v != "" {
		return v, nil
	}
	if runtime.GOOS == "windows" && serviceMode {
		base := strings.TrimSpace(os.Getenv("PROGRAMDATA"))
		if base == "" {
			base = `C:\ProgramData`
		}
		return filepath.Join(base, "AutoParts", "StoreEdge", "data"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".autoparts-store-edge"), nil
}

func configureAgentLogging(dataDir string, serviceMode bool) (func(), error) {
	if !serviceMode {
		return func() {}, nil
	}
	logDir := filepath.Join(filepath.Dir(dataDir), "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(logDir, "agent.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	return func() { _ = f.Close() }, nil
}

func localCORS(store *storeedge.Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" {
			allowed := sameLoopbackOrigin(origin, r.Host)
			if !allowed {
				for _, v := range store.Config().AllowedOrigins {
					if v == origin {
						allowed = true
						break
					}
				}
			}
			if !allowed {
				writeError(w, http.StatusForbidden, errors.New("origin is not trusted by Store Edge"))
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-AutoParts-Edge")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
			if strings.EqualFold(r.Header.Get("Access-Control-Request-Private-Network"), "true") {
				w.Header().Set("Access-Control-Allow-Private-Network", "true")
			}
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// sameLoopbackOrigin allows the Store Edge UI served by the agent itself to call
// its local API. It deliberately requires the exact same host:port and a loopback
// hostname, so another local web app on a different port cannot bypass the
// configured origin allowlist.
func sameLoopbackOrigin(origin, requestHost string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" || u.Host == "" || u.Host != requestHost {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func decodeJSONLimit(r *http.Request, dst any, limit int64) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, limit))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"message": err.Error()}})
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil || strings.TrimSpace(h) == "" {
		return "Store PC"
	}
	return h
}
