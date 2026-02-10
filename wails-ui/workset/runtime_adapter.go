package main

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func emitRuntimeEvent(_ context.Context, name string, data ...any) {
	app := application.Get()
	if app == nil {
		return
	}
	app.Event.Emit(name, data...)
}

func quitRuntime(_ context.Context) {
	app := application.Get()
	if app == nil {
		return
	}
	app.Quit()
}
