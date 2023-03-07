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

/*
Там примерно такая логика:
Клиент подключается.
Клиент ожидает пинги. Если их не будет, он будет считать коннекшн stale и переконнекчиваться.

	{ type: "ping" }

Клиент делает запрос

	{ command: "subscribe",         identifier: "{\"channel\": \"GroupResourceChannel\", "groupResource": "deckhouse.io/openstackinstanceclasses"}"}

Клиент ожидает ответ

	{ type: "confirm_subscription", identifier: "{\"channel\": \"GroupResourceChannel\", "groupResource": "deckhouse.io/openstackinstanceclasses"}"}

Клиент ожидает сообщения в канал

	{ identifier: "{\"channel\": \"GroupResourceChannel\", "groupResource": "deckhouse.io/openstackinstanceclasses"}",
	  message: {
		message_type: create|update|delete
		message: OBJECT
	  }
	}
*/

// Message represents incoming client message
// https://github.com/anycable/anycable-go/blob/master/common/common.go#LL185-L190C2
type cableCommandPayload struct {
	Command    string `json:"command"`
	Identifier string `json:"identifier"`
	// Data       interface{} `json:"data,omitempty"`
}

type cableMessagePayload struct {
	Identifier string       `json:"identifier"`
	Message    eventMessage `json:"message"`
}
type eventMessage struct {
	MessageType string      `json:"message_type"`
	Message     interface{} `json:"message"`
}

type groupResourceIdentifier struct {
	Channel       string `json:"channel"`
	GroupResource string `json:"groupResource"`
}

// parseIdentifierGroupResource expects GROUP/RESOURCE notation to parse into schema.GroupResource,
//
//	e.g. deckhouse.io/openstackinstanceslasses
func parseIdentifierGroupResource(s string) (gr schema.GroupResource, err error) {
	parts := strings.Split(s, "/")
	if len(parts) == 1 {
		gr.Resource = s
	} else if len(parts) == 2 {
		gr.Group, gr.Resource = parts[0], parts[1]
	} else {
		err = fmt.Errorf("cannot parse GroupResource: %q", s)
	}
	return
}

func rejectMessage(err error) interface{} {
	return map[string]string{
		"type":   "rejected",
		"reason": err.Error(),
	}
}

func confirmSubMessage(identifier string) interface{} {
	return map[string]string{
		"type":       "confirm_subscription",
		"identifier": identifier,
	}
}

func confirmUnsubMessage(identifier string) interface{} {
	return map[string]string{
		"type":       "confirm_unsubscription",
		"identifier": identifier,
	}
}

type subscriber struct {
	msgs      chan []byte
	closeSlow func()
}

func (s *subscriber) send(msg cableMessagePayload) {
	b, _ := json.Marshal(msg)

	select {
	case s.msgs <- b:
	default:
		go s.closeSlow()
	}
}

func gvrIdentifier(gvr schema.GroupVersionResource) string {
	b, _ := json.Marshal(map[string]string{
		"channel":       "GroupResourceChannel",
		"groupResource": gvr.GroupResource().String(),
	})
	return string(b)
}

func newSubscriptionController(resourceEventHandler *resourceEventHandler) *subscriptionController {
	return &subscriptionController{
		// subscribers:             make(map[*subscriber]struct{}),
		subscriberMessageBuffer: 16,
		resourceEventHandler:    resourceEventHandler,
	}
}

type subscriptionController struct {
	// subscribers   map[*subscriber]struct{}
	// subscribersMu sync.Mutex

	// subscriberMessageBuffer controls the max number
	// of messages that can be queued for a subscriber
	// before it is kicked.
	//
	// Defaults to 16.
	subscriberMessageBuffer int

	resourceEventHandler *resourceEventHandler
}

func (sc *subscriptionController) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case evMessage := <-sc.resourceEventHandler.Data():
				evMessage.subscriber.send(cableMessagePayload{
					Identifier: gvrIdentifier(evMessage.gvr),
					Message:    evMessage.message,
				})
			// case data := <-sc.discoveryHandler:
			// 	data.subscriber.send(cableMessagePayload{
			// 		Identifier: `{"channel": "DiscoveryChannel"}`,
			// 		Message:    data.message,
			// 	})
			case <-ctx.Done():
				return
			}
		}
	}()
}

// // addSubscriber registers a subscriber.
// func (sc *subscriptionController) addSubscriber(s *subscriber) {
// 	sc.subscribersMu.Lock()
// 	sc.subscribers[s] = struct{}{}
// 	sc.subscribersMu.Unlock()
// }

// // deleteSubscriber deletes the given subscriber.
// func (sc *subscriptionController) deleteSubscriber(s *subscriber) {
// 	sc.subscribersMu.Lock()
// 	delete(sc.subscribers, s)
// 	sc.subscribersMu.Unlock()
// }

