package storeedgemanager

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Updater struct {
	mu             sync.Mutex
	supervisor     *Supervisor
	managerVersion string
	managerPath    string
	updaterPath    string
	manifestURL    string
	publicKey      ed25519.PublicKey
	dataDir        string
	serviceMode    string
	serviceName    string
	state          string
	latest         *UpdateManifest
	lastErr        string
}

func NewUpdater(supervisor *Supervisor, managerVersion, managerPath, updaterPath, manifestURL, publicKeyBase64, dataDir, serviceMode, serviceName string) (*Updater, error) {
	u := &Updater{
		supervisor: supervisor, managerVersion: managerVersion, managerPath: managerPath, updaterPath: updaterPath,
		manifestURL: strings.TrimSpace(manifestURL), dataDir: dataDir, serviceMode: serviceMode, serviceName: serviceName, state: "idle",
	}
	if strings.TrimSpace(publicKeyBase64) != "" {
		b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(publicKeyBase64))
		if err != nil {
			return nil, fmt.Errorf("invalid update public key: %w", err)
		}
		if len(b) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("invalid update public key length: %d", len(b))
		}
		u.publicKey = ed25519.PublicKey(b)
	}
	return u, nil
}

func (u *Updater) Enabled() bool {
	return u.manifestURL != "" && len(u.publicKey) == ed25519.PublicKeySize
}
func (u *Updater) ManifestURL() string { return u.manifestURL }

func (u *Updater) State() (state, latest, lastErr string, available bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	state, lastErr = u.state, u.lastErr
	if u.latest != nil {
		latest = u.latest.Version
		available = compareVersions(latest, u.currentVersionLocked()) > 0
	}
	return
}

func (u *Updater) currentVersionLocked() string {
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	st := u.supervisor.Status(ctx)
	if st.Version != "" {
		return st.Version
	}
	return u.managerVersion
}

func (u *Updater) Check(ctx context.Context) (UpdateCheck, error) {
	if !u.Enabled() {
		return UpdateCheck{}, errors.New("signed Store Agent updates are not configured for this installation")
	}
	u.mu.Lock()
	u.state = "checking"
	u.lastErr = ""
	u.mu.Unlock()

	manifest, err := u.fetchManifest(ctx)
	if err != nil {
		u.setError(err)
		return UpdateCheck{}, err
	}
	current := u.supervisor.Status(ctx).Version
	if current == "" {
		current = u.managerVersion
	}
	available := compareVersions(manifest.Version, current) > 0
	u.mu.Lock()
	u.latest = &manifest
	u.state = "idle"
	u.lastErr = ""
	u.mu.Unlock()
	return UpdateCheck{CurrentVersion: current, LatestVersion: manifest.Version, UpdateAvailable: available, ReleaseNotes: manifest.ReleaseNotes, PublishedAt: manifest.PublishedAt}, nil
}

func (u *Updater) Apply(ctx context.Context) (string, error) {
	if !u.Enabled() {
		return "", errors.New("signed Store Agent updates are not configured for this installation")
	}
	u.mu.Lock()
	if u.state == "downloading" || u.state == "applying" {
		u.mu.Unlock()
		return "", errors.New("an update is already in progress")
	}
	u.state = "downloading"
	u.lastErr = ""
	u.mu.Unlock()

	manifest, err := u.fetchManifest(ctx)
	if err != nil {
		u.setError(err)
		return "", err
	}
	current := u.supervisor.Status(ctx).Version
	if current == "" {
		current = u.managerVersion
	}
	if compareVersions(manifest.Version, current) <= 0 {
		u.mu.Lock()
		u.state = "idle"
		u.latest = &manifest
		u.mu.Unlock()
		return manifest.Version, nil
	}

	stageDir := filepath.Join(u.dataDir, "updates", sanitizeVersion(manifest.Version))
	if err := os.MkdirAll(stageDir, 0o700); err != nil {
		u.setError(err)
		return "", err
	}
	workerNew := filepath.Join(stageDir, filepath.Base(u.supervisor.WorkerPath())+".new")
	if err := u.downloadAndVerify(ctx, manifest.Version, manifest.Platform, "worker", manifest.Worker, workerNew); err != nil {
		u.setError(err)
		return "", err
	}
	managerNew := ""
	if strings.TrimSpace(manifest.Manager.URL) != "" {
		managerNew = filepath.Join(stageDir, filepath.Base(u.managerPath)+".new")
		if err := u.downloadAndVerify(ctx, manifest.Version, manifest.Platform, "manager", manifest.Manager, managerNew); err != nil {
			u.setError(err)
			return "", err
		}
	}

	u.mu.Lock()
	u.state = "applying"
	u.latest = &manifest
	u.mu.Unlock()
	if managerNew == "" {
		stopCtx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		if err := u.supervisor.Stop(stopCtx); err != nil {
			u.setError(err)
			return "", err
		}
		if err := replaceExecutable(u.supervisor.WorkerPath(), workerNew); err != nil {
			u.setError(err)
			_ = u.supervisor.Start()
			return "", err
		}
		if err := u.supervisor.Start(); err != nil {
			u.setError(err)
			return "", err
		}
		u.mu.Lock()
		u.state = "idle"
		u.lastErr = ""
		u.mu.Unlock()
		return manifest.Version, nil
	}

	if u.serviceMode == "none" {
		err := errors.New("manager self-update requires an installed Windows or systemd service")
		u.setError(err)
		return "", err
	}
	if _, err := os.Stat(u.updaterPath); err != nil {
		err = fmt.Errorf("Store Agent updater helper not found: %w", err)
		u.setError(err)
		return "", err
	}
	args := []string{
		"--pid", strconv.Itoa(os.Getpid()),
		"--manager-current", u.managerPath,
		"--manager-new", managerNew,
		"--worker-current", u.supervisor.WorkerPath(),
		"--worker-new", workerNew,
		"--service-mode", u.serviceMode,
		"--service-name", u.serviceName,
		"--log", filepath.Join(stageDir, "updater.log"),
	}
	if err := launchUpdater(u.updaterPath, args, u.serviceMode); err != nil {
		u.setError(err)
		return "", err
	}
	return manifest.Version, nil
}

