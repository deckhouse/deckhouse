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
	"errors"
	"fmt"
	stdlog "log"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	hcml "github.com/hashicorp/memberlist"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
)

const (
	bindAddress = "0.0.0.0"

	minTCPTimeout   = time.Second
	minLeaveTimeout = 250 * time.Millisecond
	maxLeaveTimeout = 4 * time.Second
)

type Config struct {
	NodeName  string
	NodeGroup string
	// AdvertiseAddr must be the Node InternalIP. Peers reach the pod through the
	// hostPort on the Node, so the auto-detected pod IP is unreachable.
	AdvertiseAddr string
	Port          int
	// Tuning carries the SLA profile timings. It is required: the zero value
	// disables probing.
	Tuning     v1alpha1.FencingSLAProfileMemberlist
	APITimeout time.Duration
}

type Timings struct {
	TCPTimeout          time.Duration
	PushPullInterval    time.Duration
	DeadNodeReclaimTime time.Duration
	LeaveTimeout        time.Duration
}

func DeriveTimings(tuning v1alpha1.FencingSLAProfileMemberlist, apiTimeout time.Duration) Timings {
	tcpTimeout := max(apiTimeout, minTCPTimeout)

	return Timings{
		TCPTimeout:          tcpTimeout,
		PushPullInterval:    max(tuning.GossipToTheDeadTime.Duration, 2*tcpTimeout),
		DeadNodeReclaimTime: tuning.GossipToTheDeadTime.Duration,
		LeaveTimeout:        leaveTimeout(tuning),
	}
}

func leaveTimeout(tuning v1alpha1.FencingSLAProfileMemberlist) time.Duration {
	gi := tuning.GossipInterval.Duration
	rm := int64(tuning.RetransmitMult)
	if rm <= 0 || gi <= 0 {
		return minLeaveTimeout
	}
	if gi > maxLeaveTimeout/time.Duration(2*rm) {
		return maxLeaveTimeout
	}
	return max(time.Duration(2*rm)*gi, minLeaveTimeout)
}

type Cluster struct {
	list         *hcml.Memberlist
	logger       *log.Logger
	stop         chan struct{}
	events       *eventDelegate
	leaveTimeout time.Duration
}

func New(cfg Config, logger *log.Logger) (*Cluster, error) {
	// A zero ProbeInterval silently disables probing: the agent would join gossip
	// and never detect a failure. Refuse to start instead.
	if cfg.Tuning.ProbeInterval.Duration <= 0 {
		return nil, errors.New("memberlist tuning is not set: ProbeInterval must be positive")
	}

	events := newEventDelegate(logger)

	list, err := hcml.Create(buildConfig(cfg, logger, events))
	if err != nil {
		return nil, fmt.Errorf("create memberlist: %w", err)
	}

	stop := make(chan struct{})
	go events.run(stop)

	return &Cluster{
		list:         list,
		logger:       logger,
		stop:         stop,
		events:       events,
		leaveTimeout: DeriveTimings(cfg.Tuning, cfg.APITimeout).LeaveTimeout,
	}, nil
}

func buildConfig(cfg Config, logger *log.Logger, events hcml.EventDelegate) *hcml.Config {
	timings := DeriveTimings(cfg.Tuning, cfg.APITimeout)
	mlCfg := hcml.DefaultLANConfig()

	mlCfg.Name = cfg.NodeName
	mlCfg.BindAddr = bindAddress
	mlCfg.BindPort = cfg.Port
	mlCfg.AdvertiseAddr = cfg.AdvertiseAddr
	mlCfg.AdvertisePort = cfg.Port
	// Label keeps each NodeGroup a separate gossip network; foreign packets are dropped.
	mlCfg.Label = cfg.NodeGroup
	mlCfg.TCPTimeout = timings.TCPTimeout
	mlCfg.PushPullInterval = timings.PushPullInterval
	mlCfg.DeadNodeReclaimTime = timings.DeadNodeReclaimTime
	mlCfg.ProbeInterval = cfg.Tuning.ProbeInterval.Duration
	mlCfg.ProbeTimeout = cfg.Tuning.ProbeTimeout.Duration
	mlCfg.SuspicionMult = int(cfg.Tuning.SuspicionMult)
	mlCfg.SuspicionMaxTimeoutMult = int(cfg.Tuning.SuspicionMaxTimeoutMult)
	mlCfg.IndirectChecks = int(cfg.Tuning.IndirectChecks)
	mlCfg.AwarenessMaxMultiplier = int(cfg.Tuning.AwarenessMaxMultiplier)
	mlCfg.GossipInterval = cfg.Tuning.GossipInterval.Duration
	mlCfg.RetransmitMult = int(cfg.Tuning.RetransmitMult)
	mlCfg.GossipToTheDeadTime = cfg.Tuning.GossipToTheDeadTime.Duration
	mlCfg.Logger = stdlog.New(newLogWriter(logger), "", 0)
	mlCfg.Events = events

	return mlCfg
}

func (c *Cluster) Join(seeds []string) (int, error) {
	return joinEach(seeds, func(seed string) (int, error) { return c.list.Join([]string{seed}) })
}

func joinEach(seeds []string, join func(seed string) (int, error)) (int, error) {
	joined := make([]int, len(seeds))
	errs := make([]error, len(seeds))

	var wg sync.WaitGroup
	for i, seed := range seeds {
		wg.Go(func() { joined[i], errs[i] = join(seed) })
	}
	wg.Wait()

	total := 0
	var merged *multierror.Error
	for i := range seeds {
		total += joined[i]
		merged = multierror.Append(merged, errs[i])
	}

	if total > 0 {
		return total, nil
	}

	return 0, merged.ErrorOrNil()
}

func (c *Cluster) NumMembers() int {
	return c.list.NumMembers()
}

func (c *Cluster) Members() []string {
	members := c.list.Members()
	names := make([]string, 0, len(members))

	for _, member := range members {
		names = append(names, member.Name)
	}

	return names
}

func (c *Cluster) Changed() <-chan struct{} {
	return c.events.subscribe()
}

func (c *Cluster) Shutdown() error {
	defer close(c.stop)

	if err := c.list.Leave(c.leaveTimeout); err != nil {
		c.logger.Warn("memberlist leave failed, forcing shutdown", "error", err)
	}

	if err := c.list.Shutdown(); err != nil {
		return fmt.Errorf("shutdown memberlist: %w", err)
	}

	return nil
}
