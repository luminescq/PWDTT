//go:build windows

package main

import (
	"context"
	"embed"
	"sync"
	"sync/atomic"
	"time"

	"github.com/energye/systray"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"pwdtt/backend"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed assets/icons/icon.png
var appIcon []byte

//go:embed assets/icons/icon.ico
var trayIcon []byte

//go:embed assets/wintun.dll
var wintunDLL []byte

var (
	trayReady    atomic.Bool
	trayStopping atomic.Bool
	trayStop     = make(chan struct{})
	trayExited   = make(chan struct{})
	trayStopOnce sync.Once
	trayExitOnce sync.Once
)

func startTray(ctx context.Context) {
	go systray.Run(func() {
		if trayStopping.Load() {
			systray.Quit()
			return
		}

		systray.SetIcon(trayIcon)
		systray.SetTooltip("PWDTT")
		systray.SetOnClick(func(systray.IMenu) { wailsruntime.WindowShow(ctx) })
		systray.SetOnDClick(func(systray.IMenu) { wailsruntime.WindowShow(ctx) })

		show := systray.AddMenuItem("Show PWDTT", "Restore PWDTT")
		show.Click(func() { wailsruntime.WindowShow(ctx) })
		systray.AddSeparator()
		exit := systray.AddMenuItem("Exit", "Exit PWDTT")
		exit.Click(func() { wailsruntime.Quit(ctx) })

		trayReady.Store(true)
		go hideMinimisedWindow(ctx)
	}, func() {
		trayReady.Store(false)
		trayExitOnce.Do(func() { close(trayExited) })
		if !trayStopping.Load() {
			wailsruntime.WindowShow(ctx)
		}
	})
}

func hideMinimisedWindow(ctx context.Context) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-trayStop:
			return
		case <-ticker.C:
			if trayReady.Load() && wailsruntime.WindowIsMinimised(ctx) {
				wailsruntime.WindowHide(ctx)
			}
		}
	}
}

func stopTray() {
	trayStopping.Store(true)
	trayStopOnce.Do(func() { close(trayStop) })
	if trayReady.Load() {
		systray.Quit()
		select {
		case <-trayExited:
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func main() {
	backend.InitWintun(wintunDLL)
	app := backend.NewApp()
	secondInstanceLaunch := make(chan struct{}, 1)

	err := wails.Run(&options.App{
		Title:     "PWDTT",
		Width:     900,
		Height:    600,
		MinWidth:  800,
		MinHeight: 550,
		Frameless: false,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1},
		OnStartup: func(ctx context.Context) {
			app.Startup(ctx)
			startTray(ctx)
			go func() {
				for range secondInstanceLaunch {
					wailsruntime.WindowShow(ctx)
				}
			}()
		},
		OnShutdown: func(ctx context.Context) {
			stopTray()
			app.Shutdown(ctx)
		},
		Bind: []interface{}{app},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "pwdtt-windows-client",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				select {
				case secondInstanceLaunch <- struct{}{}:
				default:
				}
			},
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
	})
	if err != nil {
		panic(err)
	}
}