func (u *Updater) fetchManifest(ctx context.Context) (UpdateManifest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.manifestURL, nil)
	if err != nil {
		return UpdateManifest{}, err
	}
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return UpdateManifest{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return UpdateManifest{}, fmt.Errorf("update manifest HTTP %d", resp.StatusCode)
	}
	var m UpdateManifest
	dec := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return UpdateManifest{}, err
	}
	expected := runtime.GOOS + "-" + runtime.GOARCH
	if m.Platform != expected {
		return UpdateManifest{}, fmt.Errorf("update manifest platform %q does not match %q", m.Platform, expected)
	}
	if strings.TrimSpace(m.Version) == "" || strings.TrimSpace(m.Worker.URL) == "" {
		return UpdateManifest{}, errors.New("update manifest is incomplete")
	}
	return m, nil
}

func (u *Updater) downloadAndVerify(ctx context.Context, version, platform, component string, asset UpdateAsset, dst string) error {
	if strings.TrimSpace(asset.URL) == "" || strings.TrimSpace(asset.SHA256) == "" || strings.TrimSpace(asset.Signature) == "" {
		return fmt.Errorf("%s update asset is incomplete", component)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return err
	}
	resp, err := (&http.Client{Timeout: 90 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s download HTTP %d", component, resp.StatusCode)
	}
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o700)
	if err != nil {
		return err
	}
	h := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, 128<<20))
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	got := hex.EncodeToString(h.Sum(nil))
	want := strings.ToLower(strings.TrimSpace(asset.SHA256))
	if got != want {
		return fmt.Errorf("%s SHA256 mismatch", component)
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(asset.Signature))
	if err != nil {
		return fmt.Errorf("invalid %s signature encoding: %w", component, err)
	}
	message := signatureMessage(version, platform, component, want)
	if !ed25519.Verify(u.publicKey, []byte(message), sig) {
		return fmt.Errorf("%s signature verification failed", component)
	}
	return nil
}

func signatureMessage(version, platform, component, sha string) string {
	return version + "\n" + platform + "\n" + component + "\n" + strings.ToLower(strings.TrimSpace(sha))
}

func (u *Updater) setError(err error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.state = "error"
	u.lastErr = err.Error()
}

func replaceExecutable(current, staged string) error {
	backup := current + ".bak"
	_ = os.Remove(backup)
	if err := os.Rename(current, backup); err != nil {
		return err
	}
	if err := os.Rename(staged, current); err != nil {
		_ = os.Rename(backup, current)
		return err
	}
	_ = os.Chmod(current, 0o755)
	_ = os.Remove(backup)
	return nil
}

func launchUpdater(path string, args []string, mode string) error {
	if mode == "systemd-user" {
		unit := "autoparts-store-edge-updater-" + strconv.FormatInt(time.Now().Unix(), 10)
		full := append([]string{"--user", "--unit=" + unit, "--collect", path}, args...)
		cmd := exec.Command("systemd-run", full...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("systemd-run updater: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	cmd := exec.Command(path, args...)
	return cmd.Start()
}

func compareVersions(a, b string) int {
	pa := parseVersion(a)
	pb := parseVersion(b)
	for i := 0; i < 4; i++ {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	return 0
}

func parseVersion(v string) [4]int {
	v = strings.TrimSpace(strings.TrimPrefix(v, "v"))
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	var out [4]int
	for i := 0; i < len(parts) && i < 4; i++ {
		out[i], _ = strconv.Atoi(parts[i])
	}
	return out
}

func sanitizeVersion(v string) string {
	var b strings.Builder
	for _, r := range v {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}
