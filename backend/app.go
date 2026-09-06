package backend

import (
	"context"

	wails "github.com/wailsapp/wails/v2/pkg/runtime"
)

const Version = "1.6.0"

// App — главный объект приложения.
// Wails привязывает его методы к frontend через Bind().
type App struct {
	ctx    context.Context
	bridge *Bridge
	store  *Store
}

// NewApp создаёт App. Вызывается из main() до wails.Run().
func NewApp() *App {
	return &App{
		store: NewStore(),
	}
}

// ═══════════════════════════════════════════════════
// WAILS LIFECYCLE
// ═══════════════════════════════════════════════════

// Startup вызывается Wails после создания webview.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	settings := a.store.LoadSettings()
	a.bridge = NewBridge(ctx, a.store, a.onBridgeEvent)

	if settings.AutoStart {
		a.SetAutoStart(true)
	}

	// Уборка маршрутов, оставшихся от краша прошлого запуска
	go CleanupStaleExcludeRoutes(func(msg string) {
		a.onBridgeEvent("log", "INFO", "[WG] "+msg)
	})
}

// Shutdown вызывается Wails при закрытии приложения.
func (a *App) Shutdown(ctx context.Context) {
	if a.bridge != nil {
		a.bridge.Disconnect()
	}
}

// ═══════════════════════════════════════════════════
// WAILS BINDINGS
// ═══════════════════════════════════════════════════

func (a *App) Connect(params ConnectParams) error {
	return a.bridge.Connect(params)
}

func (a *App) Disconnect() {
	a.bridge.Disconnect()
}

func (a *App) IsRunning() bool {
	return a.bridge.IsRunning()
}

func (a *App) GetVersion() string {
	return Version
}

// ═══════════════════════════════════════════════════
// PROFILES
// ═══════════════════════════════════════════════════

func (a *App) GetProfile(name string) (*ProfileData, error) {
	return a.store.LoadProfile(name)
}

func (a *App) SaveProfile(name string, p ProfileData) error {
	return a.store.SaveProfile(name, p)
}

func (a *App) DeleteProfile(name string) error {
	return a.store.DeleteProfile(name)
}

func (a *App) ListProfiles() map[string]ProfileData {
	return a.store.ListProfiles()
}

// ═══════════════════════════════════════════════════
// SETTINGS
// ═══════════════════════════════════════════════════

func (a *App) GetAutoStart() bool {
	return a.store.LoadSettings().AutoStart
}

func (a *App) SetAutoStart(v bool) error {
	settings := a.store.LoadSettings()
	settings.AutoStart = v
	a.store.SaveSettings(settings)
	return setAutoStart(v)
}

func (a *App) GetObfsMode() string {
	return a.store.LoadSettings().ObfsMode
}

func (a *App) SetObfsMode(mode string) error {
	settings := a.store.LoadSettings()
	settings.ObfsMode = mode
	return a.store.SaveSettings(settings)
}

func (a *App) GetObfsAccepted() bool {
	return a.store.LoadSettings().ObfsAccepted
}

func (a *App) SetObfsAccepted(v bool) error {
	settings := a.store.LoadSettings()
	settings.ObfsAccepted = v
	return a.store.SaveSettings(settings)
}

func (a *App) CheckUpdate() *UpdateInfo {
	info, err := CheckUpdate(Version)
	if err != nil {
		return &UpdateInfo{Available: false}
	}
	return info
}

// ═══════════════════════════════════════════════════
// INTERNAL
// ═══════════════════════════════════════════════════

func (a *App) onBridgeEvent(name string, args ...any) {
	wails.EventsEmit(a.ctx, name, args...)
}
