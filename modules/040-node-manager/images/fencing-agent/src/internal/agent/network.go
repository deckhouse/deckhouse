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

package agent

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"

	hcml "github.com/hashicorp/memberlist"
	"golang.org/x/sync/errgroup"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/adapters/kubeclient"
	"fencing-agent/internal/adapters/memberlist"
	"fencing-agent/internal/adapters/peers"
	"fencing-agent/internal/config"
	"fencing-agent/internal/domain"
	"fencing-agent/internal/protocol"
)

// applyRetry is the pause before a new attempt to create memberlist.
const applyRetry = time.Second

// instances is the part of memberlist.Network that mainNetwork serializes.
type instances interface {
	Apply(memberlist.Config) error
	Recreate(memberlist.Config) error
	Leave()
}

// mainNetwork joins the static peer list, the memberlist instance and the
// protocol of the main network.
type mainNetwork struct {
	nodeName string
	self     *kubeclient.SelfWatcher
	status   domain.Status
	logger   *log.Logger

	peers *peers.List
	keys  *protocol.Keyring
	tr    *protocol.Transport
	ml    *memberlist.Network
	// inst is ml. Apply and Leave of memberlist need one goroutine, so mu
	// serializes them.
	inst instances

	mu   sync.Mutex
	left bool
	// cfg is the last memberlist configuration of Apply.
	cfg memberlist.Config

	// addr is set by Start.
	addr netip.Addr
}

// newMainNetwork gives each membership event to onMember: the NodeMeta of a
// member at its join and update, nil when the member is dead or left.
func newMainNetwork(env config.Env, deps Deps, self *kubeclient.SelfWatcher, status domain.Status, metrics domain.Metrics,
	onMember func(name string, meta *domain.NodeMeta), logger *log.Logger,
) (*mainNetwork, error) {
	bootID, err := protocol.ReadBootID(protocol.BootIDPath)
	if err != nil {
		return nil, err
	}

	n := &mainNetwork{
		nodeName: env.NodeName,
		self:     self,
		status:   status,
		logger:   logger,
		peers:    peers.New(deps.K8sClient, env.NodeName, logger),
		keys:     protocol.NewKeyring(protocol.KeyDir, env.KeySwitchDelay, status, metrics, logger),
	}

	n.tr = protocol.NewTransport(protocol.Config{
		NodeName: env.NodeName, Network: domain.NetworkMain, BootID: bootID, Keys: n.keys,
	}, logger)

	n.ml = memberlist.New(memberlist.Deps{
		Peers:  n.peers.Peers,
		OnJoin: n.peers.AddMember,
		OnMeta: func(name string, b []byte) {
			if b == nil {
				onMember(name, nil)

				return
			}

			meta, err := protocol.DecodeMeta(b)
			if err != nil {
				logger.Warn("decode the NodeMeta of a member", "member", name, "error", err)
			}

			onMember(name, &meta)
		},
		Encrypt: func(c *hcml.Config) {
			if err := n.tr.Configure(c); err != nil {
				logger.Error("configure memberlist encryption", "error", err)
			}
		},
		Attach: n.tr.Attach,
	}, logger)
	n.inst = n.ml

	return n, nil
}

func (n *mainNetwork) Peers() []domain.Peer { return n.peers.Peers() }

func (n *mainNetwork) Peer(name string) (domain.Peer, bool) { return n.peers.Peer(name) }

func (n *mainNetwork) Send(to domain.Peer, msg domain.Message) error { return n.tr.Send(to, msg) }

func (n *mainNetwork) Received() <-chan domain.Message { return n.tr.Received() }

func (n *mainNetwork) Members() int { return n.ml.Members() }

func (n *mainNetwork) UpdateMeta(update func(*domain.NodeMeta)) { n.tr.UpdateMeta(update) }

// PublishMeta changes the own NodeMeta and waits up to 1 s for its broadcast.
// It waits for an Apply in progress.
func (n *mainNetwork) PublishMeta(update func(*domain.NodeMeta)) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.tr.PublishMeta(update)
}

func (n *mainNetwork) Remove(node string) { n.peers.Remove(node) }

func (n *mainNetwork) Leaving(node string) bool { return n.peers.Leaving(node) }

// Leave leaves the network after an Apply in progress. Apply does nothing after it.
func (n *mainNetwork) Leave() {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.left = true
	n.inst.Leave()
}

func (n *mainNetwork) Run(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error { return n.peers.Run(gctx) })
	g.Go(func() error { return n.ml.Run(gctx) })
	g.Go(func() error { return n.tr.Run(gctx) })

	return g.Wait()
}

// Start waits for a valid key and the own address, then creates memberlist.
// Join runs in the background.
func (n *mainNetwork) Start(ctx context.Context, s config.Settings) error {
	if err := n.keys.Wait(ctx); err != nil {
		return err
	}

	addr, err := memberlist.WaitOwnIP(ctx, n.self.Snapshot, net.InterfaceAddrs, n.status, n.logger)
	if err != nil {
		return err
	}

	n.addr = addr

	if node, ok := n.self.Snapshot(); ok {
		n.tr.UpdateMeta(func(m *domain.NodeMeta) { m.NodeUID = node.UID })
	}

	for {
		err := n.Apply(s)
		if err == nil {
			return nil
		}

		n.logger.Error("create memberlist, retrying", "error", err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(applyRetry):
		}
	}
}

// Apply applies new Settings. memberlist is created again only when its
// configuration changes.
func (n *mainNetwork) Apply(s config.Settings) error {
	n.peers.SetRefreshInterval(s.Config.PeerListRefreshInterval.Duration)

	n.mu.Lock()
	defer n.mu.Unlock()

	if n.left {
		return nil
	}

	n.cfg = memberlist.Config{
		NodeName:       n.nodeName,
		Label:          s.Config.ClusterUUID,
		Addr:           n.addr,
		Port:           memberlist.Port,
		Timings:        s.Memberlist.Memberlist,
		RejoinInterval: s.Memberlist.Rejoin.Interval.Duration,
	}

	if err := n.inst.Apply(n.cfg); err != nil {
		return fmt.Errorf("apply memberlist config: %w", err)
	}

	return nil
}

// Recreate creates the memberlist instance again with the last configuration
// and joins the peers in the background. The new instance does not keep the
// Dead states of the old one. After an error it tries again each applyRetry
// until ctx ends. It does nothing before Start and after Leave.
func (n *mainNetwork) Recreate(ctx context.Context) {
	for {
		err := n.recreate()
		if err == nil {
			n.logger.Info("main network instance recreated: the own node has a FencingFailedNodeState")

			return
		}

		n.logger.Error("recreate memberlist, retrying", "error", err)

		select {
		case <-ctx.Done():
			return
		case <-time.After(applyRetry):
		}
	}
}

func (n *mainNetwork) recreate() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.left || n.cfg.NodeName == "" {
		return nil
	}

	return n.inst.Recreate(n.cfg)
}
