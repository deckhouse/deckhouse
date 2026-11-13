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
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	hcml "github.com/hashicorp/memberlist"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
	"fencing-agent/internal/protocol"
)

type e2eStatus struct{}

func (e2eStatus) SetCondition(domain.Reason, string) {}
func (e2eStatus) SetCPSource(domain.CPSource)        {}
func (e2eStatus) SetProfile(string)                  {}

type e2eMetrics struct{}

func (e2eMetrics) Decision(domain.FeedDecision)                  {}
func (e2eMetrics) Poll()                                         {}
func (e2eMetrics) PollAnswer(domain.Network, domain.AnswerClass) {}
func (e2eMetrics) ConfigInvalid(string, bool)                    {}

type e2eNode struct {
	name string
	addr netip.Addr
	tr   *protocol.Transport
	net  *Network
	mu   sync.Mutex
	meta map[string]domain.NodeMeta
}

// Network and protocol.Transport wired as in the agent, on 127.0.0.x with one port (Linux only).
func TestTwoAgentsExchangeMessagesAndMeta(t *testing.T) {
	if _, err := os.Stat("/proc"); err != nil {
		t.Skip("needs Linux loopback 127.0.0.0/8")
	}
	const port = 18500
	keyDir := filepath.Join(t.TempDir(), "k")
	_ = os.MkdirAll(keyDir, 0o700)
	_ = os.WriteFile(filepath.Join(keyDir, "primary"), []byte("dh-test-secret-0123456789abcdef0"), 0o600)

	nodes := []*e2eNode{
		{name: "node-a", addr: netip.MustParseAddr("127.0.0.2")},
		{name: "node-b", addr: netip.MustParseAddr("127.0.0.3")},
	}
	peersOf := func(self string) func() []domain.Peer {
		return func() []domain.Peer {
			var out []domain.Peer
			for _, n := range nodes {
				if n.name != self {
					out = append(out, domain.Peer{Name: n.name, Addr: n.addr})
				}
			}
			return out
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for _, n := range nodes {
		keys := protocol.NewKeyring(keyDir, 2*time.Minute, e2eStatus{}, e2eMetrics{}, log.NewNop())
		if err := keys.Wait(ctx); err != nil {
			t.Fatal(err)
		}
		n.meta = map[string]domain.NodeMeta{}
		n.tr = protocol.NewTransport(protocol.Config{NodeName: n.name, Network: domain.NetworkMain, BootID: "boot-" + n.name, Keys: keys}, log.NewNop())
		tr := n.tr
		nn := n
		n.net = New(Deps{
			Peers:  peersOf(n.name),
			OnJoin: func(string, netip.Addr) {},
			OnMeta: func(name string, meta []byte) {
				if meta != nil {
					m, _ := protocol.DecodeMeta(meta)
					nn.mu.Lock()
					nn.meta[name] = m
					nn.mu.Unlock()
				}
			},
			Encrypt: func(c *hcml.Config) {
				if err := tr.Configure(c); err != nil {
					t.Error(err)
				}
			},
			Attach: tr.Attach,
		}, log.NewNop())
		wg.Go(func() { _ = n.net.Run(ctx) })
		wg.Go(func() { _ = n.tr.Run(ctx) })
		if err := n.net.Apply(Config{NodeName: n.name, Label: "cluster-uuid", Addr: n.addr, Port: port, Timings: critical, RejoinInterval: time.Second}); err != nil {
			t.Fatal(err)
		}
	}
	a, b := nodes[0], nodes[1]
	deadline := time.Now().Add(10 * time.Second)
	for a.net.Members() < 1 || b.net.Members() < 1 {
		if time.Now().After(deadline) {
			t.Fatal("no join")
		}
		time.Sleep(50 * time.Millisecond)
	}
	a.tr.UpdateMeta(func(m *domain.NodeMeta) { m.Armed = true })

	pb := domain.Peer{Name: "node-b", Addr: b.addr}
	pa := domain.Peer{Name: "node-a", Addr: a.addr}
	if err := a.tr.Send(pb, domain.Question{PollID: 7, LastReadyAgo: time.Second}); err != nil {
		t.Fatal(err)
	}
	var q domain.Question
	select {
	case m := <-b.tr.Received():
		q = m.(domain.Question)
		t.Logf("node-b got %#v", q)
	case <-time.After(3 * time.Second):
		t.Fatal("question did not reach node-b")
	}
	if err := b.tr.Send(pa, domain.Answer{To: q.From, PollID: q.PollID, Epoch: q.Epoch, Network: q.Network, Verdict: domain.VerdictSee}); err != nil {
		t.Fatal(err)
	}
	select {
	case m := <-a.tr.Received():
		t.Logf("node-a got %#v", m)
	case <-time.After(3 * time.Second):
		t.Fatal("answer did not reach node-a")
	}
	_ = b.tr.Send(pa, domain.Leaving{})
	select {
	case m := <-a.tr.Received():
		if _, ok := m.(domain.Leaving); !ok {
			t.Fatalf("got %T", m)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("leaving did not reach node-a")
	}
	deadline = time.Now().Add(5 * time.Second)
	for {
		b.mu.Lock()
		m := b.meta["node-a"]
		b.mu.Unlock()
		if m.Armed && m.BootID == "boot-node-a" {
			t.Logf("node-b sees NodeMeta of node-a: %+v", m)
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("NodeMeta not seen: %+v", m)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err := a.tr.PublishMeta(func(m *domain.NodeMeta) { m.Leaving = true }); err != nil {
		t.Logf("PublishMeta: %v", err)
	}
	b.mu.Lock()
	lv := b.meta["node-a"].Leaving
	b.mu.Unlock()
	t.Logf("node-b sees leaving right after PublishMeta: %v", lv)
	if !lv {
		t.Error("leaving not seen right after PublishMeta")
	}
	for _, n := range nodes {
		n.net.Leave()
	}
	cancel()
	wg.Wait()
}
