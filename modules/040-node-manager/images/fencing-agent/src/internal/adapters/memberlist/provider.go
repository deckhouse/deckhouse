package memberlist

import (
	fencing_config "fencing-agent/internal/config"
	"fencing-agent/internal/core/domain"
	"fencing-agent/internal/core/ports"

	"github.com/hashicorp/memberlist"
	"go.uber.org/zap"
)

type Provider struct {
	logger   *zap.Logger
	list     *memberlist.Memberlist
	eventBus ports.EventsBus
	nodeIp   string
	isAlone  bool
}

func NewProvider(cfg fencing_config.MemberlistConfig, logger *zap.Logger, eventBus ports.EventsBus, nodeIp string, nodeName string) (*Provider, error) {
	config := createConfig(cfg, logger, eventBus, nodeIp, nodeName)
	list, err := memberlist.Create(config)
	if err != nil {
		return nil, err // TODO think, logging
	}
	return &Provider{
		logger:   logger,
		list:     list,
		eventBus: eventBus,
		isAlone:  true, // TODO think
	}, nil
}

func (p *Provider) Start(peers []string) error {
	if len(peers) == 0 {
		p.logger.Info("No other peers in node group, starting as a single node")
		p.isAlone = true
		return nil
	}

	numJoined, err := p.list.Join(peers) // numJoined
	if err != nil {
		return err
	}
	p.logger.Info("Joined to memberlist cluster", zap.Int("numJoined", numJoined))
	p.isAlone = false
	return nil
}

func (p *Provider) GetMembers() []domain.Node {
	members := p.list.Members()
	nodes := make([]domain.Node, len(members))
	for _, member := range members {
		nodes = append(nodes, domain.Node{
			Name: member.Name,
			Addresses: map[string]string{
				"eth0": member.Addr.String(),
			},
		})
	}
	return nodes
}

func (p *Provider) NumOtherMembers() int {
	return p.list.NumMembers() - 1
}

func (p *Provider) IsAlone() bool {
	return p.isAlone
}

func createConfig(
	cfg fencing_config.MemberlistConfig,
	logger *zap.Logger,
	eventBus ports.EventsBus,
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
	config.BindAddr = nodeIp
	eventHandler := NewEventHandler(logger, eventBus)
	config.Events = eventHandler
	return config
}
