package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/example/autoparts-core/internal/storeedgemanager"
)

func main() {
	var version, platform, workerPath, workerURL, managerPath, managerURL, notes, out string
	flag.StringVar(&version, "version", "", "release version")
	flag.StringVar(&platform, "platform", "", "platform such as windows-amd64")
	flag.StringVar(&workerPath, "worker", "", "worker binary path")
	flag.StringVar(&workerURL, "worker-url", "", "public worker URL")
	flag.StringVar(&managerPath, "manager", "", "manager binary path")
	flag.StringVar(&managerURL, "manager-url", "", "public manager URL")
	flag.StringVar(&notes, "notes", "", "release notes")
	flag.StringVar(&out, "out", "update-manifest.json", "output manifest")
	flag.Parse()
	if version == "" || platform == "" || workerPath == "" || workerURL == "" {
		fatal("version, platform, worker and worker-url are required")
	}
	priv, err := privateKey()
	if err != nil {
		fatal(err.Error())
	}
	worker, err := asset(priv, version, platform, "worker", workerPath, workerURL)
	if err != nil {
		fatal(err.Error())
	}
	m := storeedgemanager.UpdateManifest{Version: version, Platform: platform, PublishedAt: time.Now().UTC().Format(time.RFC3339), ReleaseNotes: notes, Worker: worker}
	if managerPath != "" || managerURL != "" {
		if managerPath == "" || managerURL == "" {
			fatal("manager and manager-url must be provided together")
		}
		m.Manager, err = asset(priv, version, platform, "manager", managerPath, managerURL)
		if err != nil {
			fatal(err.Error())
		}
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		fatal(err.Error())
	}
	b = append(b, '\n')
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil && filepath.Dir(out) != "." {
		fatal(err.Error())
	}
	if err := os.WriteFile(out, b, 0o644); err != nil {
		fatal(err.Error())
	}
	pub := priv.Public().(ed25519.PublicKey)
	fmt.Printf("manifest: %s\npublic key: %s\n", out, base64.StdEncoding.EncodeToString(pub))
}

func privateKey() (ed25519.PrivateKey, error) {
	raw := strings.TrimSpace(os.Getenv("AUTOPARTS_EDGE_UPDATE_PRIVATE_KEY"))
	if raw == "" {
		return nil, fmt.Errorf("AUTOPARTS_EDGE_UPDATE_PRIVATE_KEY is required")
	}
	b, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, err
	}
	switch len(b) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(b), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(b), nil
	default:
		return nil, fmt.Errorf("private key must decode to %d-byte seed or %d-byte private key", ed25519.SeedSize, ed25519.PrivateKeySize)
	}
}
func asset(priv ed25519.PrivateKey, version, platform, component, path, url string) (storeedgemanager.UpdateAsset, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return storeedgemanager.UpdateAsset{}, err
	}
	sum := sha256.Sum256(b)
	sha := hex.EncodeToString(sum[:])
	msg := version + "\n" + platform + "\n" + component + "\n" + sha
	sig := ed25519.Sign(priv, []byte(msg))
	return storeedgemanager.UpdateAsset{URL: url, SHA256: sha, Signature: base64.StdEncoding.EncodeToString(sig)}, nil
}
func fatal(s string) { fmt.Fprintln(os.Stderr, s); os.Exit(1) }
