package main

import (
	"context"
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
	"strings"
	"syscall"
	"time"

	"github.com/example/autoparts-core/internal/storeedgemanager"
)

var (
	version                  = "0.15.8.2"
	defaultUpdateManifestURL = ""
	defaultUpdatePublicKey   = ""
	defaultAllowedOrigins    = ""
)

const (
	windowsManagerServiceName = "AutoPartsStoreEdgeManager"
	linuxManagerServiceName   = "autoparts-store-edge-manager.service"
	managerListenDefault      = "127.0.0.1:17623"
	workerURLDefault          = "http://127.0.0.1:17624"
)

func main() {
	if len(os.Args) > 1 {
		switch strings.ToLower(strings.TrimSpace(os.Args[1])) {
		case "version", "--version", "-version":
			fmt.Println(version)
			return
		case "service":
			if runtime.GOOS != "windows" {
				log.Fatal("service mode is only available on Windows")
			}
			if err := runWindowsManagerService(windowsManagerServiceName, runManager); err != nil {
				log.Fatal(err)
			}
			return
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := runManager(ctx, false); err != nil {
		log.Fatal(err)
	}
}

func runManager(ctx context.Context, windowsService bool) error {
	serviceMode := strings.TrimSpace(os.Getenv("AUTOPARTS_EDGE_MANAGER_SERVICE"))
	if windowsService {
		serviceMode = "windows"
	}
	if serviceMode == "" {
		serviceMode = "none"
	}
	dataDir, err := managerDataDir(windowsService)
	if err != nil {
		return err
	}
	logDir := filepath.Join(filepath.Dir(dataDir), "logs")
	closeLog, err := configureManagerLogging(logDir, windowsService || serviceMode == "systemd-user")
	if err != nil {
		return err
	}
	defer closeLog()

	managerPath, err := os.Executable()
	if err != nil {
		return err
	}
	managerPath, _ = filepath.Abs(managerPath)
	workerPath := strings.TrimSpace(os.Getenv("AUTOPARTS_EDGE_WORKER_PATH"))
	if workerPath == "" {
		name := "autoparts-store-edge"
		if runtime.GOOS == "windows" {
			name = "AutoPartsStoreEdge.exe"
		}
		workerPath = filepath.Join(filepath.Dir(managerPath), name)
	}
	updaterPath := strings.TrimSpace(os.Getenv("AUTOPARTS_EDGE_UPDATER_PATH"))
	if updaterPath == "" {
		name := "autoparts-store-edge-updater"
		if runtime.GOOS == "windows" {
			name = "AutoPartsStoreEdgeUpdater.exe"
		}
		updaterPath = filepath.Join(filepath.Dir(managerPath), name)
	}
	workerURL := strings.TrimSpace(os.Getenv("AUTOPARTS_EDGE_WORKER_URL"))
	if workerURL == "" {
		workerURL = workerURLDefault
	}
	listen := strings.TrimSpace(os.Getenv("AUTOPARTS_EDGE_MANAGER_LISTEN"))
	if listen == "" {
		listen = managerListenDefault
	}
	manifestURL := strings.TrimSpace(os.Getenv("AUTOPARTS_EDGE_UPDATE_MANIFEST_URL"))
	if manifestURL == "" {
		manifestURL = defaultUpdateManifestURL
	}
	publicKey := strings.TrimSpace(os.Getenv("AUTOPARTS_EDGE_UPDATE_PUBLIC_KEY"))
	if publicKey == "" {
		publicKey = defaultUpdatePublicKey
	}

	supervisor := storeedgemanager.NewSupervisor(workerPath, workerURL, dataDir, logDir)
	if err := supervisor.Start(); err != nil {
		log.Printf("initial worker start failed: %v", err)
	}
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = supervisor.Close(shutdown)
	}()

	updater, err := storeedgemanager.NewUpdater(supervisor, version, managerPath, updaterPath, manifestURL, publicKey, dataDir, serviceMode, managerServiceName(serviceMode))
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "autoparts-store-edge-manager", "version": version, "os": runtime.GOOS, "arch": runtime.GOARCH, "service_mode": serviceMode})
	})
	mux.HandleFunc("GET /v1/lifecycle/status", func(w http.ResponseWriter, r *http.Request) {
		state, latest, lastErr, available := updater.State()
		workerCtx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		writeJSON(w, http.StatusOK, storeedgemanager.LifecycleStatus{
			ManagerVersion: version, OS: runtime.GOOS, Arch: runtime.GOARCH, ServiceMode: serviceMode,
			Worker: supervisor.Status(workerCtx), UpdateEnabled: updater.Enabled(), UpdateState: state,
			LatestVersion: latest, UpdateAvailable: available, LastUpdateError: lastErr, ManifestURL: redactURL(updater.ManifestURL()),
		})
	})
	mux.HandleFunc("POST /v1/lifecycle/start", func(w http.ResponseWriter, r *http.Request) {
		if err := supervisor.Start(); err != nil {
			writeError(w, http.StatusConflict, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "starting"})
	})
	mux.HandleFunc("POST /v1/lifecycle/stop", func(w http.ResponseWriter, r *http.Request) {
		stopCtx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
		defer cancel()
		if err := supervisor.Stop(stopCtx); err != nil {
			writeError(w, http.StatusConflict, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
	})
	mux.HandleFunc("POST /v1/lifecycle/restart", func(w http.ResponseWriter, r *http.Request) {
		restartCtx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		if err := supervisor.Restart(restartCtx); err != nil {
			writeError(w, http.StatusConflict, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "restarting"})
	})
	mux.HandleFunc("POST /v1/lifecycle/update/check", func(w http.ResponseWriter, r *http.Request) {
		checkCtx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		out, err := updater.Check(checkCtx)
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("POST /v1/lifecycle/update/apply", func(w http.ResponseWriter, r *http.Request) {
		applyCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		v, err := updater.Apply(applyCtx)
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "applying", "version": v})
	})

	handler := managerCORS(dataDir, mux)
	srv := &http.Server{Addr: listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 125 * time.Second}
	errCh := make(chan error, 1)
	go func() {
		log.Printf("AutoParts Store Edge Manager %s listening on http://%s worker=%s os=%s service=%s", version, listen, workerPath, runtime.GOOS, serviceMode)
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()
	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil {
			return err
		}
		return nil
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return srv.Shutdown(shutdown)
}

func managerCORS(dataDir string, next http.Handler) http.Handler {
	allowed := map[string]bool{
		"http://localhost:3000": true,
		"http://127.0.0.1:3000": true,
	}
	for _, source := range []string{defaultAllowedOrigins, os.Getenv("AUTOPARTS_EDGE_ALLOWED_ORIGINS")} {
		for _, v := range strings.Split(source, ",") {
			if x := strings.TrimSpace(v); x != "" {
				allowed[x] = true
			}
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" {
			trusted := allowed[origin] || sameManagerOrigin(origin, r.Host) || pairedOriginAllowed(dataDir, origin)
			if !trusted {
				writeError(w, http.StatusForbidden, errors.New("origin is not trusted by Store Edge Manager"))
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-AutoParts-Edge")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			if strings.EqualFold(r.Header.Get("Access-Control-Request-Private-Network"), "true") {
				w.Header().Set("Access-Control-Allow-Private-Network", "true")
			}
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodGet && r.Header.Get("X-AutoParts-Edge") != "1" {
			writeError(w, http.StatusForbidden, errors.New("missing Store Edge control header"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func pairedOriginAllowed(dataDir, origin string) bool {
	b, err := os.ReadFile(filepath.Join(dataDir, "config.json"))
	if err != nil {
		return false
	}
	var cfg struct {
		AllowedOrigins []string `json:"allowed_origins"`
	}
	if json.Unmarshal(b, &cfg) != nil {
		return false
	}
	for _, v := range cfg.AllowedOrigins {
		if strings.TrimSpace(v) == origin {
			return true
		}
	}
	return false
}

func sameManagerOrigin(origin, requestHost string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" || u.Host == "" || u.Host != requestHost {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func managerDataDir(windowsService bool) (string, error) {
	if v := strings.TrimSpace(os.Getenv("AUTOPARTS_EDGE_DATA_DIR")); v != "" {
		return v, nil
	}
	if runtime.GOOS == "windows" && windowsService {
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

func configureManagerLogging(logDir string, enabled bool) (func(), error) {
	if !enabled {
		return func() {}, nil
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(logDir, "manager.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	return func() { _ = f.Close() }, nil
}

func managerServiceName(mode string) string {
	if mode == "windows" {
		return windowsManagerServiceName
	}
	if mode == "systemd-user" {
		return linuxManagerServiceName
	}
	return ""
}

func redactURL(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "configured"
	}
	u.RawQuery = ""
	u.Fragment = ""
	u.User = nil
	return u.String()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"message": err.Error()}})
}
func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
