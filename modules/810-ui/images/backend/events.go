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

/*
Там примерно такая логика:
Клиент подключается.
Клиент ожидает пинги { type: "ping" } . Если их не будет, он будет считать коннекшн stale и переконнекчиваться.
Клиент делает запрос { command: "subscribe", identifier: "{\"channel\": \"MyChannel\"}"}
Клиент ожидает ответ { type: "confirm_subscription", identifier: "{\"channel\": \"MyChannel\"}"}
Клиент ожидает сообщения в канал { identifier: "{\"channel\": \"MyChannel\"}", message: "SOME JSON"}
Клиент может слать в канал  { identifier: "{\"channel\": \"MyChannel\"}", command: "message", data: "SOME JSON"}
*/
func (reh *resourceEventHandler) subscribe(ctx context.Context, conn *websocket.Conn) error {
	ctx = conn.CloseRead(ctx)

	s := &subscriber{
		msgs: make(chan []byte, reh.subscriberMessageBuffer),
		closeSlow: func() {
			conn.Close(websocket.StatusPolicyViolation, "connection too slow to keep up with messages")
		},
	}
	reh.addSubscriber(s)
	defer reh.deleteSubscriber(s)

	// Sending pings to keep the connection alive.
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	writeTimeout := 5 * time.Second
	for {
		select {
		case <-ticker.C:
			err := writeWithTimeout(ctx, writeTimeout, conn, []byte(`{"type":"ping"}`))
			if err != nil {
				return err
			}
		case msg := <-s.msgs:
			err := writeWithTimeout(ctx, writeTimeout, conn, msg)
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func writeWithTimeout(ctx context.Context, timeout time.Duration, c *websocket.Conn, msg []byte) error {
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
