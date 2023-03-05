package main

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"k8s.io/client-go/tools/cache"
	"nhooyr.io/websocket"
)

type eventMessage struct {
	MessageType string      `json:"message_type"`
	Message     interface{} `json:"message"`
}

type eventHandler struct{}

type subscriber struct {
	msgs      chan []byte
	closeSlow func()
}

func (s *subscriber) send(msg eventMessage) {
	b, _ := json.Marshal(msg)

	select {
	case s.msgs <- b:
	default:
		go s.closeSlow()
	}
}

func newResourceEventHandler() *resourceEventHandler {
	return &resourceEventHandler{
		subscribers:             make(map[*subscriber]struct{}),
		subscriberMessageBuffer: 16,
	}
}

type resourceEventHandler struct {
	subscribers   map[*subscriber]struct{}
	subscribersMu sync.Mutex

	// subscriberMessageBuffer controls the max number
	// of messages that can be queued for a subscriber
	// before it is kicked.
	//
	// Defaults to 16.
	subscriberMessageBuffer int
}

func (reh *resourceEventHandler) subscribe(ctx context.Context, c *websocket.Conn) error {
	ctx = c.CloseRead(ctx)

	s := &subscriber{
		msgs: make(chan []byte, reh.subscriberMessageBuffer),
		closeSlow: func() {
			c.Close(websocket.StatusPolicyViolation, "connection too slow to keep up with messages")
		},
	}
	reh.addSubscriber(s)
	defer reh.deleteSubscriber(s)

	for {
		select {
		case msg := <-s.msgs:
			err := writeTimeout(ctx, time.Second*5, c, msg)
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func writeTimeout(ctx context.Context, timeout time.Duration, c *websocket.Conn, msg []byte) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return c.Write(ctx, websocket.MessageText, msg)
}

func (reh *resourceEventHandler) addSubscriber(s *subscriber) {
	reh.subscribersMu.Lock()
	reh.subscribers[s] = struct{}{} // TODO: should we check for duplicating subscribers?
	reh.subscribersMu.Unlock()
}

func (reh *resourceEventHandler) deleteSubscriber(s *subscriber) {
	reh.subscribersMu.Lock()
	delete(reh.subscribers, s)
	reh.subscribersMu.Unlock()
}

func (reh *resourceEventHandler) Handle() cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(o interface{}) {
			reh.subscribersMu.Lock()
			for s := range reh.subscribers {
				s.send(eventMessage{MessageType: "create", Message: o})
			}
			reh.subscribersMu.Unlock()
		},
		UpdateFunc: func(old, updated interface{}) {
			reh.subscribersMu.Lock()
			for s := range reh.subscribers {
				s.send(eventMessage{MessageType: "update", Message: updated})
			}
			reh.subscribersMu.Unlock()
		},
		DeleteFunc: func(old interface{}) {
			reh.subscribersMu.Lock()
			for s := range reh.subscribers {
				s.send(eventMessage{MessageType: "delete", Message: old})
			}
			reh.subscribersMu.Unlock()
		},
	}
}
