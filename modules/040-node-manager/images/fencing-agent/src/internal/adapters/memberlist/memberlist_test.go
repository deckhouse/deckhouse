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
	"net/netip"
	"strings"
	"testing"
	"time"

	hcml "github.com/hashicorp/memberlist"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
)

// testLabel stands for the cluster UUID.
const testLabel = "f49dd1c3-a63a-4565-a06c-625e35587eab"

// testKey is a 32-byte AES-256 key from the allowlist of test secrets.
var testKey = []byte("dh-test-secret-0123456789abcdef0")

func encrypt(key []byte) func(*hcml.Config) {
	return func(c *hcml.Config) {
		keyring, err := hcml.NewKeyring(nil, key)
		if err != nil {
			panic(err)
		}
		c.Keyring = keyring
	}
}

func d(v time.Duration) metav1.Duration { return metav1.Duration{Duration: v} }

// The memberlist timings of the main network for each profile.
var (
	critical = v1alpha1.FencingSLAProfileMemberlist{
		ProbeInterval: d(200 * time.Millisecond), ProbeTimeout: d(80 * time.Millisecond),
		SuspicionMult: 3, SuspicionMaxTimeoutMult: 4,
		GossipInterval: d(200 * time.Millisecond), GossipNodes: 3, RetransmitMult: 4,
		IndirectChecks: 3, AwarenessMaxMultiplier: 8,
		DeadNodeReclaimTime: d(time.Second), GossipToTheDeadTime: d(30 * time.Second),
	}
	normal = v1alpha1.FencingSLAProfileMemberlist{
		ProbeInterval: d(500 * time.Millisecond), ProbeTimeout: d(200 * time.Millisecond),
		SuspicionMult: 4, SuspicionMaxTimeoutMult: 6,
		GossipInterval: d(300 * time.Millisecond), GossipNodes: 3, RetransmitMult: 4,
		IndirectChecks: 3, AwarenessMaxMultiplier: 8,
		DeadNodeReclaimTime: d(time.Second), GossipToTheDeadTime: d(30 * time.Second),
	}
	slow = v1alpha1.FencingSLAProfileMemberlist{
		ProbeInterval: d(time.Second), ProbeTimeout: d(400 * time.Millisecond),
		SuspicionMult: 4, SuspicionMaxTimeoutMult: 6,
		GossipInterval: d(500 * time.Millisecond), GossipNodes: 3, RetransmitMult: 4,
		IndirectChecks: 3, AwarenessMaxMultiplier: 8,
		DeadNodeReclaimTime: d(time.Second), GossipToTheDeadTime: d(30 * time.Second),
	}
)

func testConfig() Config {
	return Config{
		NodeName:       "node-a",
		Label:          testLabel,
		Addr:           netip.MustParseAddr("10.0.0.5"),
		Port:           Port,
		Timings:        critical,
		RejoinInterval: 5 * time.Second,
	}
}

// The timings of the profiles critical, normal and slow.
func TestBuildConfigAppliesTheProfileTimings(t *testing.T) {
	for name, timings := range map[string]v1alpha1.FencingSLAProfileMemberlist{"critical": critical, "normal": normal, "slow": slow} {
		t.Run(name, func(t *testing.T) {
			cfg := testConfig()
			cfg.Timings = timings

			c, err := New(Deps{Encrypt: encrypt(testKey)}, log.NewNop()).buildConfig(cfg)
			if err != nil {
				t.Fatalf("buildConfig: %v", err)
			}

			got := v1alpha1.FencingSLAProfileMemberlist{
				ProbeInterval: d(c.ProbeInterval), ProbeTimeout: d(c.ProbeTimeout),
				SuspicionMult: int32(c.SuspicionMult), SuspicionMaxTimeoutMult: int32(c.SuspicionMaxTimeoutMult),
				GossipInterval: d(c.GossipInterval), GossipNodes: int32(c.GossipNodes), RetransmitMult: int32(c.RetransmitMult),
				IndirectChecks: int32(c.IndirectChecks), AwarenessMaxMultiplier: int32(c.AwarenessMaxMultiplier),
				DeadNodeReclaimTime: d(c.DeadNodeReclaimTime), GossipToTheDeadTime: d(c.GossipToTheDeadTime),
			}
			if got != timings {
				t.Errorf("memberlist timings = %+v, want %+v", got, timings)
			}
		})
	}
}

// The receive settings of the main network. The sockets of the main network bind to the node address.
func TestBuildConfigReceivesOnTheNodeAddress(t *testing.T) {
	n := New(Deps{Encrypt: encrypt(testKey)}, log.NewNop())

	c, err := n.buildConfig(testConfig())
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}

	if c.BindAddr != "10.0.0.5" || c.AdvertiseAddr != "10.0.0.5" || c.BindPort != 8500 || c.AdvertisePort != 8500 {
		t.Errorf("bind %s:%d advertise %s:%d, want 10.0.0.5:8500 for both", c.BindAddr, c.BindPort, c.AdvertiseAddr, c.AdvertisePort)
	}
	if c.Name != "node-a" || c.Label != testLabel {
		t.Errorf("name %q label %q, want node-a and the cluster UUID", c.Name, c.Label)
	}
	if c.EnableCompression {
		t.Error("compression is on, want off")
	}
	if c.HandoffQueueDepth != 8192 {
		t.Errorf("HandoffQueueDepth = %d, want 8192", c.HandoffQueueDepth)
	}
	if c.Events != hcml.EventDelegate(n.delegate) {
		t.Error("the asynchronous event delegate is not set")
	}
}

