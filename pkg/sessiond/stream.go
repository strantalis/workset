package sessiond

import (
	"errors"
	"sync"
	"time"

	"github.com/strantalis/workset/pkg/kitty"
)

type streamEvent struct {
	kind  string
	data  []byte
	kitty *kitty.Event
	modes *modeSnapshot
}

type subscriber struct {
	ch        chan streamEvent
	streamID  string
	creditMu  sync.Mutex
	credit    int64
	creditCh  chan struct{}
	done      chan struct{}
	closed    bool
	closeOnce sync.Once
	lastAckAt time.Time
}

func (s *subscriber) close() {
	s.closeOnce.Do(func() {
		s.creditMu.Lock()
		s.closed = true
		s.creditMu.Unlock()
		close(s.done)
		close(s.ch)
	})
}

func newSubscriber(streamID string, initialCredit int64) *subscriber {
	sub := &subscriber{
		ch:       make(chan streamEvent, 64),
		streamID: streamID,
		credit:   initialCredit,
		creditCh: make(chan struct{}, 1),
		done:     make(chan struct{}),
	}
	if initialCredit > 0 {
		sub.lastAckAt = time.Now()
	}
	return sub
}

func (s *subscriber) addCredit(bytes int64) {
	if bytes <= 0 {
		return
	}
	s.creditMu.Lock()
	if s.closed {
		s.creditMu.Unlock()
		return
	}
	s.credit += bytes
	s.lastAckAt = time.Now()
	s.creditMu.Unlock()
	select {
	case s.creditCh <- struct{}{}:
	default:
	}
}

func (s *subscriber) waitForCredit(need int64, timeout time.Duration) bool {
	if need <= 0 {
		return true
	}
	deadline := time.Now().Add(timeout)
	for {
		s.creditMu.Lock()
		if s.closed {
			s.creditMu.Unlock()
			return false
		}
		if s.credit >= need {
			s.credit -= need
			s.creditMu.Unlock()
			return true
		}
		s.creditMu.Unlock()
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return false
		}
		timer := time.NewTimer(remaining)
		select {
		case <-s.creditCh:
			timer.Stop()
		case <-s.done:
			timer.Stop()
			return false
		case <-timer.C:
			return false
		}
	}
}

func (s *Session) hasSubscribers() bool {
	s.subscribersMu.Lock()
	defer s.subscribersMu.Unlock()
	return len(s.subscribers) > 0
}

func (s *Session) subscribe(streamID string) *subscriber {
	if streamID == "" {
		streamID = newStreamID()
	}
	sub := newSubscriber(streamID, 0)
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

func (s *Session) ack(streamID string, bytes int64) error {
	if bytes <= 0 {
		return nil
	}
	s.subscribersMu.Lock()
	sub := s.streams[streamID]
	s.subscribersMu.Unlock()
	if sub == nil {
		return errors.New("stream not found")
	}
	sub.addCredit(bytes)
	return nil
}

func (s *Session) broadcast(data []byte) {
	if len(data) == 0 {
		return
	}
	var overflow []*subscriber
	s.subscribersMu.Lock()
	for sub := range s.subscribers {
		select {
		case sub.ch <- streamEvent{kind: "data", data: data}:
		default:
			overflow = append(overflow, sub)
		}
	}
	s.subscribersMu.Unlock()
	for _, sub := range overflow {
		s.unsubscribe(sub)
		debugLogf("session_stream_drop id=%s stream=%s reason=buffer_overflow", s.id, sub.streamID)
	}
}

func (s *Session) broadcastKitty(events []kitty.Event) {
	if len(events) == 0 {
		return
	}
	var overflow []*subscriber
	s.subscribersMu.Lock()
	for sub := range s.subscribers {
		overflowed := false
		for i := range events {
			ev := events[i]
			select {
			case sub.ch <- streamEvent{kind: "kitty", kitty: &ev}:
			default:
				overflowed = true
			}
			if overflowed {
				break
			}
		}
		if overflowed {
			overflow = append(overflow, sub)
		}
	}
	s.subscribersMu.Unlock()
	for _, sub := range overflow {
		s.unsubscribe(sub)
		debugLogf("session_stream_drop id=%s stream=%s reason=kitty_overflow", s.id, sub.streamID)
	}
}

func (s *Session) broadcastModes(modes modeSnapshot) {
	var overflow []*subscriber
	s.subscribersMu.Lock()
	for sub := range s.subscribers {
		select {
		case sub.ch <- streamEvent{kind: "modes", modes: &modes}:
		default:
			overflow = append(overflow, sub)
		}
	}
	s.subscribersMu.Unlock()
	for _, sub := range overflow {
		s.unsubscribe(sub)
		debugLogf("session_stream_drop id=%s stream=%s reason=modes_overflow", s.id, sub.streamID)
	}
}
