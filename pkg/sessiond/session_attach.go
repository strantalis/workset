package sessiond

type attachSnapshot struct {
	ready  AttachReady
	replay []byte
}

func (s *Session) snapshotAttachLocked(req AttachRequest) attachSnapshot {
	snapshot := attachSnapshot{
		ready: AttachReady{
			RequestedOffset: req.Since,
			ReplayStart:     req.Since,
			ReplayNext:      req.Since,
		},
	}

	snapshot.ready.ReplayRequested = req.WithBuffer || req.Since > 0
	if s.buffer != nil {
		oldest, current := s.buffer.SnapshotOffsets()
		snapshot.ready.CurrentOffset = current
		if snapshot.ready.ReplayRequested {
			prefix := s.modeReplayPrefixLocked()
			if s.modeState.altScreen {
				snapshot.ready.ReplaySkipped = true
				snapshot.ready.ReplayStart = current
				snapshot.ready.ReplayNext = current
				snapshot.replay = append(snapshot.replay, prefix...)
			} else {
				replay, next, truncated := s.buffer.ReadSince(req.Since)
				if truncated {
					if transcriptReplay, transcriptStart, transcriptTruncated, err := s.readTranscriptSince(current, req.Since); err == nil {
						if len(transcriptReplay) > len(replay) && transcriptStart < oldest {
							replay = transcriptReplay
							truncated = transcriptTruncated
							snapshot.ready.ReplayStart = transcriptStart
						}
					}
				}
				snapshot.ready.ReplayTruncated = truncated
				if snapshot.ready.ReplayStart == req.Since && truncated {
					snapshot.ready.ReplayStart = oldest
				}
				snapshot.ready.ReplayNext = next
				if len(prefix) > 0 {
					snapshot.replay = append(snapshot.replay, prefix...)
				}
				snapshot.replay = append(snapshot.replay, replay...)
			}
		}
	}

	s.mu.Lock()
	snapshot.ready.Owner = s.inputOwner
	snapshot.ready.Running = s.cmd != nil && !s.closed
	s.mu.Unlock()

	return snapshot
}
