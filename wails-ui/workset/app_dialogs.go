package main

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func (a *App) OpenDirectoryDialog(title, defaultDirectory string) (string, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	_ = ctx
	return application.Get().Dialog.OpenFile().
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(true).
		SetTitle(title).
		SetDirectory(defaultDirectory).
		PromptForSingleSelection()
}

func (a *App) OpenFileDialog(title, defaultDirectory string) (string, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	_ = ctx
	return application.Get().Dialog.OpenFile().
		CanChooseDirectories(false).
		CanChooseFiles(true).
		SetTitle(title).
		SetDirectory(defaultDirectory).
		PromptForSingleSelection()
}
