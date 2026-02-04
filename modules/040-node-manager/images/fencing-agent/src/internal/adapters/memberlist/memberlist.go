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
	list                 *memberlist.Memberlist
	networkInterfaceName domain.NetworkInterface

	logger *log.Logger
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

	// TODO eventHandler

	config.BindPort = int(cfg.MemberListPort)
	config.AdvertisePort = int(cfg.MemberListPort)

	list, err := memberlist.Create(config)
	if err != nil {
		return nil, err
	}

	return &Memberlist{
		list:                 list,
		logger:               logger,
		networkInterfaceName: domain.NetworkInterface(nodeName),
	}, nil
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

func (ml *Memberlist) Start(peers ips) error {
	numJoined, err := ml.list.Join(peers)
	if err != nil {
		return err
	}
	ml.logger.Info("joined cluster", "numJoined", numJoined)
	return nil
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