func TestBuildConfigRequiresEncryption(t *testing.T) {
	tests := map[string]func(*hcml.Config){
		"no Encrypt": nil,
		"no keyring": func(*hcml.Config) {},
		"incoming not verified": func(c *hcml.Config) {
			encrypt(testKey)(c)
			c.GossipVerifyIncoming = false
		},
		"outgoing not verified": func(c *hcml.Config) {
			encrypt(testKey)(c)
			c.GossipVerifyOutgoing = false
		},
	}

	for name, enc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := New(Deps{Encrypt: enc}, log.NewNop()).buildConfig(testConfig())
			if err == nil || !strings.Contains(err.Error(), "encryption") {
				t.Errorf("buildConfig() error = %v, want an encryption error", err)
			}
		})
	}
}

func TestEncryptCannotRemoveTheEventDelegate(t *testing.T) {
	n := New(Deps{Encrypt: func(c *hcml.Config) {
		encrypt(testKey)(c)
		c.Events = nil
	}}, log.NewNop())

	c, err := n.buildConfig(testConfig())
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if c.Events == nil {
		t.Error("Encrypt removed the event delegate")
	}
}

type otherDelegate struct{ hcml.Delegate }

func TestEncryptCanSetTheDelegate(t *testing.T) {
	own := otherDelegate{}
	n := New(Deps{Encrypt: func(c *hcml.Config) {
		encrypt(testKey)(c)
		c.Delegate = own
	}}, log.NewNop())

	c, err := n.buildConfig(testConfig())
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if c.Delegate != own || c.Events != n.delegate {
		t.Error("the network replaced the delegate of Encrypt")
	}
}

func TestApplyRejectsAnInvalidConfig(t *testing.T) {
	tests := map[string]func(*Config){
		"no node name":       func(c *Config) { c.NodeName = "" },
		"no label":           func(c *Config) { c.Label = "" },
		"IPv6 address":       func(c *Config) { c.Addr = netip.MustParseAddr("fd00::5") },
		"no address":         func(c *Config) { c.Addr = netip.Addr{} },
		"zero probe":         func(c *Config) { c.Timings.ProbeInterval = d(0) },
		"zero rejoin period": func(c *Config) { c.RejoinInterval = 0 },
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := testConfig()
			mutate(&cfg)

			n := New(Deps{Encrypt: encrypt(testKey)}, log.NewNop())
			if err := n.Apply(cfg); err == nil {
				t.Error("Apply() = nil, want an error")
			}
			if n.cur.Load() != nil {
				t.Error("Apply() created an instance")
			}
		})
	}
}

// The processing of a message does not delay the receive.
func TestDelegateDoesNotBlockWhenNobodyReads(t *testing.T) {
	del := newDelegate()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range eventBuffer + 10 {
			del.NotifyJoin(&hcml.Node{Name: "node-b"})
			del.NotifyLeave(&hcml.Node{Name: "node-b"})
			del.NotifyUpdate(&hcml.Node{Name: "node-b"})
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("a delegate callback blocked")
	}

	if got := del.droppedEvents.Load(); got != 3*(eventBuffer+10)-eventBuffer {
		t.Errorf("dropped events = %d, want %d", got, 3*(eventBuffer+10)-eventBuffer)
	}
}

func TestSplitLevel(t *testing.T) {
	tests := []struct {
		line      string
		wantLevel string
		wantMsg   string
	}{
		{"[WARN] memberlist: probe failed", "WARN", "memberlist: probe failed"},
		{"[ERR] memberlist: bind error", "ERR", "memberlist: bind error"},
		{"memberlist: no level prefix", "", "memberlist: no level prefix"},
		{"[unterminated", "", "[unterminated"},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			level, msg := splitLevel(tt.line)
			if level != tt.wantLevel || msg != tt.wantMsg {
				t.Errorf("got (%q, %q), want (%q, %q)", level, msg, tt.wantLevel, tt.wantMsg)
			}
		})
	}
}

func TestOnMetaGetsACopyAndNilAtLeave(t *testing.T) {
	type call struct {
		name string
		meta []byte
	}

	var calls []call

	n := New(Deps{OnMeta: func(name string, meta []byte) { calls = append(calls, call{name, meta}) }}, log.NewNop())

	node := &hcml.Node{Name: "worker-2", Meta: []byte("meta-1")}
	n.delegate.NotifyJoin(node)
	node.Meta[0] = 'X'
	n.delegate.NotifyLeave(node)

	for range 2 {
		n.handle(<-n.delegate.events)
	}

	if len(calls) != 2 || string(calls[0].meta) != "meta-1" || calls[1].meta != nil {
		t.Fatalf("calls = %q, want a copy of meta-1 at join and nil at leave", calls)
	}
}
