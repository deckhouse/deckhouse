/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package memberlist

import (
	"context"
	"errors"
	"time"

	"github.com/hashicorp/memberlist"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
	"fencing-agent/internal/lib/backoff"
	"fencing-agent/internal/lib/logger"
	"fencing-agent/internal/lib/logger/sl"
)

type Config struct {
	MemberListPort      uint          `env:"MEMBERLIST_PORT" env-default:"8500"`
	ProbeInterval       time.Duration `env:"PROBE_INTERVAL" env-default:"500ms"`
	ProbeTimeout        time.Duration `env:"PROBE_TIMEOUT" env-default:"200ms"`
	SuspicionMult       uint          `env:"SUSPICION_MULT" env-default:"2"`
	IndirectChecks      uint          `env:"INDIRECT_CHECKS" env-default:"3"`
	GossipInterval      time.Duration `env:"GOSSIP_INTERVAL" env-default:"200ms"`
	RetransmitMult      uint          `env:"RETRANSMIT_MULT" env-default:"4"`
	GossipToTheDeadTime time.Duration `env:"GOSSIP_TO_THE_DEAD_TIME" env-default:"2s"`
}

func (c *Config) Validate() error {
	if c.MemberListPort == 0 {
		return errors.New("MEMBERLIST_PORT env var is empty")
	}

	if c.SuspicionMult == 0 {
		return errors.New("SUSPICION_MULT env var must be greater than zero")
	}

	if c.ProbeTimeout >= c.ProbeInterval {
		return errors.New("PROBE_TIMEOUT env var must be less than probe interval")
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

	// gossip config
	config.ProbeInterval = cfg.ProbeInterval
	config.ProbeTimeout = cfg.ProbeTimeout
	config.SuspicionMult = int(cfg.SuspicionMult)
	config.IndirectChecks = int(cfg.IndirectChecks)
	config.GossipInterval = cfg.GossipInterval
	config.RetransmitMult = int(cfg.RetransmitMult)
	config.GossipToTheDeadTime = cfg.GossipToTheDeadTime

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
