package sessiond

import "sync"

func enqueueStreamEvent(sub *subscriber, data []byte) (sent bool) {
	defer func() {
		if recover() != nil {
			sent = false
		}
	}()
	select {
	case <-sub.done:
		return false
	case sub.ch <- data:
		return true
	}
}

type subscriber struct {
	ch       chan []byte
	streamID string
	done     chan struct{}
	closed   bool
	closeMu  sync.Mutex
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
	close(s.ch)
}

func newSubscriber(streamID string) *subscriber {
	return &subscriber{
		ch:       make(chan []byte, 128),
		streamID: streamID,
		done:     make(chan struct{}),
	}
}

func (s *Session) subscribe(streamID string) *subscriber {
	if streamID == "" {
		streamID = newStreamID()
	}
	sub := newSubscriber(streamID)
	s.subscribersMu.Lock()
	s.subscribers[sub] = struct{}{}
	s.streams[streamID] = sub
	s.subscribersMu.Unlock()
	return sub
}

func (s *Session) unsubscribe(sub *subscriber) {
	s.subscribersMu.Lock()
	_, ok := s.subscribers[sub]
	clearMouseModes := false
	if ok {
		delete(s.subscribers, sub)
		if sub.streamID != "" {
			delete(s.streams, sub.streamID)
		}
		clearMouseModes = len(s.subscribers) == 0
	}
	s.subscribersMu.Unlock()
	if !ok {
		return
	}
	if clearMouseModes {
		s.outputMu.Lock()
		s.modeState.clearMouseModes()
		s.outputMu.Unlock()
	}
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
	for _, sub := range subs {
		sub.close()
	}
}

func (s *Session) broadcast(data []byte) {
	if len(data) == 0 {
		return
	}
	// Read loop buffers are reused; clone once so queued stream events remain immutable.
	payload := append([]byte(nil), data...)
	s.subscribersMu.Lock()
	subs := make([]*subscriber, 0, len(s.subscribers))
	for sub := range s.subscribers {
		subs = append(subs, sub)
	}
	s.subscribersMu.Unlock()
	for _, sub := range subs {
		_ = enqueueStreamEvent(sub, payload)
	}
}
