package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
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

var version = "0.15.7"

//go:embed offline.html
var assets embed.FS

func main() {
	dataDir := strings.TrimSpace(os.Getenv("AUTOPARTS_EDGE_DATA_DIR"))
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
		dataDir = filepath.Join(home, ".autoparts-store-edge")
	}
	store, err := storeedge.Open(dataDir)
	if err != nil {
		log.Fatal(err)
	}
	cloud := storeedge.NewCloud()
	syncer := storeedge.NewSyncer(store, cloud)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		b, _ := assets.ReadFile("offline.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "autoparts-store-edge", "version": version})
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
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		paired, err := cloud.Pair(ctx, in.CloudURL, in.PairCode, in.DeviceName)
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		if err := store.SavePairing(in.CloudURL, in.DeviceName, paired); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if err := syncer.Run(ctx); err != nil {
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
		writeJSON(w, http.StatusCreated, out)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			_ = syncer.Run(ctx)
		}()
	})
	mux.HandleFunc("POST /v1/offline-sales/{id}/retry", func(w http.ResponseWriter, r *http.Request) {
		if err := store.Retry(r.PathValue("id")); err != nil {
			writeError(w, http.StatusConflict, err)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		if err := syncer.Run(ctx); err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "retry_started"})
	})
	mux.HandleFunc("POST /v1/sync", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		if err := syncer.Run(ctx); err != nil {
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
		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			_ = syncer.Run(ctx)
			cancel()
		}
	}()
	go func() {
		log.Printf("AutoParts Store Edge %s listening on http://%s data=%s os=%s", version, cfg.Listen, dataDir, runtime.GOOS)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
}

func localCORS(store *storeedge.Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" {
			allowed := false
			for _, v := range store.Config().AllowedOrigins {
				if v == origin {
					allowed = true
					break
				}
			}
			if !allowed {
				writeError(w, http.StatusForbidden, errors.New("origin is not trusted by Store Edge"))
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-AutoParts-Edge")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
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
