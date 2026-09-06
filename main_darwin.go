//go:build darwin

package main

import (
	"embed"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"pwdtt/backend"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Привилегированный режим настройки туннеля (запускается от root через osascript).
	// Перехватываем до инициализации GUI.
	if len(os.Args) > 1 && os.Args[1] == "--wg-helper" {
		backend.RunWGHelperDarwin(os.Args[2:])
		return
	}

	app := backend.NewApp()

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
		OnStartup:        app.Startup,
		OnShutdown:       app.Shutdown,
		Bind:             []interface{}{app},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: false,
				HideTitle:                  false,
				HideTitleBar:               false,
				FullSizeContent:            false,
				UseToolbar:                 false,
				HideToolbarSeparator:       false,
			},
			About: &mac.AboutInfo{
				Title:   "PWDTT",
				Message: "© 2026 PWDTT",
			},
		},
	})
	if err != nil {
		panic(err)
	}
}
