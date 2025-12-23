package gossip

import (
	"github.com/hashicorp/memberlist"
	"go.uber.org/zap"
	"sync"
	"time"
)

type EventHandler struct {
	logger      *zap.Logger
	nodesEvents   map[string]time.Time
	mu            *sync.Mutex
	onNodeFailure func(nodeName string, nodeAddr string)
	onNodeJoin       func(status bool)
	minEventInterval time.Duration
}

func NewEventHandler(logger *zap.Logger, minEventInterval time.Duration, onNodeFailure func(nodeName string, nodeAddr string), onNodeJoin func(status bool)) *EventHandler {
	return &EventHandler{
		logger:           logger,
		onNodeFailure:    onNodeFailure,
		onNodeJoin:       onNodeJoin,
		nodesEvents:      make(map[string]time.Time),
		mu:               &sync.Mutex{},
		minEventInterval: minEventInterval,
	}
}

func (d *EventHandler) NotifyJoin(node *memberlist.Node) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if t, ok := d.nodesEvents[node.Name]; ok {
		if time.Since(t) < d.minEventInterval {
			d.logger.Debug("False joining", zap.String("node", node.Name))
			return
		}
	}
	d.nodesEvents[node.Name] = time.Now()
	d.logger.Info("Node joined", zap.String("node", node.Name))
	d.onNodeJoin(false)
	// mean that node is not alone in cluster
	//if d.memberlist.NumMembers() > 1 {
	//	d.memberlist.SetAlone(false)
	//}
}

func (d *EventHandler) NotifyLeave(node *memberlist.Node) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if t, ok := d.nodesEvents[node.Name]; ok {
		if time.Since(t) < d.minEventInterval {
			d.logger.Debug("False leaving", zap.String("node", node.Name))
			return
		}
	}
	d.nodesEvents[node.Name] = time.Now()
	d.logger.Info("Node left, have to notify cilium", zap.String("node", node.Name))

	//if d.memberlist.NumMembers() == 1 {
	//	d.memberlist.SetAlone(true)
	//}
	if d.onNodeFailure != nil {
		// TODO: send notification to cilium or another network agent
	}
}

func (d *EventHandler) NotifyUpdate(node *memberlist.Node) {
	d.logger.Info("Node updated", zap.String("node", node.Name))
}
