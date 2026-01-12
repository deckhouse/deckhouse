package memberlist

import (
	fencing_config "fencing-controller/internal/config"
	"fencing-controller/internal/core/domain"
	"fencing-controller/internal/core/ports"

	"github.com/hashicorp/memberlist"
	"go.uber.org/zap"
)

type Provider struct {
	logger   *zap.Logger
	list     *memberlist.Memberlist
	eventBus ports.EventsBus
	isAlone  bool
}

func NewProvider(cfg fencing_config.MemberlistConfig, logger *zap.Logger, eventBus ports.EventsBus) (*Provider, error) {
	config := memberlist.DefaultLANConfig()
	// TODO config
	eventHandler := NewEventHandler(logger, eventBus)
	config.Events = eventHandler
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
		// TODO logging
		p.isAlone = true
		return nil
	}

	_, err := p.list.Join(peers) // numJoined
	if err != nil {
		return err
	}
	// TODO logging
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
				"InternalIP": member.Addr.String(), // TODO think
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
