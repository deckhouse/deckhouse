package local

import (
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
