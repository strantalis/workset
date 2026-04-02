package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

var transientTerminalErrorMarkers = []string{
	"session not found",
	"terminal not started",
	"terminal not found",
	"terminal stream not ready",
}

func isTransientTerminalCallError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range transientTerminalErrorMarkers {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func (a *App) StartWorkspaceTerminalSessionForWindow(
	_ context.Context,
	workspaceID,
	terminalID string,
) (TerminalSessionDescriptor, error) {
	if err := a.ensureTerminalServiceDescriptorSupport(); err != nil {
		return TerminalSessionDescriptor{}, err
	}
	if err := a.startWorkspaceTerminal(workspaceID, terminalID); err != nil {
		return TerminalSessionDescriptor{}, err
	}
	return a.workspaceTerminalSessionDescriptor(workspaceID, terminalID)
}

func (a *App) workspaceTerminalSessionDescriptor(
	workspaceID,
	terminalID string,
) (TerminalSessionDescriptor, error) {
	if err := a.ensureTerminalServiceDescriptorSupport(); err != nil {
		return TerminalSessionDescriptor{}, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return TerminalSessionDescriptor{}, fmt.Errorf("workspace id required")
	}
	terminalID = strings.TrimSpace(terminalID)
	if terminalID == "" {
		return TerminalSessionDescriptor{}, fmt.Errorf("terminal id required")
	}
	sessionID := terminalSessionID(workspaceID, terminalID)

	infoCtx, infoCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer infoCancel()
	serverInfo, err := a.getTerminalServiceInfo(infoCtx)
	if err != nil {
		return TerminalSessionDescriptor{}, err
	}
	return TerminalSessionDescriptor{
		WorkspaceID: workspaceID,
		TerminalID:  terminalID,
		SessionID:   sessionID,
		SocketURL:   serverInfo.WebSocketURL,
		SocketToken: serverInfo.WebSocketToken,
	}, nil
}

func (a *App) StopWorkspaceTerminalForWindow(_ context.Context, workspaceID, terminalID string) error {
	return a.stopWorkspaceTerminal(workspaceID, terminalID)
}
