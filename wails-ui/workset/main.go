package main

import (
	"embed"
	"log"
	"time"

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

	mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             mainWindowName,
		Title:            "workset",
		Width:            defaultWindowWidth,
		Height:           defaultWindowHeight,
		URL:              "/",
		BackgroundColour: application.NewRGB(8, 16, 24),
		KeyBindings: map[string]func(window application.Window){
			"CmdOrCtrl+C": func(window application.Window) {
				logTerminalDebug(TerminalDebugPayload{
					WorkspaceID: "__app__",
					TerminalID:  "__window__",
					Event:       "native_copy_keybinding",
					Details:     `{"accelerator":"CmdOrCtrl+C"}`,
				})
				window.EmitEvent("workset:native-copy-command", map[string]any{
					"accelerator": "CmdOrCtrl+C",
					"emittedAt":   time.Now().Format(time.RFC3339Nano),
				})
			},
		},
		Mac: application.MacWindow{
			TitleBar:                application.MacTitleBarHidden,
			InvisibleTitleBarHeight: 34,
			Backdrop:                application.MacBackdropNormal,
		},
	})
	attachLifecycleDebugLogging(app, mainWindow)
	preventMainWindowHideOnFocusLost(mainWindow)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
