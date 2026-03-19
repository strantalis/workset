package sessiond

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"
)

type sessionSnapshotState struct {
	payload    json.RawMessage
	nextOffset int64
	cols       int
	rows       int
	updatedAt  time.Time
}

type terminalSnapshotEnvelope struct {
	Version    int   `json:"version"`
	NextOffset int64 `json:"nextOffset"`
	Cols       int   `json:"cols"`
	Rows       int   `json:"rows"`
}

func cloneSnapshotPayload(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	cloned := make([]byte, len(value))
	copy(cloned, value)
	return cloned
}

func (s *Session) storeSnapshotForOwner(snapshot json.RawMessage, owner string) error {
	snapshot = bytes.TrimSpace(snapshot)
	if len(snapshot) == 0 {
		return nil
	}

	var envelope terminalSnapshotEnvelope
	if err := json.Unmarshal(snapshot, &envelope); err != nil {
		return errors.New("invalid terminal snapshot: " + err.Error())
	}
	if envelope.Version <= 0 {
		return errors.New("terminal snapshot version required")
	}
	if envelope.NextOffset < 0 {
		return errors.New("terminal snapshot next offset must be non-negative")
	}
	if envelope.Cols < 1 || envelope.Rows < 1 {
		return errors.New("terminal snapshot dimensions required")
	}

	s.outputMu.Lock()
	defer s.outputMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.enforceOwnerLocked(owner); err != nil {
		return err
	}
	if envelope.NextOffset < s.snapshot.nextOffset {
		return nil
	}

	s.snapshot.payload = cloneSnapshotPayload(snapshot)
	s.snapshot.nextOffset = envelope.NextOffset
	s.snapshot.cols = envelope.Cols
	s.snapshot.rows = envelope.Rows
	s.snapshot.updatedAt = time.Now()
	return nil
}

func snapshotMatchesAttachDimensions(req AttachRequest, state sessionSnapshotState) bool {
	if len(state.payload) == 0 {
		return false
	}
	if req.Cols > 0 && state.cols > 0 && req.Cols != state.cols {
		return false
	}
	if req.Rows > 0 && state.rows > 0 && req.Rows != state.rows {
		return false
	}
	return true
}
