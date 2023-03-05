package main

import (
	"sync"

	"k8s.io/client-go/tools/cache"
	"nhooyr.io/websocket"
)

type eventMessage struct {
	MessageType string      `json:"message_type"`
	Message     interface{} `json:"message"`
}

type eventHandler struct{}

type subscriber struct {
	conn *websocket.Conn
}

func (s *subscriber) send(msg eventMessage) {
}

type resourceEventHandler struct {
	subscribers   map[*subscriber]struct{}
	subscribersMu sync.Mutex
}

func (reh *resourceEventHandler) subscribe(s *subscriber) {
	reh.subscribersMu.Lock()
	reh.subscribers[s] = struct{}{} // TODO: should we check for duplicating subscribers?
	reh.subscribersMu.Unlock()
}

func (reh *resourceEventHandler) unsubscribe(s *subscriber) {
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
