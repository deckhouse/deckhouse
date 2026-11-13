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

// Package memberlist runs the memberlist instance of the main network.
package memberlist

import (
	"context"
	"errors"
	"fmt"
	stdlog "log"
	"math/rand/v2"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	hcml "github.com/hashicorp/memberlist"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/domain"
)

const (
	// Port is port_ml: the TCP and UDP port of the main network.
	Port = 8500

	handoffQueueDepth = 8192
	leaveTimeout      = 3 * time.Second
	// stepTimeout limits each step of a recreation that has no timeout of its own.
	stepTimeout = 5 * time.Second
	createRetry = 200 * time.Millisecond
	// joinFanout is the number of missing peers in one Join attempt. One
	// successful push/pull gives the full member list, so a few peers are enough.
	joinFanout = 3
)

// Config is the configuration of one memberlist instance. A change of a field
// recreates the instance.
type Config struct {
	NodeName string
	// Label is the cluster UUID. memberlist drops packets and streams with another label.
	Label string
	// Addr is the own IPv4 address of the main network. The instance listens only on it.
	Addr netip.Addr
	Port int
	// Timings are the memberlist timings of the fastest active profile.
	Timings        v1alpha1.FencingSLAProfileMemberlist
	RejoinInterval time.Duration
}

func (c Config) validate() error {
	switch {
	case c.NodeName == "":
		return errors.New("node name is not set")
	case c.Label == "":
		return errors.New("label is not set")
	case !c.Addr.Is4():
		return fmt.Errorf("address %q is not IPv4", c.Addr)
	case c.Timings.ProbeInterval.Duration <= 0:
		return errors.New("probe interval is not positive")
	case c.RejoinInterval <= 0:
		return errors.New("rejoin interval is not positive")
	}

	return nil
}

type Deps struct {
	// Peers gives the static peer list for Join.
	Peers func() []domain.Peer
	// OnJoin gets each node that becomes a member.
	OnJoin func(name string, addr netip.Addr)
	// OnMeta gets a copy of the NodeMeta of a member at each join and update,
	// and nil when the member leaves or fails. memberlist changes the Meta of
	// the nodes from Members() without a lock, so read the NodeMeta only here.
	OnMeta func(name string, meta []byte)
	// Encrypt sets the keyring, the encryption checks and the Delegate of
	// the messages and of the NodeMeta in the memberlist config. It must not
	// set Events or Transport.
	Encrypt func(*hcml.Config)
	// Attach gets each new instance after Create and nil before its Shutdown.
	Attach func(*hcml.Memberlist)
}

type instance struct {
	cfg    Config
	list   *hcml.Memberlist
	port   int
	cancel context.CancelFunc
	// done closes when the Join loop stops.
	done chan struct{}
}

// Network keeps one memberlist instance of the main network and recreates it
// in the process with the same name. Apply and Leave change the instance:
// call them from one goroutine. The other methods are safe for concurrent use.
type Network struct {
	deps     Deps
	logger   *log.Logger
	delegate *delegate

	cur atomic.Pointer[instance]

	// addrOf gives the address of a peer. Tests replace it to run many agents on one IP.
	addrOf func(p domain.Peer, port int) string
}

func New(deps Deps, logger *log.Logger) *Network {
	return &Network{
		deps:     deps,
		logger:   logger,
		delegate: newDelegate(),
		addrOf: func(p domain.Peer, port int) string {
			return netip.AddrPortFrom(p.Addr, uint16(port)).String()
		},
	}
}

// Run delivers the member events to the log and to OnJoin. It blocks until ctx ends.
func (n *Network) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case e := <-n.delegate.events:
			n.handle(e)
		}
	}
}