// subscribe handles the user subscription
func (sc *subscriptionController) subscribe(ctx context.Context, conn *websocket.Conn) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	s := &subscriber{
		msgs: make(chan []byte, sc.subscriberMessageBuffer),
		closeSlow: func() {
			conn.Close(websocket.StatusPolicyViolation, "connection too slow to keep up with messages")
		},
	}
	// sc.addSubscriber(s)
	// defer sc.deleteSubscriber(s)

	in := make(chan cableCommandPayload)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				var msg cableCommandPayload
				if err := wsjson.Read(ctx, conn, &msg); err != nil {
					klog.V(5).ErrorS(err, "reading JSON from websocket")
					continue
				}
				in <- msg
			}
		}
	}()

	// Sending pings to keep the connection alive, frontend considers the connection stale after 6s
	// of silence.
	ticker := time.NewTicker(time.Second * 3)
	defer ticker.Stop()

	writeTimeout := 5 * time.Second // TODO: move to config
	for {
		select {
		case <-ticker.C:
			err := writeWithTimeout(ctx, writeTimeout, conn, []byte(`{"type":"ping"}`))
			if err != nil {
				return err
			}
		case msg := <-s.msgs:
			//
			err := writeWithTimeout(ctx, writeTimeout, conn, msg)
			if err != nil {
				return err
			}
		case command := <-in:
			fmt.Println("ws received", command)
			resp := sc.dispatchCommand(s, command)
			msg, _ := json.Marshal(resp)
			err := writeWithTimeout(ctx, writeTimeout, conn, msg)
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (sc *subscriptionController) dispatchCommand(s *subscriber, command cableCommandPayload) interface{} {
	var grID groupResourceIdentifier
	if err := json.Unmarshal([]byte(command.Identifier), &grID); err == nil {
		if grID.Channel == "GroupResourceChannel" {
			switch command.Command {
			case "subscribe":
				gr, err := parseIdentifierGroupResource(grID.GroupResource)
				if err != nil {
					return rejectMessage(err)
				}
				sc.resourceEventHandler.addResourceSubscription(s, gr)
				return confirmSubMessage(command.Identifier)

			case "unsubscribe":
				gr, err := parseIdentifierGroupResource(grID.GroupResource)
				if err != nil {
					return rejectMessage(err)
				}
				sc.resourceEventHandler.deleteResourceSubscription(s, gr)
				return confirmUnsubMessage(command.Identifier)
			}
		}
	}

	// DiscoveryChannel
	// NamedResourceChannel, e.g. ModuleConfig/deckhouse

	return map[string]string{
		"type": "rejected",
	}
}

type resourceEventMessage struct {
	gvr        schema.GroupVersionResource
	subscriber *subscriber
	message    eventMessage
}

func newResourceEventHandler() *resourceEventHandler {
	return &resourceEventHandler{
		subscribers: make(map[*subscriber]map[string]struct{}),
		data:        make(chan resourceEventMessage),
	}
}

func (reh *resourceEventHandler) Data() <-chan resourceEventMessage {
	return reh.data
}

type resourceEventHandler struct {
	subscribers   map[*subscriber]map[string]struct{}
	subscribersMu sync.Mutex

	data chan resourceEventMessage
}

func writeWithTimeout(ctx context.Context, timeout time.Duration, c *websocket.Conn, msg []byte) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return c.Write(ctx, websocket.MessageText, msg)
}

func (reh *resourceEventHandler) addResourceSubscription(s *subscriber, gr schema.GroupResource) {
	reh.subscribersMu.Lock()
	key := gr.String()
	if _, ok := reh.subscribers[s]; !ok {
		reh.subscribers[s] = make(map[string]struct{})
	}
	reh.subscribers[s][key] = struct{}{}
	reh.subscribersMu.Unlock()
}

func (reh *resourceEventHandler) deleteResourceSubscription(s *subscriber, gr schema.GroupResource) {
	reh.subscribersMu.Lock()
	key := gr.String()
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

					reh.data <- resourceEventMessage{
						gvr:        gvr,
						subscriber: s,
						message: eventMessage{
							MessageType: "create",
							Message:     o,
						},
					}

				}
			}
			reh.subscribersMu.Unlock()
		},
		UpdateFunc: func(old, updated interface{}) {
			reh.subscribersMu.Lock()
			for s, groupResourceSubs := range reh.subscribers {
				if _, ok := groupResourceSubs[key]; ok {
					reh.data <- resourceEventMessage{
						gvr:        gvr,
						subscriber: s,
						message: eventMessage{
							MessageType: "update",
							Message:     updated,
						},
					}

				}
			}
			reh.subscribersMu.Unlock()
		},
		DeleteFunc: func(old interface{}) {
			reh.subscribersMu.Lock()
			for s, groupResourceSubs := range reh.subscribers {
				if _, ok := groupResourceSubs[key]; ok {
					reh.data <- resourceEventMessage{
						gvr:        gvr,
						subscriber: s,
						message: eventMessage{
							MessageType: "delete",
							Message:     old,
						},
					}

				}
			}
			reh.subscribersMu.Unlock()
		},
	}
}
