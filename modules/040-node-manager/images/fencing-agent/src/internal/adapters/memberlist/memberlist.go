package memberlist

import (
	"context"
	"errors"
	"fencing-agent/internal/domain"
	"fencing-agent/internal/helper/logger/sl"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/hashicorp/memberlist"
)

type ips []string
type Config struct {
	MemberListPort uint `env:"MEMBERLIST_PORT" env-required:"true"`
}

func (c *Config) Validate() error {
	if c.MemberListPort == 0 {
		return errors.New("MEMBERLIST_PORT env var is empty")
	}
	return nil
}

type EventHandler interface {
	NotifyJoin(node *memberlist.Node)
	NotifyLeave(node *memberlist.Node)
	NotifyUpdate(node *memberlist.Node)
}

type Memberlist struct {
	list     *memberlist.Memberlist
	delegate *Delegate
	logger   *log.Logger
}

func New(
	cfg Config,
	logger *log.Logger,
	nodeIP string,
	nodeName string,
) (*Memberlist, error) {

	config := memberlist.DefaultLANConfig()

	config.Name = nodeName
	config.AdvertiseAddr = nodeIP

	config.BindPort = int(cfg.MemberListPort)
	config.AdvertisePort = int(cfg.MemberListPort)

	// Create a temporary memberlist reference for delegate's NumNodes function
	var ml *Memberlist

	delegate := NewDelegate(logger, nodeName, func() int {
		if ml != nil && ml.list != nil {
			return ml.list.NumMembers()
		}
		return 1
	})

	config.Delegate = delegate

	list, err := memberlist.Create(config)
	if err != nil {
		return nil, err
	}

	ml = &Memberlist{
		list:     list,
		delegate: delegate,
		logger:   logger,
	}

	return ml, nil
}

func (ml *Memberlist) GetNodes(ctx context.Context) (domain.Nodes, error) {
	members := ml.list.Members()
	nodes := domain.Nodes{
		Nodes: make([]domain.Node, 0, len(members)),
	}
	for _, member := range members {
		var node domain.Node
		node.Name = member.Name
		node.Addr = member.Addr.String()
		nodes.Nodes = append(nodes.Nodes, node)
	}
	return nodes, nil
}

func (ml *Memberlist) Start(peers ips) error {
	numJoined, err := ml.list.Join(peers)
	if err != nil {
		return err
	}
	ml.logger.Info("joined cluster", "numJoined", numJoined)

	// Broadcast current member count to all peers
	ml.BroadcastMemberCount()

	return nil
}

// BroadcastMemberCount sends current member count to all cluster members
func (ml *Memberlist) BroadcastMemberCount() {
	ml.delegate.BroadcastMemberCount(ml.list.NumMembers())
}

// OnMessage registers a handler to be called when a broadcast message is received
func (ml *Memberlist) OnMessage(handler MessageHandler) {
	ml.delegate.OnMessage(handler)
}

func (ml *Memberlist) Stop() error {
	// TODO graceful leave
	tmpTimeout := 3 * time.Second
	if err := ml.list.Leave(tmpTimeout); err != nil {
		ml.logger.Error("failed to leave cluster, shutdown", sl.Err(err))
		if err = ml.list.Shutdown(); err != nil {
			ml.logger.Error("failed to shutdown", sl.Err(err))
			return err
		}
		ml.logger.Info("shutdown successfully")
	}
	ml.logger.Info("left cluster correctly")
	return nil
}

func (ml *Memberlist) NumMembers() int {
	return ml.list.NumMembers()
}

func (ml *Memberlist) IsAlone() bool {
	// TODO think about it later
	return false
}
