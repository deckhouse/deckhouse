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
	"fmt"
	"math/rand/v2"
	"net"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	hcml "github.com/hashicorp/memberlist"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

var loopback = netip.MustParseAddr("127.0.0.1")

func peer(name string) domain.Peer { return domain.Peer{Name: name, Addr: loopback} }

// testCluster runs agents in one process on 127.0.0.1. Each agent has its own port.
type testCluster struct {
	t *testing.T

	mu     sync.Mutex
	ports  map[string]int
	agents []*testAgent
}

type testAgent struct {
	name  string
	net   *Network
	cfg   Config
	peers func() []domain.Peer

	mu     sync.Mutex
	leaves []string
}

func newTestCluster(t *testing.T) *testCluster {
	return &testCluster{t: t, ports: map[string]int{}}
}

// freePort returns a port that is free for TCP and UDP on 127.0.0.1.
func freePort(t *testing.T) int {
	t.Helper()

	for range 100 {
		ln, err := net.ListenTCP("tcp", &net.TCPAddr{IP: loopback.AsSlice()})
		if err != nil {
			t.Fatalf("reserve a port: %v", err)
		}
		port := ln.Addr().(*net.TCPAddr).Port

		conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: loopback.AsSlice(), Port: port})
		ln.Close()
		if err == nil {
			conn.Close()
			return port
		}
	}

	t.Fatal("no port is free for TCP and UDP")
	return 0
}

func (c *testCluster) addrOf(p domain.Peer, _ int) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return fmt.Sprintf("127.0.0.1:%d", c.ports[p.Name])
}

// allPeers is the static peer list: every agent of the cluster except name.
func (c *testCluster) allPeers(name string) func() []domain.Peer {
	return func() []domain.Peer {
		c.mu.Lock()
		defer c.mu.Unlock()

		var out []domain.Peer
		for other := range c.ports {
			if other != name {
				out = append(out, peer(other))
			}
		}

		return out
	}
}

// start creates and starts an agent. With OnJoin or OnMeta, Network.Run reads
// the member events; else the agent records the leave events.
func (c *testCluster) start(name, label string, timings Config, peers func() []domain.Peer, deps Deps) *testAgent {
	c.t.Helper()

	port := freePort(c.t)
	c.mu.Lock()
	c.ports[name] = port
	c.mu.Unlock()

	if peers == nil {
		peers = c.allPeers(name)
	}
	if deps.Encrypt == nil {
		deps.Encrypt = encrypt(testKey)
	}
	deps.Peers = peers

	a := &testAgent{name: name, peers: peers}
	a.net = New(deps, log.NewNop())
	a.net.addrOf = c.addrOf

	a.cfg = timings
	a.cfg.NodeName = name
	a.cfg.Label = label
	a.cfg.Addr = loopback
	a.cfg.Port = port

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if deps.OnJoin != nil || deps.OnMeta != nil {
			_ = a.net.Run(ctx)
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			case e := <-a.net.delegate.events:
				if e.kind == eventLeave {
					a.mu.Lock()
					a.leaves = append(a.leaves, e.name)
					a.mu.Unlock()
				}
			}
		}
	}()

	if err := a.net.Apply(a.cfg); err != nil {
		c.t.Fatalf("apply %s: %v", name, err)
	}

	c.t.Cleanup(func() {
		a.net.Leave()
		cancel()
	})

	c.mu.Lock()
	c.agents = append(c.agents, a)
	c.mu.Unlock()

	return a
}

func (a *testAgent) members() []string {
	inst := a.net.cur.Load()
	if inst == nil {
		return nil
	}

	var names []string
	for _, m := range inst.list.Members() {
		names = append(names, m.Name)
	}
	slices.Sort(names)

	return names
}

func (a *testAgent) leaveEvents() []string {
	a.mu.Lock()
	defer a.mu.Unlock()

	return slices.Clone(a.leaves)
}

func (c *testCluster) converged(n int) bool {
	c.mu.Lock()
	agents := slices.Clone(c.agents)
	c.mu.Unlock()

	for _, a := range agents {
		if len(a.members()) != n {
			return false
		}
	}

	return true
}

