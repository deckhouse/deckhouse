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

package protocol

import (
	"bytes"
	"context"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	hcml "github.com/hashicorp/memberlist"
	"google.golang.org/protobuf/encoding/protowire"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

const testLabel = "cluster-uuid"

var peerAddr = netip.MustParseAddr("127.0.0.1")

// testAgent is the protocol part of one agent on a memberlist mock network.
type testAgent struct {
	name   string
	dir    string
	keys   *Keyring
	tr     *Transport
	list   *hcml.Memberlist
	leaves atomic.Int32
	// address is read once at start: UpdateNode changes the local node of memberlist.
	address string

	// metas are the copies of the NodeMeta of the members. memberlist calls
	// the events with its node lock, so the copy is safe only there.
	mu    sync.Mutex
	metas map[string][]byte
}

func (a *testAgent) NotifyJoin(n *hcml.Node)   { a.observe(n.Name, n.Meta) }
func (a *testAgent) NotifyUpdate(n *hcml.Node) { a.observe(n.Name, n.Meta) }

func (a *testAgent) NotifyLeave(n *hcml.Node) {
	a.leaves.Add(1)
	a.observe(n.Name, nil)
}

func (a *testAgent) observe(name string, meta []byte) {
	meta = bytes.Clone(meta)

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.metas == nil {
		a.metas = make(map[string][]byte)
	}
	a.metas[name] = meta
}

func (a *testAgent) peer() domain.Peer { return domain.Peer{Name: a.name, Addr: peerAddr} }

func (a *testAgent) addr() string { return a.address }

// newTestAgent creates the keyring and the transport. It does not wait for the key.
func newTestAgent(t *testing.T, name string, keys ...[]byte) *testAgent {
	t.Helper()

	k, dir, _, _ := newTestKeyring(t, keys...)
	tr := NewTransport(Config{NodeName: name, Network: domain.NetworkMain, BootID: "boot-" + name, Keys: k}, log.NewNop())

	return &testAgent{name: name, dir: dir, keys: k, tr: tr}
}

// start creates memberlist like the agent does: Configure, Create, Attach.
func (a *testAgent) start(t *testing.T, network *testNetwork) {
	t.Helper()

	cfg := testConfig(network, a.name)
	cfg.Events = a
	if err := a.tr.Configure(cfg); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	list, err := hcml.Create(cfg)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	a.tr.Attach(list)
	a.list = list
	a.address = list.LocalNode().Address()
}

func (a *testAgent) stop() {
	a.tr.Attach(nil)
	_ = a.list.Shutdown()
}

func testConfig(network *testNetwork, name string) *hcml.Config {
	cfg := hcml.DefaultLANConfig()
	cfg.Name = name
	cfg.Transport = network.transport(name)
	cfg.Label = testLabel
	cfg.LogOutput = io.Discard

	return cfg
}

// startAgents starts agents with the keys, joins them and runs their transports.
func startAgents(t *testing.T, ctx context.Context, wg *sync.WaitGroup, network *testNetwork, names []string, keys ...[]byte) []*testAgent {
	t.Helper()

	agents := make([]*testAgent, 0, len(names))
	for _, name := range names {
		a := newTestAgent(t, name, keys...)
		if err := a.keys.Wait(ctx); err != nil {
			t.Fatal(err)
		}
		a.start(t, network)
		if len(agents) > 0 {
			if _, err := a.list.Join([]string{agents[0].addr()}); err != nil {
				t.Fatalf("Join: %v", err)
			}
		}
		wg.Go(func() { _ = a.tr.Run(ctx) })
		agents = append(agents, a)
	}

	return agents
}

func stopAgents(cancel context.CancelFunc, wg *sync.WaitGroup, agents []*testAgent) {
	cancel()
	wg.Wait()
	for _, a := range agents {
		a.stop()
	}
	afterShutdown()
}

// afterShutdown lets the goroutines of stopped memberlist instances end:
// some of them wait for a timeout after Shutdown, and synctest does not move
// the time after the test function returns.
func afterShutdown() {
	time.Sleep(time.Minute)
}

// receivedNow returns the messages that the transport has now.
func receivedNow(tr *Transport) []domain.Message {
	synctest.Wait()

	var msgs []domain.Message
	for {
		select {
		case m := <-tr.Received():
			msgs = append(msgs, m)
		default:
			return msgs
		}
	}
}

// peerMeta returns the NodeMeta of the member name that the agent a knows.
func peerMeta(t *testing.T, a *testAgent, name string) domain.NodeMeta {
	t.Helper()

	a.mu.Lock()
	meta, ok := a.metas[name]
	a.mu.Unlock()

	if !ok || meta == nil {
		t.Fatalf("%s is not a member", name)
	}

	m, err := DecodeMeta(meta)
	if err != nil {
		t.Fatal(err)
	}

	return m
}

// A question and an answer carry all fields through encrypted memberlist,
// and the transport matches the answer to the question.
func TestTransportQuestionAndAnswer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		var wg sync.WaitGroup
		agents := startAgents(t, ctx, &wg, newTestNetwork(), []string{"node-a", "node-b", "node-c"}, key1)
		defer stopAgents(cancel, &wg, agents)
		a, b, c := agents[0], agents[1], agents[2]

		if err := a.tr.Send(b.peer(), domain.Question{PollID: 77, LastReadyAgo: 2 * time.Second}); err != nil {
			t.Fatalf("Send: %v", err)
		}

		got := receivedNow(b.tr)
		if len(got) != 1 {
			t.Fatalf("node-b received %d messages, want 1", len(got))
		}
		q := got[0].(domain.Question)
		want := domain.Question{
			Header: domain.Header{Version: domain.ProtocolVersion, From: "node-a"}, PollID: 77,
			Network: domain.NetworkMain, Epoch: 1, To: "node-b", Addr: peerAddr, LastReadyAgo: 2 * time.Second,
		}
		if q != want {
			t.Fatalf("question:\n got %+v\nwant %+v", q, want)
		}

		reply := domain.Answer{To: q.From, PollID: q.PollID, Epoch: q.Epoch, Verdict: domain.VerdictSee, LastReadyAgo: time.Second}
		if err := b.tr.Send(a.peer(), reply); err != nil {
			t.Fatal(err)
		}
		// node-c was not asked: its answer does not count.
		if err := c.tr.Send(a.peer(), reply); err != nil {
			t.Fatal(err)
		}
		// An answer to another poll does not count.
		stale := reply
		stale.PollID = 76
		if err := b.tr.Send(a.peer(), stale); err != nil {
			t.Fatal(err)
		}

		got = receivedNow(a.tr)
		if len(got) != 1 {
			t.Fatalf("node-a received %d messages, want 1: %+v", len(got), got)
		}
		ans := got[0].(domain.Answer)
		if ans.From != "node-b" || ans.To != "node-a" || ans.PollID != 77 || ans.Epoch != 1 ||
			ans.Network != domain.NetworkMain || ans.Verdict != domain.VerdictSee || ans.LastReadyAgo != time.Second ||
			ans.Version != domain.ProtocolVersion || ans.UnknownFields {
			t.Errorf("answer = %+v", ans)
		}
	})
}

