package sessiond

import "github.com/strantalis/workset/pkg/kitty"

func (s *Session) snapshot() SnapshotResponse {
	s.mu.Lock()
	emu := s.emu
	kittyState := s.kittyState
	id := s.id
	altScreen := s.altScreen
	mouseMask := s.mouseMask
	mouseEnabled := mouseMask != 0
	mouseSGR := s.mouseSGR
	mouseEncoding := s.mouseEncoding()
	tuiMode := s.tuiMode
	s.mu.Unlock()
	data := ""
	if emu != nil {
		if !altScreen && !tuiMode && emu.HistoryLen() > 0 {
			data = emu.SnapshotANSIWithHistory()
		} else {
			data = emu.SnapshotANSI()
		}
	}
	if emu != nil && emu.IsAltScreen() {
		altScreen = true
	}
	safeToReplay := !altScreen && !tuiMode
	var kittySnapshot *kitty.Snapshot
	if kittyState != nil {
		snap := kittyState.Snapshot()
		if len(snap.Images) > 0 || len(snap.Placements) > 0 {
			kittySnapshot = &snap
		}
	}
	return SnapshotResponse{
		SessionID:     id,
		Data:          data,
		Source:        "snapshot",
		Kitty:         kittySnapshot,
		AltScreen:     altScreen,
		MouseMask:     mouseMask,
		Mouse:         mouseEnabled,
		MouseSGR:      mouseSGR,
		MouseEncoding: mouseEncoding,
		SafeToReplay:  safeToReplay,
	}
}

func (s *Session) backlog(since int64) (BacklogResponse, error) {
	s.mu.Lock()
	tui := s.tuiMode
	emu := s.emu
	s.mu.Unlock()
	if tui {
		if emu != nil {
			snapshot := emu.SnapshotANSI()
			if snapshot != "" {
				return BacklogResponse{
					SessionID: s.id,
					Data:      snapshot,
					Truncated: false,
					Source:    "snapshot",
				}, nil
			}
		}
		return BacklogResponse{
			SessionID: s.id,
			Data:      "",
			Truncated: true,
			Source:    "tui",
		}, nil
	}
	if emu != nil && emu.IsAltScreen() {
		return BacklogResponse{
			SessionID: s.id,
			Data:      emu.SnapshotANSI(),
			Truncated: false,
			Source:    "snapshot",
		}, nil
	}
	if since == 0 && emu != nil && emu.HistoryLen() > 0 {
		return BacklogResponse{
			SessionID: s.id,
			Data:      emu.SnapshotANSIWithHistory(),
			Truncated: false,
			Source:    "history",
		}, nil
	}
	if since < 0 {
		since = 0
	}
	if s.buffer != nil {
		data, next, truncated := s.buffer.ReadSince(since)
		if len(data) > 0 || next > 0 {
			return BacklogResponse{
				SessionID:  s.id,
				Data:       string(data),
				NextOffset: next,
				Truncated:  truncated,
				Source:     "buffer",
			}, nil
		}
	}
	data, truncated, err := s.readTranscriptTail(s.opts.TranscriptTailBytes)
	if err != nil {
		return BacklogResponse{}, err
	}
	return BacklogResponse{
		SessionID:  s.id,
		Data:       string(data),
		NextOffset: 0,
		Truncated:  truncated,
		Source:     "transcript",
	}, nil
}