// metaDelegate gives the NodeMeta of a test agent. Encrypt sets it, as
// protocol.Transport does.
type metaDelegate struct{ meta atomic.Value }

func (d *metaDelegate) NodeMeta(int) []byte {
	b, _ := d.meta.Load().([]byte)

	return b
}

func (*metaDelegate) NotifyMsg([]byte)                {}
func (*metaDelegate) GetBroadcasts(int, int) [][]byte { return nil }
func (*metaDelegate) LocalState(bool) []byte          { return nil }
func (*metaDelegate) MergeRemoteState([]byte, bool)   {}

func eventually(t *testing.T, within time.Duration, what string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(within)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("%s: not reached in %s", what, within)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func timingsConfig(rejoin time.Duration) Config {
	return Config{Timings: critical, RejoinInterval: rejoin}
}

func TestAgentsJoinTheStaticPeers(t *testing.T) {
	c := newTestCluster(t)
	for i := range 3 {
		c.start(fmt.Sprintf("node-%d", i), testLabel, timingsConfig(5*time.Second), nil, Deps{})
	}

	eventually(t, 10*time.Second, "all agents are members", func() bool { return c.converged(3) })
}

// A packet of another cluster is dropped.
func TestAgentOfAnotherClusterIsDropped(t *testing.T) {
	c := newTestCluster(t)
	ours := c.start("node-a", testLabel, timingsConfig(time.Hour), func() []domain.Peer { return nil }, Deps{})
	theirs := c.start("node-x", "another-cluster", timingsConfig(time.Hour), func() []domain.Peer { return []domain.Peer{peer("node-a")} }, Deps{})

	time.Sleep(time.Second)

	if got := ours.members(); !slices.Equal(got, []string{"node-a"}) {
		t.Errorf("members = %v, want only node-a: an agent with another label must stay out", got)
	}
	if got := theirs.members(); !slices.Equal(got, []string{"node-x"}) {
		t.Errorf("members = %v, want only node-x", got)
	}
}

// A packet with a foreign key is dropped on the main network.
func TestAgentWithAnotherKeyIsDropped(t *testing.T) {
	c := newTestCluster(t)
	ours := c.start("node-a", testLabel, timingsConfig(time.Hour), func() []domain.Peer { return nil }, Deps{})
	theirs := c.start("node-x", testLabel, timingsConfig(time.Hour), func() []domain.Peer { return []domain.Peer{peer("node-a")} },
		Deps{Encrypt: encrypt([]byte("dh-test-secret-0123456789abcdefX"))})

	time.Sleep(time.Second)

	if got := ours.members(); !slices.Equal(got, []string{"node-a"}) {
		t.Errorf("members = %v, want only node-a", got)
	}
	if got := theirs.members(); !slices.Equal(got, []string{"node-x"}) {
		t.Errorf("members = %v, want only node-x", got)
	}
}

// The own join has not succeeded yet.
func TestInstanceAcceptsAJoinBeforeItsOwnAndRejoinsEveryInterval(t *testing.T) {
	const rejoin = 5 * time.Second

	c := newTestCluster(t)

	// The static list of node-a has node-b, but node-b is not started yet.
	c.mu.Lock()
	c.ports["node-b"] = freePort(t)
	c.mu.Unlock()
	a := c.start("node-a", testLabel, timingsConfig(rejoin), func() []domain.Peer { return []domain.Peer{peer("node-b")} }, Deps{})

	// The own Join of node-a fails, the Join of node-s to node-a succeeds.
	c.start("node-s", testLabel, timingsConfig(time.Hour), func() []domain.Peer { return []domain.Peer{peer("node-a")} }, Deps{})
	eventually(t, 5*time.Second, "node-s joins node-a", func() bool { return slices.Contains(a.members(), "node-s") })

	// node-b starts and does not join by itself.
	c.mu.Lock()
	delete(c.ports, "node-b")
	c.mu.Unlock()
	time.Sleep(rejoin / 2)
	b := c.start("node-b", testLabel, timingsConfig(time.Hour), func() []domain.Peer { return nil }, Deps{})

	start := time.Now()
	eventually(t, rejoin+2*time.Second, "node-a joins node-b", func() bool { return slices.Contains(b.members(), "node-a") })
	t.Logf("node-a joined %s after node-b started", time.Since(start).Round(time.Millisecond))
}

func TestOnMetaGetsTheNodeMetaOfThePeer(t *testing.T) {
	c := newTestCluster(t)

	md := &metaDelegate{}
	md.meta.Store([]byte("v1"))

	var mu sync.Mutex
	metas := map[string]string{}

	a := c.start("node-a", testLabel, timingsConfig(time.Second), nil, Deps{Encrypt: func(cfg *hcml.Config) {
		encrypt(testKey)(cfg)
		cfg.Delegate = md
	}})
	c.start("node-b", testLabel, timingsConfig(time.Second), nil, Deps{OnMeta: func(name string, meta []byte) {
		mu.Lock()
		defer mu.Unlock()
		metas[name] = string(meta)
	}})

	metaOf := func(want string) func() bool {
		return func() bool {
			mu.Lock()
			defer mu.Unlock()
			return metas["node-a"] == want
		}
	}

	eventually(t, 10*time.Second, "node-b gets the NodeMeta of node-a", metaOf("v1"))

	md.meta.Store([]byte("v2"))
	if err := a.net.cur.Load().list.UpdateNode(time.Second); err != nil {
		t.Fatalf("UpdateNode: %v", err)
	}
	eventually(t, 5*time.Second, "node-b gets the new NodeMeta of node-a", metaOf("v2"))
}

// A timing change on all nodes recreates memberlist on all nodes.
func TestConcurrentRecreationDeclaresNoNodeDead(t *testing.T) {
	const agents = 5

	c := newTestCluster(t)
	for i := range agents {
		c.start(fmt.Sprintf("node-%d", i), testLabel, Config{Timings: normal, RejoinInterval: 5 * time.Second}, nil, Deps{})
	}
	eventually(t, 15*time.Second, "all agents are members", func() bool { return c.converged(agents) })

	var wg sync.WaitGroup
	for _, a := range c.agents {
		wg.Go(func() {
			time.Sleep(rand.N(2 * time.Second))

			cfg := a.cfg
			cfg.Timings = critical
			if err := a.net.Apply(cfg); err != nil {
				t.Errorf("recreate %s: %v", a.name, err)
			}
		})
	}
	wg.Wait()

	eventually(t, 10*time.Second, "membership is back", func() bool { return c.converged(agents) })
	// Keep watching for late suspicion timeouts.
	time.Sleep(3 * time.Second)

	for _, a := range c.agents {
		if got := a.leaveEvents(); len(got) != 0 {
			t.Errorf("%s saw dead or left members %v, want none", a.name, got)
		}
		if got := a.net.cur.Load().cfg.Timings; got != critical {
			t.Errorf("%s timings = %+v, want critical after the recreation", a.name, got)
		}
		if got := a.net.cur.Load().list.LocalNode(); got == nil {
			t.Errorf("%s has no local node", a.name)
		}
	}
}

// Two changes in a row during the recreation of memberlist, the network part.
func TestTwoQuickChangesApplyTheLast(t *testing.T) {
	c := newTestCluster(t)
	a := c.start("node-a", testLabel, Config{Timings: slow, RejoinInterval: time.Second}, nil, Deps{})
	c.start("node-b", testLabel, Config{Timings: slow, RejoinInterval: time.Second}, nil, Deps{})
	eventually(t, 10*time.Second, "both are members", func() bool { return c.converged(2) })

	for _, timings := range []Config{{Timings: normal}, {Timings: critical}} {
		cfg := a.cfg
		cfg.Timings = timings.Timings
		if err := a.net.Apply(cfg); err != nil {
			t.Fatalf("apply: %v", err)
		}
		time.Sleep(300 * time.Millisecond)
	}

	if got := a.net.cur.Load().cfg.Timings; got != critical {
		t.Errorf("applied timings = %+v, want the last ones (critical)", got)
	}
	eventually(t, 10*time.Second, "membership is back", func() bool { return c.converged(2) })
}

func TestApplyWithTheSameConfigKeepsTheInstance(t *testing.T) {
	c := newTestCluster(t)
	a := c.start("node-a", testLabel, timingsConfig(time.Second), nil, Deps{})
	before := a.net.cur.Load()

	if err := a.net.Apply(a.cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}

	if a.net.cur.Load() != before {
		t.Error("Apply with the same config recreated the instance")
	}
}

// Recreate with the same config makes a new instance with the same name, and
// the peers keep the node alive.
func TestRecreateWithTheSameConfigJoinsAgain(t *testing.T) {
	c := newTestCluster(t)
	a := c.start("node-a", testLabel, timingsConfig(time.Second), nil, Deps{})
	b := c.start("node-b", testLabel, timingsConfig(time.Second), nil, Deps{})
	eventually(t, 10*time.Second, "both are members", func() bool { return c.converged(2) })
	before := a.net.cur.Load()

	if err := a.net.Recreate(a.cfg); err != nil {
		t.Fatalf("recreate: %v", err)
	}

	if a.net.cur.Load() == before || a.net.cur.Load().cfg != a.cfg {
		t.Fatal("Recreate did not make a new instance with the same config")
	}

	eventually(t, 10*time.Second, "membership is back", func() bool { return c.converged(2) })

	if got := b.leaveEvents(); len(got) != 0 {
		t.Errorf("node-b saw dead or left members %v, want none", got)
	}
}

// The notification callback of memberlist: the consumers block, memberlist does not.
func TestBlockedConsumersDoNotBlockMemberlist(t *testing.T) {
	const agents = 3

	block := make(chan struct{})
	t.Cleanup(func() { close(block) })

	c := newTestCluster(t)
	for i := range agents {
		name := fmt.Sprintf("node-%d", i)
		c.start(name, testLabel, timingsConfig(time.Second), nil, Deps{
			OnJoin: func(string, netip.Addr) { <-block },
		})
	}

	eventually(t, 10*time.Second, "all agents are members", func() bool { return c.converged(agents) })

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 50 {
			_ = c.agents[1].members()
			time.Sleep(20 * time.Millisecond)
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Members() blocked while the consumers block")
	}

	if !c.converged(agents) {
		t.Error("a member was lost while the consumers block")
	}
}

// The network part of the graceful stop of the agent ('leaving').
func TestLeaveTellsThePeersAndStops(t *testing.T) {
	c := newTestCluster(t)
	a := c.start("node-a", testLabel, timingsConfig(time.Second), nil, Deps{})
	b := c.start("node-b", testLabel, timingsConfig(time.Second), nil, Deps{})
	eventually(t, 10*time.Second, "both are members", func() bool { return c.converged(2) })

	start := time.Now()
	a.net.Leave()
	if elapsed := time.Since(start); elapsed > leaveTimeout+stepTimeout {
		t.Errorf("Leave took %s", elapsed)
	}

	// The probe timeout of critical is far longer than this: only the leave message gives it.
	eventually(t, 500*time.Millisecond, "node-b sees node-a left", func() bool {
		return slices.Equal(b.members(), []string{"node-b"})
	})

	if a.net.cur.Load() != nil || a.net.Members() != 0 {
		t.Error("the instance runs after Leave")
	}

	a.net.Leave()
}

func TestOnJoinGetsTheMembers(t *testing.T) {
	c := newTestCluster(t)

	var mu sync.Mutex
	joined := map[string]netip.Addr{}
	c.start("node-a", testLabel, timingsConfig(time.Second), nil, Deps{OnJoin: func(name string, addr netip.Addr) {
		mu.Lock()
		defer mu.Unlock()
		joined[name] = addr
	}})

	c.start("node-b", testLabel, timingsConfig(time.Second), nil, Deps{})

	eventually(t, 10*time.Second, "OnJoin gets node-b", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return joined["node-b"] == loopback
	})
}

var (
	_ hcml.EventDelegate = (*delegate)(nil)
	_ hcml.Delegate      = (*metaDelegate)(nil)
)