// A message without encryption and a packet with a foreign key are not accepted.
func TestTransportDropsPlainAndForeignMessages(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		var wg sync.WaitGroup
		network := newTestNetwork()
		agents := startAgents(t, ctx, &wg, network, []string{"node-a"}, key1)
		defer stopAgents(cancel, &wg, agents)
		a := agents[0]

		msg, err := Encode(domain.Header{Version: domain.ProtocolVersion, From: "intruder"}, domain.SeeCP{})
		if err != nil {
			t.Fatal(err)
		}

		plain, err := hcml.Create(testConfig(network, "plain"))
		if err != nil {
			t.Fatal(err)
		}

		foreignCfg := testConfig(network, "foreign")
		if foreignCfg.Keyring, err = hcml.NewKeyring(nil, key3); err != nil {
			t.Fatal(err)
		}
		foreign, err := hcml.Create(foreignCfg)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = plain.Shutdown()
			_ = foreign.Shutdown()
			afterShutdown()
		}()

		to := &hcml.Node{Name: a.name, Addr: peerAddr.AsSlice()}
		for _, sender := range []*hcml.Memberlist{plain, foreign} {
			if err := sender.SendBestEffort(to, msg); err != nil {
				t.Fatal(err)
			}
			if _, err := sender.Join([]string{a.addr()}); err == nil {
				t.Error("a node without the cluster key joined")
			}
		}

		if got := receivedNow(a.tr); len(got) != 0 {
			t.Errorf("node-a received %+v", got)
		}
		if a.list.NumMembers() != 1 {
			t.Errorf("node-a has %d members, want 1", a.list.NumMembers())
		}
	})
}

