package memberlist

import (
	"encoding/json"
	"fencing-agent/internal/helper/logger/sl"
	"log/slog"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/hashicorp/memberlist"
)

// BroadcastMessage is the message structure sent between cluster members
type BroadcastMessage struct {
	MemberCount int    `json:"member_count"`
	TimestampMs int64  `json:"timestamp_ms"`
	NodeName    string `json:"node_name"`
}

// MessageHandler is called when a broadcast message is received from another node
type MessageHandler func(msg BroadcastMessage)

// simpleBroadcast implements memberlist.Broadcast interface
type simpleBroadcast struct {
	msg    []byte
	notify chan struct{}
}

func (b *simpleBroadcast) Invalidates(other memberlist.Broadcast) bool {
	return false
}

func (b *simpleBroadcast) Message() []byte {
	return b.msg
}

func (b *simpleBroadcast) Finished() {
	if b.notify != nil {
		select {
		case b.notify <- struct{}{}:
		default:
		}
	}
}

// Delegate implements memberlist.Delegate interface for custom message handling
type Delegate struct {
	logger     *log.Logger
	nodeName   string
	broadcasts *memberlist.TransmitLimitedQueue
	numNodes   func() int

	mu       sync.RWMutex
	handlers []MessageHandler
}

// NewDelegate creates a new Delegate for handling memberlist broadcasts
func NewDelegate(logger *log.Logger, nodeName string, numNodes func() int) *Delegate {
	d := &Delegate{
		logger:   logger,
		nodeName: nodeName,
		numNodes: numNodes,
		handlers: make([]MessageHandler, 0),
	}

	d.broadcasts = &memberlist.TransmitLimitedQueue{
		NumNodes:       numNodes,
		RetransmitMult: 3,
	}

	return d
}

// OnMessage registers a handler that will be called when a message is received
func (d *Delegate) OnMessage(handler MessageHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers = append(d.handlers, handler)
}

// BroadcastMemberCount queues a broadcast message with current member count
func (d *Delegate) BroadcastMemberCount(memberCount int) {
	msg := BroadcastMessage{
		MemberCount: memberCount,
		TimestampMs: time.Now().UnixMilli(),
		NodeName:    d.nodeName,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		d.logger.Error("failed to marshal broadcast message", slog.Any("error", err))
		return
	}

	d.broadcasts.QueueBroadcast(&simpleBroadcast{
		msg: data,
	})

	d.logger.Debug("broadcast message queued",
		slog.Int("member_count", memberCount),
		slog.Int64("timestamp_ms", msg.TimestampMs))
}

// NodeMeta returns metadata about this node (not used)
func (d *Delegate) NodeMeta(limit int) []byte {
	return nil
}

// NotifyMsg is called when a user-data message is received
func (d *Delegate) NotifyMsg(data []byte) {
	if len(data) == 0 {
		return
	}

	var msg BroadcastMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		d.logger.Warn("failed to unmarshal broadcast message", sl.Err(err))
		return
	}

	d.logger.Debug("broadcast message received",
		slog.String("from_node", msg.NodeName),
		slog.Int("member_count", msg.MemberCount),
		slog.Int64("timestamp_ms", msg.TimestampMs))

	d.mu.RLock()
	handlers := d.handlers
	d.mu.RUnlock()

	for _, handler := range handlers {
		handler(msg)
	}
}

// GetBroadcasts returns queued broadcasts to be sent
func (d *Delegate) GetBroadcasts(overhead, limit int) [][]byte {
	return d.broadcasts.GetBroadcasts(overhead, limit)
}

func (d *Delegate) LocalState(join bool) []byte {
	return nil
}

func (d *Delegate) MergeRemoteState(buf []byte, join bool) {
}
