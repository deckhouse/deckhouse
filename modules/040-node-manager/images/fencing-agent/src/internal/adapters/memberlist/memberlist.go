package memberlist

import (
	"context"
	"errors"
	"fencing-agent/internal/domain"
	"fencing-agent/internal/lib/backoff"
	"fencing-agent/internal/lib/logger"
	"fencing-agent/internal/lib/logger/sl"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/hashicorp/memberlist"
)

type Config struct {
	MemberListPort uint `env:"MEMBERLIST_PORT" env-default:"8500"`
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
	log *log.Logger,
	nodeIP string,
	nodeName string,
	numNodes int,
	eventHandler EventHandler,
	receiver NodesNumberReceiver,
) (*Memberlist, error) {

	config := memberlist.DefaultLANConfig()

	config.Name = nodeName
	config.AdvertiseAddr = nodeIP

	config.BindPort = int(cfg.MemberListPort)
	config.AdvertisePort = int(cfg.MemberListPort)

	delegate := NewDelegate(log, func() int {
		return numNodes
	}, receiver)

	config.Delegate = delegate

	config.Events = eventHandler

	config.LogOutput = logger.NewLogWriter(log)

	list, err := memberlist.Create(config)
	if err != nil {
		return nil, err
	}

	ml := &Memberlist{
		list:     list,
		delegate: delegate,
		logger:   log,
	}

	return ml, nil
}

func (ml *Memberlist) GetNodes(ctx context.Context) (domain.Nodes, error) {
	res := make(chan domain.Nodes, 1)

	go func() {
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

		res <- nodes
	}()

	select {
	case <-ctx.Done():
		return domain.Nodes{}, ctx.Err()
	case r := <-res:
		return r, nil
	}
}

func (ml *Memberlist) Start(ctx context.Context, peersIPs []string) error {
	wrapped := backoff.Wrap(ctx, ml.logger, 5, "memberlist",
		func() error {
			_, err := ml.list.Join(peersIPs)
			return err
		})

	err := wrapped()
	if err != nil {
		return err
	}

	ml.logger.Info("memberlist started successfully")
	return nil
}

// BroadcastNodesNumber sends current total nodes count to all cluster members
func (ml *Memberlist) BroadcastNodesNumber(nodesNumber int) {
	ml.delegate.BroadcastNodesNumber(nodesNumber)
}

func (ml *Memberlist) Stop() {
	tmpTimeout := 3 * time.Second
	if err := ml.list.Leave(tmpTimeout); err != nil {
		ml.logger.Error("failed to leave cluster, shutdown", sl.Err(err))
		if err = ml.list.Shutdown(); err != nil {
			ml.logger.Error("failed to shutdown", sl.Err(err))
			return
		}
		ml.logger.Info("shutdown successfully")
		return
	}
	ml.logger.Info("left cluster correctly")
}

func (ml *Memberlist) NumMembers() int {
	return ml.list.NumMembers()
}
