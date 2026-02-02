package event

import (
	"fencing-agent/internal/domain"
	"time"

	"log/slog"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/hashicorp/memberlist"
)

type Producer interface {
	Publish(event domain.Event)
}
type EventHandler struct {
	logger   *log.Logger
	eventBus Producer
}

func NewEventHandler(logger *log.Logger, eventBus Producer) *EventHandler {
	return &EventHandler{logger: logger, eventBus: eventBus}
}

func (h *EventHandler) NotifyJoin(node *memberlist.Node) {
	h.logger.Debug("Node joined", slog.String("node_name", node.Name), slog.String("node_addr", node.Addr.String()))
	// TODO false joining?
	ips := make(map[string]string)
	ips[domain.InterfaceName] = node.Addr.String()
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
	h.logger.Debug("Node left", slog.String("node_name", node.Name), slog.String("node_addr", node.Addr.String()))
	// TODO false leaving?
	ips := make(map[string]string)
	ips[domain.InterfaceName] = node.Addr.String()
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
	h.logger.Debug("Node updated", slog.String("node", node.Name))
}
