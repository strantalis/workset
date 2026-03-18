package sessiond

import "encoding/json"

type attachSnapshot struct {
	ready    AttachReady
	snapshot json.RawMessage
	replay   []byte
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
		replaySince := req.Since
		if snapshotMatchesAttachDimensions(req, s.snapshot) && s.snapshot.nextOffset >= req.Since {
			snapshot.snapshot = cloneSnapshotPayload(s.snapshot.payload)
			replaySince = s.snapshot.nextOffset
			snapshot.ready.ReplayRequested = true
			snapshot.ready.ReplayStart = replaySince
			snapshot.ready.ReplayNext = replaySince
		}
		if snapshot.ready.ReplayRequested {
			prefix := s.modeReplayPrefixLocked()
			if len(snapshot.snapshot) == 0 && s.modeState.altScreen {
				snapshot.ready.ReplaySkipped = true
				snapshot.ready.ReplayStart = current
				snapshot.ready.ReplayNext = current
				snapshot.replay = append(snapshot.replay, prefix...)
			} else {
				replay, next, truncated := s.buffer.ReadSince(replaySince)
				if truncated {
					if transcriptReplay, transcriptStart, transcriptTruncated, err := s.readTranscriptSince(current, replaySince); err == nil {
						if len(transcriptReplay) > len(replay) && transcriptStart < oldest {
							replay = transcriptReplay
							truncated = transcriptTruncated
							snapshot.ready.ReplayStart = transcriptStart
						}
					}
				}
				snapshot.ready.ReplayTruncated = truncated
				if snapshot.ready.ReplayStart == replaySince && truncated {
					snapshot.ready.ReplayStart = oldest
				}
				snapshot.ready.ReplayNext = next
				if len(prefix) > 0 && len(snapshot.snapshot) == 0 {
					snapshot.replay = append(snapshot.replay, prefix...)
				}
				snapshot.replay = append(snapshot.replay, replay...)
			}
		}
		snapshot.replay = stripReplayQueries(snapshot.replay)
	}

	s.mu.Lock()
	snapshot.ready.Owner = s.inputOwner
	snapshot.ready.Running = s.cmd != nil && !s.closed
	s.mu.Unlock()

	return snapshot
}
