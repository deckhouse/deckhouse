package usecase

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
type Notifier struct {
	logger   *log.Logger
	eventBus Producer
}

func NewNotifier(logger *log.Logger, eventBus Producer) *Notifier {
	return &Notifier{logger: logger, eventBus: eventBus}
}

func (h *Notifier) NotifyJoin(node *memberlist.Node) {
	h.logger.Debug("node joined", slog.String("node_name", node.Name), slog.String("node_addr", node.Addr.String()))
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

func (h *Notifier) NotifyLeave(node *memberlist.Node) {
	h.logger.Debug("node left", slog.String("node_name", node.Name), slog.String("node_addr", node.Addr.String()))
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

func (h *Notifier) NotifyUpdate(node *memberlist.Node) {
	h.logger.Debug("node updated", slog.String("node", node.Name))
}
