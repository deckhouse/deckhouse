package local

import (
	"context"
	"fencing-agent/internal/domain"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/hashicorp/memberlist"
)

type ips []string

type EventHandler interface {
	NotifyJoin(node *memberlist.Node)
	NotifyLeave(node *memberlist.Node)
	NotifyUpdate(node *memberlist.Node)
}

type Memberlist struct {
	logger *log.Logger
}

func NewMemberlist(log *log.Logger,
) (*Memberlist, error) {
	return &Memberlist{
		logger: log,
	}, nil
}

func (ml *Memberlist) Start(peers ips) error {
	ml.logger.Info("starting memberlist")
	return nil
}

func (ml *Memberlist) Stop() {
	ml.logger.Info("stopping memberlist")
}

func (ml *Memberlist) NumMembers() int {
	return 3
}

func (ml *Memberlist) IsAlone() bool {
	// TODO think about it later
	return false
}

func (ml *Memberlist) GetNodes(ctx context.Context) (domain.Nodes, error) {
	node1 := domain.Node{Name: "bobkov-worker-small-c0a29e27-swk4t-74rk7", Addr: "10.12.1.194"}
	node2 := domain.Node{Name: "bobkov-worker-small-c0a29e27-swk4t-b4x76", Addr: "10.12.1.201"}
	node3 := domain.Node{Name: "bobkov-worker-small-c0a29e27-swk4t-mnd4k", Addr: "10.12.0.165"}
	return domain.Nodes{Nodes: []domain.Node{node1, node2, node3}}, nil
}
