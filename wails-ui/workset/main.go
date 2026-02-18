package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

const (
	defaultWindowWidth  = 1600
	defaultWindowHeight = 1000
)

func main() {
	appService := NewApp()

	app := application.New(application.Options{
		Name:        "workset",
		Description: "Workset desktop app",
		Icon:        appIcon,
		Services: []application.Service{
			application.NewService(appService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	appService.setRuntime(app)

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             mainWindowName,
		Title:            "workset",
		Width:            defaultWindowWidth,
		Height:           defaultWindowHeight,
		URL:              "/",
		BackgroundColour: application.NewRGB(8, 16, 24),
		Mac: application.MacWindow{
			TitleBar:                application.MacTitleBarHiddenInset,
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
		},
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
