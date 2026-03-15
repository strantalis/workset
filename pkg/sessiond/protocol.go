package sessiond

import "encoding/json"

const ProtocolVersion = 2

type ControlRequest struct {
	ProtocolVersion int             `json:"protocolVersion"`
	Method          string          `json:"method"`
	Params          json.RawMessage `json:"params,omitempty"`
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
	Owner     string `json:"owner,omitempty"`
}

type ResizeRequest struct {
	SessionID string `json:"sessionId"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
	Owner     string `json:"owner,omitempty"`
}

type StopRequest struct {
	SessionID string `json:"sessionId"`
	Owner     string `json:"owner,omitempty"`
}

type InspectRequest struct {
	SessionID string `json:"sessionId"`
}

type ShutdownRequest struct {
	Source     string `json:"source,omitempty"`
	Reason     string `json:"reason,omitempty"`
	PID        int    `json:"pid,omitempty"`
	Executable string `json:"executable,omitempty"`
}

type OwnerRequest struct {
	SessionID string `json:"sessionId"`
	Owner     string `json:"owner"`
}

type OwnerResponse struct {
	SessionID string `json:"sessionId"`
	Owner     string `json:"owner"`
}

type SessionInfo struct {
	SessionID  string `json:"sessionId"`
	Cwd        string `json:"cwd"`
	StartedAt  string `json:"startedAt"`
	LastActive string `json:"lastActive"`
	Running    bool   `json:"running"`
}

type InspectResponse struct {
	SessionID     string `json:"sessionId"`
	Cwd           string `json:"cwd"`
	StartedAt     string `json:"startedAt"`
	LastActive    string `json:"lastActive"`
	Running       bool   `json:"running"`
	Owner         string `json:"owner,omitempty"`
	CurrentOffset int64  `json:"currentOffset"`
}

type ListResponse struct {
	Sessions []SessionInfo `json:"sessions"`
}

type AttachRequest struct {
	ProtocolVersion int    `json:"protocolVersion"`
	Type            string `json:"type"`
	SessionID       string `json:"sessionId"`
	StreamID        string `json:"streamId,omitempty"`
	ClientID        string `json:"clientId,omitempty"`
	Token           string `json:"token,omitempty"`
	Since           int64  `json:"since,omitempty"`
	WithBuffer      bool   `json:"withBuffer,omitempty"`
	Cols            int    `json:"cols,omitempty"`
	Rows            int    `json:"rows,omitempty"`
}

type WebsocketControlRequest struct {
	ProtocolVersion int             `json:"protocolVersion,omitempty"`
	Type            string          `json:"type"`
	Data            string          `json:"data,omitempty"`
	Cols            int             `json:"cols,omitempty"`
	Rows            int             `json:"rows,omitempty"`
	Owner           string          `json:"owner,omitempty"`
	RequestID       string          `json:"requestId,omitempty"`
	Snapshot        json.RawMessage `json:"snapshot,omitempty"`
}

type AttachReady struct {
	RequestedOffset int64  `json:"requestedOffset"`
	ReplayStart     int64  `json:"replayStart"`
	ReplayNext      int64  `json:"replayNext"`
	CurrentOffset   int64  `json:"currentOffset"`
	ReplayRequested bool   `json:"replayRequested"`
	ReplayTruncated bool   `json:"replayTruncated"`
	ReplaySkipped   bool   `json:"replaySkipped"`
	Owner           string `json:"owner,omitempty"`
	Running         bool   `json:"running"`
}

type InfoResponse struct {
	Executable     string `json:"executable"`
	BinaryHash     string `json:"binaryHash"`
	WebSocketURL   string `json:"webSocketUrl,omitempty"`
	WebSocketToken string `json:"webSocketToken,omitempty"`
}

type StreamMessage struct {
	Type       string          `json:"type"`
	SessionID  string          `json:"sessionId,omitempty"`
	StreamID   string          `json:"streamId,omitempty"`
	RequestID  string          `json:"requestId,omitempty"`
	DataB64    string          `json:"dataB64,omitempty"`
	Len        int             `json:"len,omitempty"`
	NextOffset int64           `json:"nextOffset,omitempty"`
	Ready      *AttachReady    `json:"ready,omitempty"`
	Snapshot   json.RawMessage `json:"snapshot,omitempty"`
	Error      string          `json:"error,omitempty"`
}
