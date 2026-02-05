package memberlist

import (
	"encoding/json"
	"fencing-agent/internal/domain"
	"fencing-agent/internal/lib/logger/sl"
	"log/slog"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/hashicorp/memberlist"
)

// NodesNumberReceiver is called when a NodesNumber message is received
type NodesNumberReceiver interface {
	SetTotalNodes(nodesNumber domain.NodesNumber)
}

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
	broadcasts *memberlist.TransmitLimitedQueue
	numNodes   func() int
	receiver   NodesNumberReceiver
}

// NewDelegate creates a new Delegate for handling memberlist broadcasts
func NewDelegate(logger *log.Logger, numNodes func() int, receiver NodesNumberReceiver) *Delegate {
	d := &Delegate{
		logger:   logger,
		numNodes: numNodes,
		receiver: receiver,
	}

	d.broadcasts = &memberlist.TransmitLimitedQueue{
		NumNodes:       numNodes,
		RetransmitMult: 3,
	}

	return d
}

// BroadcastNodesNumber queues a broadcast message with current nodes count
func (d *Delegate) BroadcastNodesNumber(totalNodes int) {
	msg := domain.NodesNumber{
		TotalNodes: totalNodes,
		Timestamp:  time.Now().UnixMilli(),
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
		slog.Int("total_nodes", totalNodes),
		slog.Int64("timestamp", msg.Timestamp))
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

	var msg domain.NodesNumber
	if err := json.Unmarshal(data, &msg); err != nil {
		d.logger.Warn("failed to unmarshal broadcast message", sl.Err(err))
		return
	}

	d.logger.Debug("broadcast message received",
		slog.Int("total_nodes", msg.TotalNodes),
		slog.Int64("timestamp", msg.Timestamp))

	if d.receiver != nil {
		d.receiver.SetTotalNodes(msg)
	}
}

// GetBroadcasts returns queued broadcasts to be sent
func (d *Delegate) GetBroadcasts(overhead, limit int) [][]byte {
	return d.broadcasts.GetBroadcasts(overhead, limit)
}

// LocalState is used for TCP push/pull state exchange (not used)
func (d *Delegate) LocalState(join bool) []byte {
	return nil
}

// MergeRemoteState handles state received from remote nodes (not used)
func (d *Delegate) MergeRemoteState(buf []byte, join bool) {
}
