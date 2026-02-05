package memberlist

import (
	"context"
	"errors"
	"fencing-agent/internal/domain"
	"fencing-agent/internal/helper/logger/sl"
	"log/slog"
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
	numNodes int,
	eventHandler EventHandler,
	receiver NodesNumberReceiver,
) (*Memberlist, error) {

	config := memberlist.DefaultLANConfig()

	config.Name = nodeName
	config.AdvertiseAddr = nodeIP

	config.BindPort = int(cfg.MemberListPort)
	config.AdvertisePort = int(cfg.MemberListPort)

	delegate := NewDelegate(logger, func() int {
		return numNodes
	}, receiver)

	config.Delegate = delegate

	config.Events = eventHandler

	list, err := memberlist.Create(config)
	if err != nil {
		return nil, err
	}

	ml := &Memberlist{
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
	const triesLimit = 5
	currenTry := 1
	numberJoin, memberErr := ml.list.Join(peers)
	base, mx := time.Second, time.Minute
	for backoff := base; memberErr != nil; backoff <<= 1 {
		if currenTry > triesLimit {
			ml.logger.Error("failed to start memberlist, tries limit reached", sl.Err(memberErr))
			return memberErr
		}
		if backoff > mx {
			backoff = mx
		}
		ml.logger.Warn("failed to start memberlist", sl.Err(memberErr), slog.String("backoff", backoff.String()), slog.Int("tries", currenTry))

		time.Sleep(backoff)

		numberJoin, memberErr = ml.list.Join(peers)
		currenTry++
	}
	ml.logger.Info("memberlist started successfully", slog.Int("number_joined", numberJoin))
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