// An agent of the next protocol version answers with an unknown
// verdict and with an unknown field, and sends a message of an unknown type.
// The answers come as answers that count as support (VerdictUnknown or
// UnknownFields); the message of the unknown type is ignored and the
// transport continues to work.
func TestTransportMixedVersions(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		var wg sync.WaitGroup
		agents := startAgents(t, ctx, &wg, newTestNetwork(), []string{"node-a", "node-next"}, key1)
		defer stopAgents(cancel, &wg, agents)
		a, next := agents[0], agents[1]

		if err := a.tr.Send(next.peer(), domain.Question{PollID: 5}); err != nil {
			t.Fatal(err)
		}
		q := receivedNow(next.tr)[0].(domain.Question)

		header := domain.Header{Version: domain.ProtocolVersion + 1, From: next.name}
		answerBody := func(verdict uint64, extra []byte) []byte {
			b := appendString(nil, 1, q.From)
			b = appendFixed64(b, 2, q.PollID)
			b = appendVarint(b, 3, q.Epoch)
			b = appendString(b, 4, string(q.Network))
			b = appendVarint(b, 5, verdict)

			return append(b, extra...)
		}
		raw := func(typ msgType, body []byte) []byte {
			b := appendVarint(nil, fieldType, uint64(typ))
			b = appendVarint(b, fieldVersion, uint64(header.Version))
			b = appendString(b, fieldFrom, header.From)

			return appendBytes(b, fieldBody, body)
		}
		newField := protowire.AppendVarint(protowire.AppendTag(nil, 7, protowire.VarintType), 1)

		to := &hcml.Node{Name: a.name, Addr: peerAddr.AsSlice()}
		for _, b := range [][]byte{
			raw(msgType(50), []byte("new message")),
			raw(typeAnswer, answerBody(9, nil)),
			raw(typeAnswer, answerBody(uint64(domain.VerdictSee), newField)),
		} {
			if err := next.list.SendBestEffort(to, b); err != nil {
				t.Fatal(err)
			}
		}

		got := receivedNow(a.tr)
		if len(got) != 2 {
			t.Fatalf("node-a received %d messages, want 2 answers: %+v", len(got), got)
		}
		// memberlist does not keep the order of the messages.
		var unknownVerdict, unknownField bool
		for _, m := range got {
			ans := m.(domain.Answer)
			unknownVerdict = unknownVerdict || (ans.Verdict == domain.VerdictUnknown && !ans.UnknownFields)
			unknownField = unknownField || (ans.Verdict == domain.VerdictSee && ans.UnknownFields)
		}
		if !unknownVerdict || !unknownField {
			t.Errorf("answers = %+v, want one with VerdictUnknown and one with UnknownFields", got)
		}

		// The transport continues to work after the message of the unknown type.
		if err := next.tr.Send(a.peer(), domain.SeeCP{}); err != nil {
			t.Fatal(err)
		}
		if got := receivedNow(a.tr); len(got) != 1 {
			t.Errorf("node-a received %d messages after the unknown type, want 1", len(got))
		}
	})
}

