package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) OpenDirectoryDialog(title, defaultDirectory string) (string, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{
		Title:                title,
		DefaultDirectory:     defaultDirectory,
		CanCreateDirectories: true,
	})
}
