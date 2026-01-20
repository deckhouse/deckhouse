package memberlist

import (
	fencing_config "fencing-agent/internal/config"
	"fencing-agent/internal/core/domain"
	"log/slog"
	"sync/atomic"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/hashicorp/memberlist"
)

type EventHandler interface {
	NotifyJoin(node *memberlist.Node)
	NotifyLeave(node *memberlist.Node)
	NotifyUpdate(node *memberlist.Node)
}
type Provider struct {
	logger       *log.Logger
	list         *memberlist.Memberlist
	eventHandler EventHandler
	nodeIp       string
	isAlone      atomic.Bool
}

func NewProvider(cfg fencing_config.MemberlistConfig, logger *log.Logger, eventHandler EventHandler, nodeIp string, nodeName string) (*Provider, error) {
	config := createConfig(cfg, eventHandler, nodeIp, nodeName)
	list, err := memberlist.Create(config)
	if err != nil {
		return nil, err // TODO think, logging
	}
	return &Provider{
		logger:       logger,
		list:         list,
		eventHandler: eventHandler,
	}, nil
}

func (p *Provider) Start(peers []string) error {
	if len(peers) == 0 {
		p.logger.Info("No other peers in node group, starting as a single node")
		p.isAlone.Store(true)
		return nil
	}

	numJoined, err := p.list.Join(peers) // numJoined
	if err != nil {
		return err
	}
	p.logger.Info("Joined to memberlist cluster", slog.Int("numJoined", numJoined))
	p.isAlone.Store(false)
	return nil
}

func (p *Provider) GetMembers() []domain.Node {
	members := p.list.Members()
	nodes := make([]domain.Node, 0, len(members))
	for _, member := range members {
		nodes = append(nodes, domain.Node{
			Name: member.Name,
			Addresses: map[string]string{
				domain.InterfaceName: member.Addr.String(),
			},
		})
	}
	return nodes
}

func (p *Provider) NumOtherMembers() int {
	return p.list.NumMembers() - 1
}

func (p *Provider) IsAlone() bool {
	return p.isAlone.Load()
}

func createConfig(
	cfg fencing_config.MemberlistConfig,
	eventHandler EventHandler,
	nodeIp string,
	nodeName string) *memberlist.Config {
	config := memberlist.DefaultLANConfig()
	config.ProbeInterval = cfg.ProbeInterval
	config.ProbeTimeout = cfg.ProbeTimeout
	config.SuspicionMult = cfg.SuspicionMult
	config.IndirectChecks = cfg.IndirectChecks
	config.GossipInterval = cfg.GossipInterval
	config.RetransmitMult = cfg.RetransmitMult
	config.GossipToTheDeadTime = cfg.GossipToTheDeadTime
	config.BindPort = cfg.MemberListPort
	config.AdvertisePort = cfg.MemberListPort
	config.Name = nodeName
	config.AdvertiseAddr = nodeIp
	config.Events = eventHandler
	return config
}
