package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

type eventMessage struct {
	MessageType string      `json:"message_type"`
	Message     interface{} `json:"message"`
}

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
		subscribers:             make(map[*subscriber]map[string]struct{}),
		subscriberMessageBuffer: 16,
	}
}

type resourceEventHandler struct {
	subscribers   map[*subscriber]map[string]struct{}
	subscribersMu sync.Mutex

	// subscriberMessageBuffer controls the max number
	// of messages that can be queued for a subscriber
	// before it is kicked.
	//
	// Defaults to 16.
	subscriberMessageBuffer int
}

// type cableMessage struct {
// 	Type       string `json:"type"`
// 	Message    string `json:"message"`
// 	Identifier string `json:"identifier"`
// }

// Message represents incoming client message
// https://github.com/anycable/anycable-go/blob/master/common/common.go#LL185-L190C2
type cableMessage struct {
	Command    string      `json:"command"`
	Identifier string      `json:"identifier"`
	Data       interface{} `json:"data,omitempty"`
}

/*
Там примерно такая логика:
Клиент подключается.
Клиент ожидает пинги. Если их не будет, он будет считать коннекшн stale и переконнекчиваться.

	{ type: "ping" }

Клиент делает запрос

	{ command: "subscribe",         identifier: "{\"channel\": \"MyChannel\"}"}

Клиент ожидает ответ

	{ type: "confirm_subscription", identifier: "{\"channel\": \"MyChannel\"}"}

Клиент ожидает сообщения в канал

	{ identifier: "{\"channel\": \"MyChannel\"}", message: "SOME JSON"}

Клиент может слать в канал  (зочем?)

	{ identifier: "{\"channel\": \"MyChannel\"}", command: "message", data: "SOME JSON"}
*/
func (reh *resourceEventHandler) subscribe(ctx context.Context, conn *websocket.Conn) error {
	// ctx = conn.CloseRead(ctx)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

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

	in := make(chan cableMessage)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// msgType, msg, err := conn.Read(ctx)
				// if err != nil {
				// 	defer cancel()
				// 	klog.V(5).ErrorS(err, "reading from websocket")
				// 	conn.Close(websocket.StatusNormalClosure, "")
				// 	return
				// }
				// if msgType != websocket.MessageText {
				// 	klog.V(5).ErrorS(err, "got binary data from websocket")
				// 	continue
				// }
				// klog.V(5).Info("message", msg)

				var msg cableMessage
				if err := wsjson.Read(ctx, conn, &msg); err != nil {
					klog.V(5).ErrorS(err, "reading JSON from websocket")
					continue
				}
				in <- msg
			}
		}
	}()

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
		case command := <-in:
			fmt.Println("ws received", command)
			switch command.Command {
			case "subscribe":
				var cid struct{ Channel string }
				_ = json.Unmarshal([]byte(command.Identifier), &cid)
				gvr := schema.GroupVersionResource{Resource: strings.ToLower(strings.TrimSuffix(cid.Channel, "Channel"))}
				reh.addResourceSubscription(s, gvr)
				err := writeWithTimeout(ctx, writeTimeout, conn, []byte(`{"type": "confirm_subscription", "identifier": "`+command.Identifier+`"}`))
				if err != nil {
					return err
				}
			case "unsubscribe":
				var cid struct{ Channel string }
				_ = json.Unmarshal([]byte(command.Identifier), &cid)
				gvr := schema.GroupVersionResource{Resource: strings.ToLower(strings.TrimSuffix(cid.Channel, "Channel"))}
				reh.deleteResourceSubscription(s, gvr)
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
	reh.subscribers[s] = make(map[string]struct{}) // TODO: should we check for duplicating subscribers?
	reh.subscribersMu.Unlock()
}

func (reh *resourceEventHandler) deleteSubscriber(s *subscriber) {
	reh.subscribersMu.Lock()
	delete(reh.subscribers, s)
	reh.subscribersMu.Unlock()
}

func (reh *resourceEventHandler) addResourceSubscription(s *subscriber, gvr schema.GroupVersionResource) {
	reh.subscribersMu.Lock()
	key := gvr.GroupResource().String()
	reh.subscribers[s][key] = struct{}{}
	reh.subscribersMu.Unlock()
}

func (reh *resourceEventHandler) deleteResourceSubscription(s *subscriber, gvr schema.GroupVersionResource) {
	reh.subscribersMu.Lock()
	key := gvr.GroupResource().String()
	delete(reh.subscribers[s], key)
	reh.subscribersMu.Unlock()
}

func (reh *resourceEventHandler) Handle(gvr schema.GroupVersionResource) cache.ResourceEventHandlerFuncs {

	key := gvr.GroupResource().String()

	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(o interface{}) {
			reh.subscribersMu.Lock()
			for s, groupResourceSubs := range reh.subscribers {
				if _, ok := groupResourceSubs[key]; ok {
					s.send(eventMessage{MessageType: "create", Message: o})
				}
			}
			reh.subscribersMu.Unlock()
		},
		UpdateFunc: func(old, updated interface{}) {
			reh.subscribersMu.Lock()
			for s, groupResourceSubs := range reh.subscribers {
				if _, ok := groupResourceSubs[key]; ok {
					s.send(eventMessage{MessageType: "update", Message: updated})
				}
			}
			reh.subscribersMu.Unlock()
		},
		DeleteFunc: func(old interface{}) {
			reh.subscribersMu.Lock()
			for s, groupResourceSubs := range reh.subscribers {
				if _, ok := groupResourceSubs[key]; ok {
					s.send(eventMessage{MessageType: "delete", Message: old})
				}
			}
			reh.subscribersMu.Unlock()
		},
	}
}
