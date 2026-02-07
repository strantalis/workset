package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const terminalLayoutStoreVersion = 1

type TerminalLayoutTab struct {
	ID         string `json:"id"`
	TerminalID string `json:"terminalId"`
	Title      string `json:"title"`
}

type TerminalLayoutNode struct {
	ID          string              `json:"id"`
	Kind        string              `json:"kind"`
	Tabs        []TerminalLayoutTab `json:"tabs,omitempty"`
	ActiveTabID string              `json:"activeTabId,omitempty"`
	Direction   string              `json:"direction,omitempty"`
	Ratio       float64             `json:"ratio,omitempty"`
	First       *TerminalLayoutNode `json:"first,omitempty"`
	Second      *TerminalLayoutNode `json:"second,omitempty"`
}

type TerminalLayout struct {
	Version       int                 `json:"version"`
	Root          *TerminalLayoutNode `json:"root"`
	FocusedPaneID string              `json:"focusedPaneId,omitempty"`
}

type TerminalLayoutPayload struct {
	WorkspaceID   string          `json:"workspaceId"`
	WorkspacePath string          `json:"workspacePath"`
	Layout        *TerminalLayout `json:"layout,omitempty"`
}

type TerminalLayoutRequest struct {
	WorkspaceID string         `json:"workspaceId"`
	Layout      TerminalLayout `json:"layout"`
}

type TerminalLayoutState struct {
	Version       int            `json:"version"`
	WorkspacePath string         `json:"workspacePath"`
	WorkspaceName string         `json:"workspaceName"`
	Layout        TerminalLayout `json:"layout"`
	UpdatedAt     time.Time      `json:"updatedAt"`
}

type terminalLayoutStore struct {
	Version int                            `json:"version"`
	Layouts map[string]TerminalLayoutState `json:"layouts"`
}

func (a *App) GetWorkspaceTerminalLayout(workspaceID string) (TerminalLayoutPayload, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return TerminalLayoutPayload{}, fmt.Errorf("workspace id required")
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	path, err := a.resolveWorkspaceRoot(ctx, workspaceID)
	if err != nil {
		return TerminalLayoutPayload{}, err
	}
	store, err := a.loadTerminalLayoutStore()
	if err != nil {
		return TerminalLayoutPayload{}, err
	}
	entry, ok := store.Layouts[path]
	if !ok || entry.Layout.Root == nil {
		return TerminalLayoutPayload{WorkspaceID: workspaceID, WorkspacePath: path}, nil
	}
	layout := entry.Layout
	return TerminalLayoutPayload{WorkspaceID: workspaceID, WorkspacePath: path, Layout: &layout}, nil
}

func (a *App) SetWorkspaceTerminalLayout(input TerminalLayoutRequest) error {
	workspaceID := strings.TrimSpace(input.WorkspaceID)
	if workspaceID == "" {
		return fmt.Errorf("workspace id required")
	}
	if input.Layout.Root == nil {
		return fmt.Errorf("layout root required")
	}
	layout := input.Layout
	if layout.Version == 0 {
		layout.Version = terminalLayoutStoreVersion
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	path, err := a.resolveWorkspaceRoot(ctx, workspaceID)
	if err != nil {
		return err
	}
	store, err := a.loadTerminalLayoutStore()
	if err != nil {
		return err
	}
	if store.Layouts == nil {
		store.Layouts = map[string]TerminalLayoutState{}
	}
	store.Layouts[path] = TerminalLayoutState{
		Version:       terminalLayoutStoreVersion,
		WorkspacePath: path,
		WorkspaceName: workspaceID,
		Layout:        layout,
		UpdatedAt:     time.Now(),
	}
	return a.persistTerminalLayoutStore(store)
}

func (a *App) terminalLayoutStorePath() (string, error) {
	dir, err := worksetAppDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ui_layouts.json"), nil
}

func (a *App) loadTerminalLayoutStore() (terminalLayoutStore, error) {
	path, err := a.terminalLayoutStorePath()
	if err != nil {
		return terminalLayoutStore{}, err
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return terminalLayoutStore{Version: terminalLayoutStoreVersion, Layouts: map[string]TerminalLayoutState{}}, nil
		}
		return terminalLayoutStore{}, readErr
	}
	var store terminalLayoutStore
	if err := json.Unmarshal(data, &store); err != nil {
		return terminalLayoutStore{}, err
	}
	if store.Version == 0 {
		store.Version = terminalLayoutStoreVersion
	}
	if store.Layouts == nil {
		store.Layouts = map[string]TerminalLayoutState{}
	}
	return store, nil
}

func (a *App) persistTerminalLayoutStore(store terminalLayoutStore) error {
	path, err := a.terminalLayoutStorePath()
	if err != nil {
		return err
	}
	if store.Version == 0 {
		store.Version = terminalLayoutStoreVersion
	}
	if store.Layouts == nil {
		store.Layouts = map[string]TerminalLayoutState{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func collectTerminalIDs(layout *TerminalLayout) []string {
	if layout == nil || layout.Root == nil {
		return nil
	}
	ids := make([]string, 0)
	seen := make(map[string]struct{})
	var visit func(node *TerminalLayoutNode)
	visit = func(node *TerminalLayoutNode) {
		if node == nil {
			return
		}
		if node.Kind == "pane" {
			for _, tab := range node.Tabs {
				if tab.TerminalID == "" {
					continue
				}
				if _, ok := seen[tab.TerminalID]; ok {
					continue
				}
				seen[tab.TerminalID] = struct{}{}
				ids = append(ids, tab.TerminalID)
			}
			return
		}
		if node.Kind == "split" {
			visit(node.First)
			visit(node.Second)
		}
	}
	visit(layout.Root)
	return ids
}

func (a *App) startSessionsFromLayouts(ctx context.Context, store terminalLayoutStore) bool {
	targets := a.terminalLayoutRestoreTargets(ctx, store)
	started := false
	for _, target := range targets {
		_ = a.StartWorkspaceTerminal(target.workspaceID, target.terminalID)
		started = true
	}
	return started
}

func (a *App) terminalLayoutRestoreTargets(ctx context.Context, store terminalLayoutStore) []terminalRestoreTarget {
	if len(store.Layouts) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	a.ensureService()
	list, err := a.service.ListWorkspaces(ctx)
	if err != nil {
		return nil
	}
	nameByPath := make(map[string]string, len(list.Workspaces))
	for _, workspace := range list.Workspaces {
		nameByPath[workspace.Path] = workspace.Name
	}
	targets := make([]terminalRestoreTarget, 0)
	for path, entry := range store.Layouts {
		name := strings.TrimSpace(nameByPath[path])
		if name == "" {
			continue
		}
		for _, terminalID := range collectTerminalIDs(&entry.Layout) {
			targets = append(targets, terminalRestoreTarget{
				workspaceID: name,
				terminalID:  terminalID,
			})
		}
	}
	return targets
}
