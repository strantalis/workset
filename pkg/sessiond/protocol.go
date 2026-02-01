package sessiond

import (
	"encoding/json"

	"github.com/strantalis/workset/pkg/kitty"
)

type ControlRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type ControlResponse struct {
	OK     bool   `json:"ok"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

type CreateRequest struct {
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd"`
}

type CreateResponse struct {
	SessionID string `json:"sessionId"`
	Existing  bool   `json:"existing"`
}

type SendRequest struct {
	SessionID string `json:"sessionId"`
	Data      string `json:"data"`
}

type ResizeRequest struct {
	SessionID string `json:"sessionId"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
}

type StopRequest struct {
	SessionID string `json:"sessionId"`
}

type BacklogRequest struct {
	SessionID string `json:"sessionId"`
	Since     int64  `json:"since"`
}

type SnapshotRequest struct {
	SessionID string `json:"sessionId"`
}

type BootstrapRequest struct {
	SessionID string `json:"sessionId"`
}

type AckRequest struct {
	SessionID string `json:"sessionId"`
	StreamID  string `json:"streamId"`
	Bytes     int64  `json:"bytes"`
}

type ShutdownRequest struct {
	Source     string `json:"source,omitempty"`
	Reason     string `json:"reason,omitempty"`
	PID        int    `json:"pid,omitempty"`
	Executable string `json:"executable,omitempty"`
}

type BacklogResponse struct {
	SessionID  string `json:"sessionId"`
	Data       string `json:"data"`
	NextOffset int64  `json:"nextOffset"`
	Truncated  bool   `json:"truncated"`
	Source     string `json:"source,omitempty"`
}

type SnapshotResponse struct {
	SessionID     string          `json:"sessionId"`
	Data          string          `json:"data"`
	Source        string          `json:"source,omitempty"`
	Kitty         *kitty.Snapshot `json:"kitty,omitempty"`
	AltScreen     bool            `json:"altScreen,omitempty"`
	MouseMask     uint8           `json:"mouseMask,omitempty"`
	Mouse         bool            `json:"mouse,omitempty"`
	MouseSGR      bool            `json:"mouseSGR,omitempty"`
	MouseEncoding string          `json:"mouseEncoding,omitempty"`
	SafeToReplay  bool            `json:"safeToReplay,omitempty"`
}

type BootstrapResponse struct {
	SessionID        string          `json:"sessionId"`
	Snapshot         string          `json:"snapshot"`
	SnapshotSource   string          `json:"snapshotSource,omitempty"`
	Kitty            *kitty.Snapshot `json:"kitty,omitempty"`
	Backlog          string          `json:"backlog,omitempty"`
	NextOffset       int64           `json:"nextOffset,omitempty"`
	BacklogTruncated bool            `json:"backlogTruncated,omitempty"`
	BacklogSource    string          `json:"backlogSource,omitempty"`
	AltScreen        bool            `json:"altScreen,omitempty"`
	MouseMask        uint8           `json:"mouseMask,omitempty"`
	Mouse            bool            `json:"mouse,omitempty"`
	MouseSGR         bool            `json:"mouseSGR,omitempty"`
	MouseEncoding    string          `json:"mouseEncoding,omitempty"`
	SafeToReplay     bool            `json:"safeToReplay,omitempty"`
	InitialCredit    int64           `json:"initialCredit,omitempty"`
}

type SessionInfo struct {
	SessionID  string `json:"sessionId"`
	Cwd        string `json:"cwd"`
	StartedAt  string `json:"startedAt"`
	LastActive string `json:"lastActive"`
	Running    bool   `json:"running"`
}

type ListResponse struct {
	Sessions []SessionInfo `json:"sessions"`
}

type AttachRequest struct {
	Type       string `json:"type"`
	SessionID  string `json:"sessionId"`
	StreamID   string `json:"streamId,omitempty"`
	Since      int64  `json:"since"`
	WithBuffer bool   `json:"withBuffer"`
}

type InfoResponse struct {
	Executable string `json:"executable"`
	BinaryHash string `json:"binaryHash"`
}

type StreamMessage struct {
	Type       string       `json:"type"`
	SessionID  string       `json:"sessionId,omitempty"`
	StreamID   string       `json:"streamId,omitempty"`
	Data       string       `json:"data,omitempty"`
	Len        int          `json:"len,omitempty"`
	NextOffset int64        `json:"nextOffset,omitempty"`
	Truncated  bool         `json:"truncated,omitempty"`
	Source     string       `json:"source,omitempty"`
	Kitty      *kitty.Event `json:"kitty,omitempty"`
	Error      string       `json:"error,omitempty"`
}