// Messages of the previous memberlist instance and answers to its questions
// do not count.
func TestTransportNewEpoch(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := newTestAgent(t, "node-a", key1)
		if err := a.keys.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}

		network := newTestNetwork()
		oldCfg := testConfig(network, a.name)
		if err := a.tr.Configure(oldCfg); err != nil {
			t.Fatal(err)
		}
		a.tr.polls.ask(domain.Question{PollID: 1, Network: domain.NetworkMain, Epoch: 1, To: "node-b"})

		if err := a.tr.Configure(testConfig(network, a.name)); err != nil {
			t.Fatal(err)
		}

		late, err := Encode(domain.Header{From: "node-b"},
			domain.Answer{To: a.name, PollID: 1, Epoch: 1, Network: domain.NetworkMain, Verdict: domain.VerdictSee})
		if err != nil {
			t.Fatal(err)
		}
		seeCP, err := Encode(domain.Header{From: "node-b"}, domain.SeeCP{})
		if err != nil {
			t.Fatal(err)
		}

		oldCfg.Delegate.NotifyMsg(seeCP)
		a.tr.receive(2, late)
		if got := receivedNow(a.tr); len(got) != 0 {
			t.Errorf("received %+v from the previous epoch", got)
		}
	})
}

// A node in maintenance (the maintenance field): the agent publishes all
// fields for the writers, and keyID matches the encryption key.
func TestTransportPublishesMeta(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		var wg sync.WaitGroup
		agents := startAgents(t, ctx, &wg, newTestNetwork(), []string{"node-a", "node-b"}, key1)
		defer stopAgents(cancel, &wg, agents)
		a, b := agents[0], agents[1]

		a.tr.UpdateMeta(func(m *domain.NodeMeta) {
			m.NodeUID = "uid-a"
			m.Armed = true
			m.Maintenance = true
		})
		time.Sleep(5 * time.Second)

		want := domain.NodeMeta{
			BootID: "boot-node-a", Armed: true, NodeUID: "uid-a",
			KeyID: keyID(key1), Maintenance: true, ProtocolVersion: domain.ProtocolVersion,
		}
		if got := peerMeta(t, b, a.name); got != want {
			t.Fatalf("node-b sees the NodeMeta of node-a\n got %+v\nwant %+v", got, want)
		}

		a.tr.UpdateMeta(func(m *domain.NodeMeta) { m.Maintenance, m.Armed, m.Leaving = false, false, true })
		time.Sleep(5 * time.Second)

		want.Maintenance, want.Armed, want.Leaving = false, false, true
		if got := peerMeta(t, b, a.name); got != want {
			t.Errorf("node-b sees the NodeMeta of node-a\n got %+v\nwant %+v", got, want)
		}
	})
}

// The graceful stop of the agent: the peers get leaving in the NodeMeta before
// Dead. The stop is the same as in the agent: PublishMeta, then Attach(nil),
// Leave and Shutdown of memberlist.
func TestLeavingMetaReachesThePeerBeforeLeave(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		var wg sync.WaitGroup
		agents := startAgents(t, ctx, &wg, newTestNetwork(), []string{"node-a", "node-b"}, key1)
		a, b := agents[0], agents[1]
		synctest.Wait()

		// Disarm publishes the NodeMeta in the background just before.
		a.tr.UpdateMeta(func(m *domain.NodeMeta) { m.Armed = false })
		if err := a.tr.PublishMeta(func(m *domain.NodeMeta) { m.Leaving = true }); err != nil {
			t.Fatalf("PublishMeta: %v", err)
		}
		synctest.Wait()

		if !peerMeta(t, b, a.name).Leaving {
			t.Fatal("node-b does not see leaving of node-a before Leave")
		}

		a.tr.Attach(nil)
		if err := a.list.Leave(3 * time.Second); err != nil {
			t.Fatalf("Leave: %v", err)
		}
		_ = a.list.Shutdown()
		synctest.Wait()

		if b.leaves.Load() != 1 {
			t.Errorf("node-b saw %d leaves of node-a, want 1", b.leaves.Load())
		}

		if err := a.tr.PublishMeta(func(m *domain.NodeMeta) {}); err == nil {
			t.Error("PublishMeta after Attach(nil) = nil, want an error")
		}

		stopAgents(cancel, &wg, []*testAgent{b})
	})
}

