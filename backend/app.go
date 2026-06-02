package backend

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx         context.Context
	orch        *Orchestrator
	trayEnabled atomic.Bool
	trayIcon    []byte
}

func NewApp(trayIcon []byte) *App { return &App{trayIcon: trayIcon} }

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.orch = NewOrchestrator(ctx)
	startTray(a.trayIcon,
		func() { runtime.WindowShow(ctx) },
		func() { a.trayEnabled.Store(false); runtime.Quit(ctx) },
	)
}

// OnBeforeClose hides the window instead of quitting when tray is enabled.
func (a *App) OnBeforeClose(ctx context.Context) bool {
	if a.trayEnabled.Load() {
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
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return os.WriteFile(profilePath(name), data, 0600)
}

func (a *App) GetProfile(name string) (*ProfileData, error) {
	return loadProfile(name)
}

func (a *App) DeleteProfile(name string) error {
	return os.Remove(profilePath(name))
}
