package sessiond

import (
	"sort"
	"sync"
)

type subscriber struct {
	notify   chan struct{}
	streamID string
	done     chan struct{}
	closed   bool
	closeMu  sync.Mutex

	// offset tracks the last buffer position this subscriber consumed.
	// Protected by the subscriber's own mutex so the WebSocket writer
	// goroutine can update it without holding the session lock.
	offsetMu sync.Mutex
	offset   int64
}

type streamState struct {
	Count     int
	StreamIDs []string
}

func (s *subscriber) close() {
	s.closeMu.Lock()
	if s.closed {
		s.closeMu.Unlock()
		return
	}
	s.closed = true
	s.closeMu.Unlock()
	close(s.done)
	close(s.notify)
}

func (s *subscriber) getOffset() int64 {
	s.offsetMu.Lock()
	v := s.offset
	s.offsetMu.Unlock()
	return v
}

func (s *subscriber) setOffset(v int64) {
	s.offsetMu.Lock()
	s.offset = v
	s.offsetMu.Unlock()
}

func newSubscriber(streamID string, startOffset int64) *subscriber {
	return &subscriber{
		notify:   make(chan struct{}, 1),
		streamID: streamID,
		done:     make(chan struct{}),
		offset:   startOffset,
	}
}

func (s *Session) subscribe(streamID string, startOffset int64) *subscriber {
	if streamID == "" {
		streamID = newStreamID()
	}
	sub := newSubscriber(streamID, startOffset)
	s.subscribersMu.Lock()
	s.subscribers[sub] = struct{}{}
	s.streams[streamID] = sub
	state := streamStateFromMaps(s.subscribers, s.streams)
	s.subscribersMu.Unlock()
	logServerf("ws_subscribe session=%s stream=%s start_offset=%d subscribers=%d streams=%q", s.id, streamID, startOffset, state.Count, state.StreamIDs)
	return sub
}

func (s *Session) unsubscribe(sub *subscriber) {
	s.unsubscribeWithReason(sub, "detach")
}

func (s *Session) unsubscribeWithReason(sub *subscriber, reason string) {
	s.subscribersMu.Lock()
	_, ok := s.subscribers[sub]
	state := streamState{}
	if ok {
		delete(s.subscribers, sub)
		if sub.streamID != "" {
			delete(s.streams, sub.streamID)
		}
		state = streamStateFromMaps(s.subscribers, s.streams)
	}
	s.subscribersMu.Unlock()
	if !ok {
		return
	}
	logServerf(
		"ws_unsubscribe session=%s stream=%s reason=%s offset=%d subscribers=%d streams=%q",
		s.id,
		sub.streamID,
		reason,
		sub.getOffset(),
		state.Count,
		state.StreamIDs,
	)
	sub.close()
}

func (s *Session) closeSubscribers() {
	s.subscribersMu.Lock()
	subs := make([]*subscriber, 0, len(s.subscribers))
	for sub := range s.subscribers {
		subs = append(subs, sub)
	}
	s.subscribers = make(map[*subscriber]struct{})
	s.streams = make(map[string]*subscriber)
	s.subscribersMu.Unlock()
	logServerf("ws_close_subscribers session=%s count=%d", s.id, len(subs))
	for _, sub := range subs {
		logServerf("ws_unsubscribe session=%s stream=%s reason=session_close offset=%d subscribers=0 streams=[]", s.id, sub.streamID, sub.getOffset())
		sub.close()
	}
}

// notifySubscribers signals all subscribers that new data is available in the
// ring buffer.  The notification is non-blocking: if a subscriber already has
// a pending signal it will see the new data when it next drains.
func (s *Session) notifySubscribers() {
	s.subscribersMu.Lock()
	subs := make([]*subscriber, 0, len(s.subscribers))
	for sub := range s.subscribers {
		subs = append(subs, sub)
	}
	s.subscribersMu.Unlock()
	for _, sub := range subs {
		select {
		case sub.notify <- struct{}{}:
		default:
			// Already has a pending notification — subscriber will catch up.
		}
	}
}

// pullBuffer reads all data the subscriber hasn't consumed yet from the
// session's ring buffer.  Returns nil when there is nothing new.
func (s *Session) pullBuffer(sub *subscriber) ([]byte, int64, bool) {
	offset := sub.getOffset()
	data, nextOffset, truncated := s.buffer.ReadSince(offset)
	if len(data) > 0 {
		sub.setOffset(nextOffset)
	}
	return data, nextOffset, truncated
}

func (s *Session) getStreamState() streamState {
	s.subscribersMu.Lock()
	state := streamStateFromMaps(s.subscribers, s.streams)
	s.subscribersMu.Unlock()
	return state
}

func streamStateFromMaps(subscribers map[*subscriber]struct{}, streams map[string]*subscriber) streamState {
	ids := make([]string, 0, len(streams))
	for streamID := range streams {
		ids = append(ids, streamID)
	}
	sort.Strings(ids)
	return streamState{
		Count:     len(subscribers),
		StreamIDs: ids,
	}
}