func TestTransportRejectsLargeMeta(t *testing.T) {
	a := newTestAgent(t, "node-a", key1)

	before := (&delegate{t: a.tr}).NodeMeta(domain.MetaMaxSize)
	a.tr.UpdateMeta(func(m *domain.NodeMeta) { m.NodeUID = string(make([]byte, domain.MetaMaxSize)) })

	if after := (&delegate{t: a.tr}).NodeMeta(domain.MetaMaxSize); !bytes.Equal(after, before) {
		t.Error("a NodeMeta larger than MetaMaxSize is set")
	}
}

// The publication does not block the poll: the agent changes the NodeMeta
// while a broadcast of the NodeMeta is slow.
func TestPublishDoesNotBlock(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := newTestAgent(t, "node-a", key1)

		var calls atomic.Int32
		slowUpdate := func() error {
			calls.Add(1)
			time.Sleep(10 * time.Second)
			return nil
		}

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		go a.tr.publishLoop(ctx, slowUpdate)

		start := time.Now()
		for i := range 10 {
			a.tr.UpdateMeta(func(m *domain.NodeMeta) { m.Armed = i%2 == 0 })
			synctest.Wait()
		}
		if elapsed := time.Since(start); elapsed != 0 {
			t.Errorf("UpdateMeta waited %v for the slow broadcast", elapsed)
		}

		time.Sleep(time.Minute)
		if got := calls.Load(); got != 2 {
			t.Errorf("updates = %d, want 2: changes during a slow update give one more update", got)
		}
	})
}

// A planned rotation by the administrator and the removal of the old key:
// the new key is first an additional key, the agents switch after 2 min, the
// old key is removed not earlier than 2 min after the switch, and no agent
// loses a peer.
func TestTransportKeyRotation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		var wg sync.WaitGroup
		agents := startAgents(t, ctx, &wg, newTestNetwork(), []string{"node-a", "node-b", "node-c"}, key1)
		defer stopAgents(cancel, &wg, agents)

		check := func(step string, primary []byte, ringSize int) {
			t.Helper()
			for _, a := range agents {
				keys := a.keys.Ring().GetKeys()
				if !bytes.Equal(keys[0], primary) || len(keys) != ringSize {
					t.Errorf("%s: %s keyring has primary %q and %d keys, want %q and %d", step, a.name, keys[0], len(keys), primary, ringSize)
				}
				if n := a.list.NumMembers(); n != len(agents) || a.leaves.Load() != 0 {
					t.Errorf("%s: %s has %d members and %d leaves", step, a.name, n, a.leaves.Load())
				}
			}
		}

		// The administrator writes the new key first and keeps the old key second.
		for _, a := range agents {
			writeKeys(t, a.dir, key2, key1)
		}
		time.Sleep(testSwitchDelay - 2*time.Second)
		check("before the timeout", key1, 2)

		time.Sleep(4 * time.Second)
		check("after the timeout", key2, 2)

		// The NodeMeta broadcast takes some gossip rounds. A dropped broadcast
		// comes with the next push/pull, not later than its interval.
		seen := func() bool {
			for _, a := range agents {
				for _, peer := range agents {
					if peer != a && peerMeta(t, a, peer.name).KeyID != keyID(key2) {
						return false
					}
				}
			}
			return true
		}
		for deadline := time.Now().Add(hcml.DefaultLANConfig().PushPullInterval + 5*time.Second); !seen(); time.Sleep(time.Second) {
			if time.Now().After(deadline) {
				t.Fatal("the agents do not see the new keyID of each other")
			}
		}

		// The old key leaves the Secret. The agents remove it after the delay.
		for _, a := range agents {
			writeKeys(t, a.dir, key2)
		}
		removeAt := time.Now().Add(testSwitchDelay)
		time.Sleep(time.Until(removeAt.Add(-3 * time.Second)))
		check("before the removal", key2, 2)

		time.Sleep(5 * time.Second)
		check("after the removal", key2, 1)

		// The agents still talk.
		if err := agents[0].tr.Send(agents[1].peer(), domain.SeeCP{}); err != nil {
			t.Fatal(err)
		}
		if got := receivedNow(agents[1].tr); len(got) != 1 {
			t.Errorf("node-b received %d messages, want 1", len(got))
		}
	})
}

