package terminalservice

type attachSnapshot struct {
	replay          []byte
	replayStart     int64
	replayNext      int64
	replayTruncated bool
	replaySkipped   bool
}

func (s *Session) snapshotAttachLocked() attachSnapshot {
	snapshot := attachSnapshot{}

	if s.buffer != nil {
		oldest, current := s.buffer.SnapshotOffsets()
		replaySince := int64(0)
		prefix := s.modeReplayPrefixLocked()
		if s.modeState.altScreen {
			snapshot.replaySkipped = true
			snapshot.replayStart = current
			snapshot.replayNext = current
			snapshot.replay = append(snapshot.replay, prefix...)
		} else {
			replay, next, truncated := s.buffer.ReadSince(replaySince)
			if truncated {
				if transcriptReplay, transcriptStart, transcriptTruncated, err := s.readTranscriptSince(current, replaySince); err == nil {
					if len(transcriptReplay) > len(replay) && transcriptStart < oldest {
						replay = transcriptReplay
						truncated = transcriptTruncated
						snapshot.replayStart = transcriptStart
					}
				}
			}
			snapshot.replayTruncated = truncated
			if snapshot.replayStart == replaySince && truncated {
				snapshot.replayStart = oldest
			}
			snapshot.replayNext = next
			if len(prefix) > 0 {
				snapshot.replay = append(snapshot.replay, prefix...)
			}
			snapshot.replay = append(snapshot.replay, replay...)
		}
		snapshot.replay = stripReplayQueries(snapshot.replay)
	}
	return snapshot
}
