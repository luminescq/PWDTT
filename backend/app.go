package backend

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx         context.Context
	orch        *Orchestrator
	trayEnabled atomic.Bool
	quitting    atomic.Bool
	trayIcon    []byte
	encKey      [32]byte
}

func NewApp(trayIcon []byte) *App {
	a := &App{trayIcon: trayIcon}
	_, err := rand.Read(a.encKey[:])
	if err != nil {
		panic(fmt.Sprintf("encryption key generation: %v", err))
	}
	return a
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.orch = NewOrchestrator(ctx, a.updateTray)
	startTray(a.trayIcon,
		func() { runtime.WindowShow(ctx) },
		func() {
			if a.orch.IsRunning() {
				a.orch.Stop()
			} else {
				runtime.WindowShow(ctx)
			}
		},
		func() { a.quitting.Store(true); a.orch.Stop(); os.Exit(0) },
	)
}

func (a *App) updateTray(connected bool, rx, tx int64, workers int32) {
	setTrayStatus(connected, rx, tx, workers)
}

func (a *App) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(a.encKey[:])
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := aead.Seal(nil, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(append(nonce, ciphertext...)), nil
}

func (a *App) Decrypt(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(a.encKey[:])
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := aead.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// OnBeforeClose hides the window instead of quitting when tray is enabled.
func (a *App) OnBeforeClose(ctx context.Context) bool {
	if a.trayEnabled.Load() && !a.quitting.Load() {
		runtime.WindowHide(ctx)
		return true // prevent close
	}
	return false
}

func (a *App) Connect(p ConnectParams) error { return a.orch.Start(p) }
func (a *App) Disconnect()                   { a.orch.Stop() }
func (a *App) IsRunning() bool               { return a.orch.IsRunning() }

// CheckVPN returns names of active VPN interfaces (excluding our wg-turn).
func (a *App) CheckVPN() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var found []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		n := strings.ToLower(iface.Name)
		if n == wgIface {
			continue
		}
		if strings.HasPrefix(n, "tun") ||
			strings.HasPrefix(n, "tap") ||
			strings.HasPrefix(n, "wg") ||
			strings.HasPrefix(n, "ppp") ||
			strings.HasPrefix(n, "nordlynx") ||
			strings.HasPrefix(n, "proton") ||
			strings.HasPrefix(n, "utun") ||
			strings.HasPrefix(n, "ipsec") {
			found = append(found, iface.Name)
		}
	}
	return found
}

func (a *App) SaveProfile(name string, p ProfileData) error {
	dir := filepath.Join(configDir(), "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if p.DeviceID == "" {
		if existing, err := loadProfile(name); err == nil && existing.DeviceID != "" {
			p.DeviceID = existing.DeviceID
		} else {
			p.DeviceID = uuid.New().String()
		}
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return os.WriteFile(profilePath(name), data, 0o600)
}

func (a *App) GetProfile(name string) (*ProfileData, error) {
	return loadProfile(name)
}

func (a *App) DeleteProfile(name string) error {
	return os.Remove(profilePath(name))
}

func (a *App) ListProfiles() map[string]ProfileData {
	dir := filepath.Join(configDir(), "profiles")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	result := make(map[string]ProfileData)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		var p ProfileData
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}
		result[name] = p
	}
	return result
}