// A peer got the new key later: node-c sees the new key
// 1 min after node-a and node-b. node-a and node-b switch 2 min after they
// added the key, and node-c decrypts their messages with the new additional key.
func TestTransportPeerGetsKeyLater(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		var wg sync.WaitGroup
		agents := startAgents(t, ctx, &wg, newTestNetwork(), []string{"node-a", "node-b", "node-c"}, key1)
		defer stopAgents(cancel, &wg, agents)
		a, c := agents[0], agents[2]

		check := func(step string, primaries ...[]byte) {
			t.Helper()
			for i, ag := range agents {
				if got := ag.keys.Ring().GetKeys()[0]; !bytes.Equal(got, primaries[i]) {
					t.Errorf("%s: %s has primary %q, want %q", step, ag.name, got, primaries[i])
				}
				if n := ag.list.NumMembers(); n != len(agents) || ag.leaves.Load() != 0 {
					t.Errorf("%s: %s has %d members and %d leaves", step, ag.name, n, ag.leaves.Load())
				}
			}
		}
		talk := func(step string, from, to *testAgent) {
			t.Helper()
			if err := from.tr.Send(to.peer(), domain.SeeCP{}); err != nil {
				t.Fatalf("%s: %v", step, err)
			}
			if got := receivedNow(to.tr); len(got) != 1 {
				t.Errorf("%s: %s received %d messages from %s, want 1", step, to.name, len(got), from.name)
			}
		}

		writeKeys(t, agents[0].dir, key2, key1)
		writeKeys(t, agents[1].dir, key2, key1)
		time.Sleep(time.Minute)
		writeKeys(t, c.dir, key2, key1)

		time.Sleep(time.Minute + 2*time.Second)
		check("node-a and node-b switched", key2, key2, key1)
		talk("node-a and node-b switched", a, c)
		talk("node-a and node-b switched", c, a)

		time.Sleep(time.Minute)
		check("node-c switched", key2, key2, key2)
		talk("node-c switched", c, a)
	})
}

// The correct key appeared in the Secret: the agent waits without
// memberlist while the key is not valid, and joins without a restart after a
// valid key appears.
func TestTransportStartAfterInvalidKey(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		var wg sync.WaitGroup
		network := newTestNetwork()
		agents := startAgents(t, ctx, &wg, network, []string{"node-a"}, key1)

		b := newTestAgent(t, "node-b", badKey)
		waited := make(chan error, 1)
		go func() { waited <- b.keys.Wait(ctx) }()

		time.Sleep(10 * time.Second)
		if err := b.tr.Configure(testConfig(network, b.name)); err == nil {
			t.Fatal("Configure accepts a keyring without a valid key")
		}

		writeKeys(t, b.dir, key1)
		if err := <-waited; err != nil {
			t.Fatal(err)
		}
		b.start(t, network)
		agents = append(agents, b)
		defer stopAgents(cancel, &wg, agents)

		if _, err := b.list.Join([]string{agents[0].addr()}); err != nil {
			t.Fatalf("Join: %v", err)
		}
		time.Sleep(time.Second)
		if n := agents[0].list.NumMembers(); n != 2 {
			t.Errorf("node-a has %d members, want 2", n)
		}
	})
}

func TestReadBootID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "boot_id")
	if err := os.WriteFile(path, []byte("6f1c2c4e-2b0e-4d5c-9a56-0d1e7e3c5b11\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ReadBootID(path)
	if err != nil || got != "6f1c2c4e-2b0e-4d5c-9a56-0d1e7e3c5b11" {
		t.Errorf("ReadBootID = %q, %v", got, err)
	}
}
