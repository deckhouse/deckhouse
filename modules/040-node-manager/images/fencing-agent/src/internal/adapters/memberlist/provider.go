package memberlist

import (
	"context"
	fencingconfig "fencing-agent/internal/config"
	"fencing-agent/internal/core/domain"
	"fencing-agent/internal/lib/logger/sl"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

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
	isAlone      atomic.Bool
}

func NewProvider(cfg fencingconfig.MemberlistConfig, logger *log.Logger, eventHandler EventHandler, nodeIP string, nodeName string) (*Provider, error) {
	config := createConfig(cfg, eventHandler, nodeIP, nodeName)
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
	const op = "memberlist.Provider.Start"
	if len(peers) == 0 {
		p.logger.Info("No other peers in node group, starting as a single node", slog.String("op", op))
		p.isAlone.Store(true)
		return nil
	}

	numJoined, err := p.list.Join(peers) // numJoined
	if err != nil {
		return err
	}
	p.logger.Info("Joined to memberlist cluster", slog.Int("numJoined", numJoined), slog.String("op", op))
	p.isAlone.Store(false)
	return nil
}

func (p *Provider) Stop(ctx context.Context) error {
	if p.isAlone.Load() {
		p.logger.Debug("node was running alone, no cluster to leave")
		return nil
	}

	done := make(chan error, 1)
	go func() {
		if err := p.list.Leave(3 * time.Second); err != nil {
			done <- fmt.Errorf("error leaving memberlist: %w", err)
			return
		}
		done <- nil
	}()

	select {
	case err := <-done:
		if err != nil {
			p.logger.Warn("memberlist leave failed, forcing shutdown", sl.Err(err))

			if shutdownErr := p.list.Shutdown(); shutdownErr != nil {
				return fmt.Errorf("memberlist shutdown failed: %w", shutdownErr)
			}
		}
		p.logger.Info("memberlist leave gracefully")
		return nil
	case <-ctx.Done():
		p.logger.Warn("memberlist leave timeout, forcing shutdown")
		if err := p.list.Shutdown(); err != nil {
			return fmt.Errorf("memberlist shutdown failed: %w", err)
		}
		return nil
	}
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
	cfg fencingconfig.MemberlistConfig,
	eventHandler EventHandler,
	nodeIP string,
	nodeName string) *memberlist.Config {
	config := memberlist.DefaultLANConfig()
	config.ProbeInterval = cfg.ProbeInterval
	config.ProbeTimeout = cfg.ProbeTimeout
	config.SuspicionMult = int(cfg.SuspicionMult)
	config.IndirectChecks = int(cfg.IndirectChecks)
	config.GossipInterval = cfg.GossipInterval
	config.RetransmitMult = int(cfg.RetransmitMult)
	config.GossipToTheDeadTime = cfg.GossipToTheDeadTime
	config.BindPort = int(cfg.MemberListPort)
	config.AdvertisePort = int(cfg.MemberListPort)
	config.Name = nodeName
	config.AdvertiseAddr = nodeIP
	config.Events = eventHandler
	return config
}
