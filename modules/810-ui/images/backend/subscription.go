package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

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

// parseIdentifierGroupResource expects RESOURCE.GROUP notation to parse into schema.GroupResource,
//
//	e.g. openstackinstanceslasses.deckhouse.io
func parseIdentifierGroupResource(s string) (gr schema.GroupResource, err error) {
	if s == "" {
		err = fmt.Errorf("group-resource cannot be empty")
		return
	}
	parts := strings.SplitN(s, ".", 2)
	switch len(parts) {
	case 1:
		gr.Resource = s
	case 2:
		gr.Resource, gr.Group = parts[0], parts[1]
	default:
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
	// subscriberMessageBuffer controls the max number
	// of messages that can be queued for a subscriber
	// before it is kicked.
	//
	// Defaults to 16.
	subscriberMessageBuffer int

	resourceEventHandler *resourceEventHandler
}

func (sc *subscriptionController) Start(ctx context.Context) {
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
}

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

	in := make(chan cableCommandPayload)
	readerr := make(chan error)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				var msg cableCommandPayload
				if err := wsjson.Read(ctx, conn, &msg); err != nil {
					readerr <- err
					return
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
			klog.V(5).InfoS("received command", "command", command.Command, "identifier", command.Identifier)
			resp := sc.dispatchCommand(s, command)
			msg, _ := json.Marshal(resp)
			err := writeWithTimeout(ctx, writeTimeout, conn, msg)
			if err != nil {
				return err
			}
		case err := <-readerr:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

type channelMessage struct {
	Channel string `json:"channel"`
}

func (sc *subscriptionController) dispatchCommand(s *subscriber, command cableCommandPayload) interface{} {
	var chm channelMessage
	if err := json.Unmarshal([]byte(command.Identifier), &chm); err != nil {
		return rejectMessage(err)
	}

	if chm.Channel == "GroupResourceChannel" {
		return sc.handleGroupResourceChannelSubscription(s, command)
	}

	// TODO DiscoveryChannel 🐒🐒🐒
	// TODO NamedResourceChannel, e.g. ModuleConfig/deckhouse

	return rejectMessage(fmt.Errorf("invalid subscription parameters"))
}

type groupResourceMessage struct {
	GroupResource string `json:"groupResource"`
}

func (sc *subscriptionController) handleGroupResourceChannelSubscription(s *subscriber, command cableCommandPayload) interface{} {
	var grm groupResourceMessage
	if err := json.Unmarshal([]byte(command.Identifier), &grm); err != nil {
		return rejectMessage(err)
	}
	gr, err := parseIdentifierGroupResource(grm.GroupResource)
	if err != nil {
		return rejectMessage(err)
	}
	switch command.Command {
	case "subscribe":
		sc.resourceEventHandler.addResourceSubscription(s, gr)
		return confirmSubMessage(command.Identifier)

	case "unsubscribe":
		sc.resourceEventHandler.deleteResourceSubscription(s, gr)
		return confirmUnsubMessage(command.Identifier)
	}

	return rejectMessage(fmt.Errorf("invalid subscription parameters"))
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
			defer reh.subscribersMu.Unlock()

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
		},
		UpdateFunc: func(old, updated interface{}) {
			reh.subscribersMu.Lock()
			defer reh.subscribersMu.Unlock()

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
		},
		DeleteFunc: func(old interface{}) {
			reh.subscribersMu.Lock()
			defer reh.subscribersMu.Unlock()

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
		},
	}
}

func handleSubscribe(sc *subscriptionController) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
			// Declaring supported protocol for frontend tooling based on ActionCable;
			// "actioncable-unsupported" is omitted because it seem to be unneeded.
			Subprotocols: []string{"actioncable-v1-json"},
		})
		if err != nil {
			klog.V(5).ErrorS(err, "failed to accept websocket connection")
			return
		}
		defer c.Close(websocket.StatusInternalError, "")

		err = sc.subscribe(r.Context(), c)
		if errors.Is(err, context.Canceled) {
			klog.V(5).InfoS("websocket connection closed", "context", "cancelled")
			return
		}
		if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
			websocket.CloseStatus(err) == websocket.StatusGoingAway {
			klog.V(5).InfoS("websocket connection closed", "status", websocket.CloseStatus(err))
			return
		}
		if err != nil {
			klog.V(5).ErrorS(err, "websocket connection closed with error")
			return
		}
	}
}