func (n *Network) handle(e event) {
	if e.kind == eventUpdate {
		n.logger.Debug("membership changed", "event", e.kind, "member", e.name, "address", e.addr)
	} else {
		n.logger.Info("membership changed", "event", e.kind, "member", e.name, "address", e.addr)
	}

	if dropped := n.delegate.droppedEvents.Swap(0); dropped > 0 {
		n.logger.Warn("member events were dropped: the queue is full", "count", dropped)
	}

	if e.kind == eventJoin && n.deps.OnJoin != nil {
		n.deps.OnJoin(e.name, e.addr)
	}

	if n.deps.OnMeta != nil {
		meta := e.meta
		if e.kind == eventLeave {
			meta = nil
		}

		n.deps.OnMeta(e.name, meta)
	}
}

// Apply creates the instance or recreates it when cfg changes. It returns when
// the new instance listens; Join runs in the background. After an error the
// network has no instance: call Apply again.
func (n *Network) Apply(cfg Config) error {
	if old := n.cur.Load(); old != nil && old.cfg == cfg {
		return nil
	}

	return n.Recreate(cfg)
}

// Recreate creates the instance again with cfg, also when cfg is the same. The
// new instance does not keep the members and their states. It returns when the
// new instance listens; Join runs in the background. After an error the
// network has no instance: call Apply or Recreate again.
func (n *Network) Recreate(cfg Config) error {
	if err := cfg.validate(); err != nil {
		return fmt.Errorf("memberlist config: %w", err)
	}

	if old := n.cur.Load(); old != nil {
		// No Leave: the peers keep the node alive while the new instance
		// takes over the same name.
		n.cur.Store(nil)
		n.close(old, false)
	}

	list, err := n.create(cfg)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	inst := &instance{
		cfg:    cfg,
		list:   list,
		port:   int(list.LocalNode().Port),
		cancel: cancel,
		done:   make(chan struct{}),
	}

	n.cur.Store(inst)
	n.attach(list)

	go n.rejoin(ctx, inst)

	n.logger.Info("main network instance created",
		"address", cfg.Addr, "port", inst.port, "probe_interval", cfg.Timings.ProbeInterval.Duration)

	return nil
}

// Leave announces the leave to the peers, waits up to 3 s and stops the instance.
func (n *Network) Leave() {
	inst := n.cur.Swap(nil)
	if inst == nil {
		return
	}

	n.close(inst, true)

	select {
	case <-inst.done:
	case <-time.After(stepTimeout):
		n.logger.Warn("join loop did not stop in time")
	}
}

// Members returns the number of members without the own node.
func (n *Network) Members() int {
	inst := n.cur.Load()
	if inst == nil {
		return 0
	}

	return max(0, inst.list.NumMembers()-1)
}

func (n *Network) attach(list *hcml.Memberlist) {
	if n.deps.Attach != nil {
		n.deps.Attach(list)
	}
}

func (n *Network) create(cfg Config) (*hcml.Memberlist, error) {
	deadline := time.Now().Add(stepTimeout)

	for {
		mlCfg, err := n.buildConfig(cfg)
		if err != nil {
			return nil, err
		}

		// Create only binds the sockets. It retries while the closed instance
		// releases the port.
		list, err := hcml.Create(mlCfg)
		if err == nil {
			if err := raiseRecvBuf(cfg.Addr, int(list.LocalNode().Port)); err != nil {
				n.logger.Warn("cannot raise the UDP receive buffer", "error", err)
			}

			return list, nil
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("create memberlist: %w", err)
		}

		time.Sleep(createRetry)
	}
}

