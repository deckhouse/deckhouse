package memberlist

import (
	"fencing-agent/internal/core/domain"
	"fencing-agent/internal/core/ports"
	"time"

	"github.com/hashicorp/memberlist"
	"go.uber.org/zap"
)

type EventHandler struct {
	logger   *zap.Logger
	eventBus ports.EventsBus
}

func NewEventHandler(logger *zap.Logger, eventBus ports.EventsBus) *EventHandler {
	return &EventHandler{logger: logger, eventBus: eventBus}
}

func (h *EventHandler) NotifyJoin(node *memberlist.Node) {
	h.logger.Debug("Node joined", zap.String("node_name", node.Name), zap.String("node_addr", node.Addr.String()))
	// TODO false joining?
	ips := make(map[string]string)
	ips["eth0"] = node.Addr.String()
	event := domain.Event{
		Node: domain.Node{
			Name:      node.Name,
			Addresses: ips,
		},
		EventType: domain.EventTypeJoin,
		Timestamp: time.Now().Unix(),
	}
	h.eventBus.Publish(event)
}

func (h *EventHandler) NotifyLeave(node *memberlist.Node) {
	h.logger.Debug("Node left", zap.String("node_name", node.Name), zap.String("node_addr", node.Addr.String()))
	// TODO false leaving?
	ips := make(map[string]string)
	ips["eth0"] = node.Addr.String()
	event := domain.Event{
		Node: domain.Node{
			Name:      node.Name,
			Addresses: ips,
		},
		EventType: domain.EventTypeLeave,
		Timestamp: time.Now().Unix(),
	}
	h.eventBus.Publish(event)
}

func (h *EventHandler) NotifyUpdate(node *memberlist.Node) {
	h.logger.Debug("Node updated", zap.String("node", node.Name))
}
