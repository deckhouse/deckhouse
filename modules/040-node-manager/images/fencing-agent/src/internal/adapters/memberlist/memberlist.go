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
	"fmt"
	stdlog "log"
	"time"

	hcml "github.com/hashicorp/memberlist"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	bindAddress  = "0.0.0.0"
	leaveTimeout = 3 * time.Second

	// deadNodeReclaimTime lets a hard-died node rejoin under the same name with a
	// new address; the zero default makes peers refuse it forever.
	deadNodeReclaimTime = 10 * time.Second
)

type Config struct {
	NodeName  string
	NodeGroup string
	// AdvertiseAddr must be the Node InternalIP: peers reach the pod only through
	// the hostPort on the Node, so the auto-detected pod IP is unreachable.
	AdvertiseAddr string
	Port          int
}

type Cluster struct {
	list   *hcml.Memberlist
	logger *log.Logger
	stop   chan struct{}
}

func New(cfg Config, logger *log.Logger) (*Cluster, error) {
	events := newEventDelegate(logger)

	list, err := hcml.Create(buildConfig(cfg, logger, events))
	if err != nil {
		return nil, fmt.Errorf("create memberlist: %w", err)
	}

	stop := make(chan struct{})
	go events.run(stop)

	return &Cluster{list: list, logger: logger, stop: stop}, nil
}

func buildConfig(cfg Config, logger *log.Logger, events hcml.EventDelegate) *hcml.Config {
	mlCfg := hcml.DefaultLANConfig()

	mlCfg.Name = cfg.NodeName
	mlCfg.BindAddr = bindAddress
	mlCfg.BindPort = cfg.Port
	mlCfg.AdvertiseAddr = cfg.AdvertiseAddr
	mlCfg.AdvertisePort = cfg.Port
	// Label keeps each NodeGroup a separate gossip network; foreign packets are dropped.
	mlCfg.Label = cfg.NodeGroup
	mlCfg.DeadNodeReclaimTime = deadNodeReclaimTime
	mlCfg.Logger = stdlog.New(newLogWriter(logger), "", 0)
	mlCfg.Events = events

	return mlCfg
}

func (c *Cluster) Join(seeds []string) (int, error) {
	return c.list.Join(seeds)
}

func (c *Cluster) NumMembers() int {
	return c.list.NumMembers()
}

// Shutdown leaves before closing so peers see the node as left, not dead: a
// planned restart must not look like a failure.
func (c *Cluster) Shutdown() error {
	defer close(c.stop)

	if err := c.list.Leave(leaveTimeout); err != nil {
		c.logger.Warn("memberlist leave failed, forcing shutdown", "error", err)
	}

	if err := c.list.Shutdown(); err != nil {
		return fmt.Errorf("shutdown memberlist: %w", err)
	}

	return nil
}
