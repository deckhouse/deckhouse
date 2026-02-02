package adapters

import (
	"context"
	"fencing-agent/internal/controllers/event"
	"fencing-agent/internal/domain"

	"github.com/hashicorp/memberlist"
)

type EventHandler interface {
	NotifyJoin(node *memberlist.Node)
	NotifyLeave(node *memberlist.Node)
	NotifyUpdate(node *memberlist.Node)
}

type Memberlist struct {
	list                 *memberlist.Memberlist
	networkInterfaceName domain.NetworkInterface
}

func NewMemberlist(eventHandler event.EventHandler) *Memberlist {
	return &Memberlist{}
}

func (ml *Memberlist) GetNodes(ctx context.Context) (domain.NetworkInterface, domain.NodesInNetwork, error) {
	members := ml.list.Members()
	nodesInNetwork := domain.NodesInNetwork{
		Members: make(map[domain.NodeName]domain.Node, len(members)),
		Size:    len(members),
	}
	for _, member := range members {
		nodesInNetwork.Members[domain.NodeName(member.Name)] = domain.Node{
			Name: member.Name,
			Addr: member.Addr.String(),
		}
	}

	return ml.networkInterfaceName, nodesInNetwork, nil
}
