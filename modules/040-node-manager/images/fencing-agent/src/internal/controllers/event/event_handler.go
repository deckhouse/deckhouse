package event

import (
	"log/slog"
	"time"

	"github.com/hashicorp/memberlist"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

type Producer interface {
	Publish(event domain.Event)
}
type Handler struct {
	logger   *log.Logger
	eventBus Producer
}

func NewHandler(logger *log.Logger, eventBus Producer) *Handler {
	return &Handler{logger: logger, eventBus: eventBus}
}

func (h *Handler) NotifyJoin(node *memberlist.Node) {
	h.logger.Debug("Node joined", slog.String("node_name", node.Name), slog.String("node_addr", node.Addr.String()))
	// TODO false joining?
	event := domain.Event{
		Node: domain.Node{
			Name: node.Name,
			Addr: node.Addr.String(),
		},
		EventType: domain.EventTypeJoin,
		Timestamp: time.Now().Unix(),
	}
	h.eventBus.Publish(event)
}

func (h *Handler) NotifyLeave(node *memberlist.Node) {
	h.logger.Debug("Node left", slog.String("node_name", node.Name), slog.String("node_addr", node.Addr.String()))
	// TODO false leaving?
	event := domain.Event{
		Node: domain.Node{
			Name: node.Name,
			Addr: node.Addr.String(),
		},
		EventType: domain.EventTypeLeave,
		Timestamp: time.Now().Unix(),
	}
	h.eventBus.Publish(event)
}

func (h *Handler) NotifyUpdate(node *memberlist.Node) {
	h.logger.Debug("Node updated", slog.String("node", node.Name))
}
