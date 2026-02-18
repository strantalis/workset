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
}

type StopRequest struct {
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

type ListResponse struct {
	Sessions []SessionInfo `json:"sessions"`
}

type AttachRequest struct {
	ProtocolVersion int    `json:"protocolVersion"`
	Type            string `json:"type"`
	SessionID       string `json:"sessionId"`
	StreamID        string `json:"streamId,omitempty"`
}

type InfoResponse struct {
	Executable string `json:"executable"`
	BinaryHash string `json:"binaryHash"`
}

type StreamMessage struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionId,omitempty"`
	StreamID  string `json:"streamId,omitempty"`
	DataB64   string `json:"dataB64,omitempty"`
	Len       int    `json:"len,omitempty"`
	Error     string `json:"error,omitempty"`
}
