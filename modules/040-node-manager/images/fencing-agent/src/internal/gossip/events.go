package gossip

import (
	"github.com/hashicorp/memberlist"
	"go.uber.org/zap"
)

type EventHandler struct {
	logger        *zap.Logger
	onNodeFailure func(nodeName string, nodeAddr string)
}


func NewEventHandler(logger *zap.Logger, onNodeFailure func(nodeName string, nodeAddr string)) *EventHandler {
	return &EventHandler{
		logger:        logger,
		onNodeFailure: onNodeFailure,
	}
}

func (d *EventHandler) NotifyJoin(node *memberlist.Node) {
	d.logger.Info("Node joined", zap.String("node", node.Name))
	// mean that node is not alone in cluster
	//if d.memberlist.NumMembers() > 1 {
	//	d.memberlist.SetAlone(false)
	//}
}


func (d *EventHandler) NotifyLeave(node *memberlist.Node) {
	d.logger.Info("Node left", zap.String("node", node.Name))
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