func (n *Network) buildConfig(cfg Config) (*hcml.Config, error) {
	c := hcml.DefaultLANConfig()

	if n.deps.Encrypt != nil {
		n.deps.Encrypt(c)
	}

	if c.Keyring == nil || !c.GossipVerifyIncoming || !c.GossipVerifyOutgoing {
		return nil, errors.New("memberlist encryption is not configured")
	}

	c.Name = cfg.NodeName
	c.Label = cfg.Label
	c.BindAddr = cfg.Addr.String()
	c.BindPort = cfg.Port
	c.AdvertiseAddr = cfg.Addr.String()
	c.AdvertisePort = cfg.Port

	t := cfg.Timings
	c.ProbeInterval = t.ProbeInterval.Duration
	c.ProbeTimeout = t.ProbeTimeout.Duration
	c.SuspicionMult = int(t.SuspicionMult)
	c.SuspicionMaxTimeoutMult = int(t.SuspicionMaxTimeoutMult)
	c.IndirectChecks = int(t.IndirectChecks)
	c.AwarenessMaxMultiplier = int(t.AwarenessMaxMultiplier)
	c.GossipInterval = t.GossipInterval.Duration
	c.GossipNodes = int(t.GossipNodes)
	c.RetransmitMult = int(t.RetransmitMult)
	c.DeadNodeReclaimTime = t.DeadNodeReclaimTime.Duration
	c.GossipToTheDeadTime = t.GossipToTheDeadTime.Duration

	c.EnableCompression = false
	c.HandoffQueueDepth = handoffQueueDepth
	c.Events = n.delegate
	c.Logger = stdlog.New(newLogWriter(n.logger), "", 0)

	return c, nil
}

// close stops the Join loop and the instance. Shutdown has no timeout of its
// own, so close does not wait for it longer than stepTimeout.
func (n *Network) close(inst *instance, leave bool) {
	inst.cancel()
	n.attach(nil)

	if leave {
		if err := inst.list.Leave(leaveTimeout); err != nil {
			n.logger.Warn("memberlist leave failed", "error", err)
		}
	}

	shutdown := make(chan error, 1)
	go func() { shutdown <- inst.list.Shutdown() }()

	select {
	case err := <-shutdown:
		if err != nil {
			n.logger.Warn("memberlist shutdown failed", "error", err)
		}
	case <-time.After(stepTimeout):
		n.logger.Warn("memberlist shutdown did not end in time")
	}
}

// rejoin joins the static peers that are not members, at once and then every
// RejoinInterval. memberlist does not contact a dead member after
// GossipToTheDeadTime, so this loop also joins the halves of a healed partition.
func (n *Network) rejoin(ctx context.Context, inst *instance) {
	defer close(inst.done)

	ticker := time.NewTicker(inst.cfg.RejoinInterval)
	defer ticker.Stop()

	failing := false

	for {
		err := n.joinMissing(inst)

		switch {
		case err != nil && !failing:
			n.logger.Warn("join failed, retrying", "interval", inst.cfg.RejoinInterval, "error", err)
		case err != nil:
			n.logger.Debug("join failed", "error", err)
		case failing:
			n.logger.Info("join succeeded")
		}

		failing = err != nil

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (n *Network) joinMissing(inst *instance) error {
	members := make(map[string]bool)
	for _, m := range inst.list.Members() {
		members[m.Name] = true
	}

	var seeds []string
	for _, p := range n.deps.Peers() {
		if !members[p.Name] && p.Addr.IsValid() {
			seeds = append(seeds, n.addrOf(p, inst.port))
		}
	}

	rand.Shuffle(len(seeds), func(i, j int) { seeds[i], seeds[j] = seeds[j], seeds[i] })

	_, err := joinEach(seeds[:min(len(seeds), joinFanout)], func(seed string) (int, error) {
		return inst.list.Join([]string{seed})
	})

	return err
}

// joinEach joins every seed in its own goroutine. It succeeds when one seed
// succeeds, else it returns all errors in the order of seeds.
func joinEach(seeds []string, join func(seed string) (int, error)) (int, error) {
	joined := make([]int, len(seeds))
	errs := make([]error, len(seeds))

	var wg sync.WaitGroup
	for i, seed := range seeds {
		wg.Go(func() { joined[i], errs[i] = join(seed) })
	}
	wg.Wait()

	total := 0
	for _, n := range joined {
		total += n
	}

	if total > 0 {
		return total, nil
	}

	return 0, errors.Join(errs...)
}
