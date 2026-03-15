package sessiond

import "sync"

type streamEvent struct {
	data       []byte
	nextOffset int64
}

func enqueueStreamEvent(sub *subscriber, event streamEvent) (sent bool) {
	defer func() {
		if recover() != nil {
			sent = false
		}
	}()
	select {
	case <-sub.done:
		return false
	case sub.ch <- event:
		return true
	default:
		return false
	}
}

type subscriber struct {
	ch       chan streamEvent
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
		ch:       make(chan streamEvent, 128),
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
	if ok {
		delete(s.subscribers, sub)
		if sub.streamID != "" {
			delete(s.streams, sub.streamID)
		}
	}
	s.subscribersMu.Unlock()
	if !ok {
		return
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

func (s *Session) broadcast(data []byte, nextOffset int64) {
	if len(data) == 0 {
		return
	}
	// Read loop buffers are reused; clone once so queued stream events remain immutable.
	event := streamEvent{
		data:       append([]byte(nil), data...),
		nextOffset: nextOffset,
	}
	s.subscribersMu.Lock()
	subs := make([]*subscriber, 0, len(s.subscribers))
	for sub := range s.subscribers {
		subs = append(subs, sub)
	}
	s.subscribersMu.Unlock()
	var stalled []*subscriber
	for _, sub := range subs {
		if enqueueStreamEvent(sub, event) {
			continue
		}
		stalled = append(stalled, sub)
	}
	for _, sub := range stalled {
		s.unsubscribe(sub)
	}
}
