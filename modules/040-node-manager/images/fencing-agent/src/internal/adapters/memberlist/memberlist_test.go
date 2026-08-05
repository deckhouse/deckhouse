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
	"strings"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

// testTuning returns a valid tuning fixture shared by the buildConfig tests.
func testTuning() domain.MemberlistTuning {
	return domain.MemberlistTuning{
		ProbeInterval:           100 * time.Millisecond,
		ProbeTimeout:            50 * time.Millisecond,
		SuspicionMult:           2,
		SuspicionMaxTimeoutMult: 6,
		IndirectChecks:          4,
		AwarenessMaxMultiplier:  8,
		GossipInterval:          100 * time.Millisecond,
		RetransmitMult:          4,
		GossipToTheDeadTime:     2 * time.Second,
	}
}

func TestBuildConfigAppliesProfileTuning(t *testing.T) {
	tuning := testTuning()

	cfg := buildConfig(Config{
		NodeName:      "worker-1",
		NodeGroup:     "worker",
		AdvertiseAddr: "10.0.0.1",
		Port:          8500,
		Tuning:        tuning,
	}, log.NewNop(), newEventDelegate(log.NewNop()))

	if cfg.ProbeInterval != tuning.ProbeInterval || cfg.ProbeTimeout != tuning.ProbeTimeout {
		t.Errorf("probe timings not applied: interval=%s timeout=%s", cfg.ProbeInterval, cfg.ProbeTimeout)
	}

	if cfg.SuspicionMult != 2 || cfg.SuspicionMaxTimeoutMult != 6 {
		t.Errorf("suspicion tuning not applied: mult=%d maxMult=%d", cfg.SuspicionMult, cfg.SuspicionMaxTimeoutMult)
	}

	if cfg.IndirectChecks != 4 || cfg.AwarenessMaxMultiplier != 8 {
		t.Errorf("lifeguard tuning not applied: indirect=%d awareness=%d", cfg.IndirectChecks, cfg.AwarenessMaxMultiplier)
	}

	if cfg.GossipInterval != tuning.GossipInterval || cfg.RetransmitMult != 4 || cfg.GossipToTheDeadTime != tuning.GossipToTheDeadTime {
		t.Errorf("gossip tuning not applied: interval=%s retransmit=%d deadTime=%s",
			cfg.GossipInterval, cfg.RetransmitMult, cfg.GossipToTheDeadTime)
	}
}

func TestBuildConfigAdvertisesTheNodeAddress(t *testing.T) {
	cfg := buildConfig(Config{
		NodeName:      "worker-1",
		NodeGroup:     "worker",
		AdvertiseAddr: "10.0.0.1",
		Port:          8500,
		Tuning:        testTuning(),
	}, log.NewNop(), newEventDelegate(log.NewNop()))

	if cfg.Name != "worker-1" {
		t.Errorf("Name is %q, want worker-1", cfg.Name)
	}

	// Peers reach the agent through the hostPort on the Node address, so an
	// auto-detected pod IP here would make the node unreachable.
	if cfg.AdvertiseAddr != "10.0.0.1" {
		t.Errorf("AdvertiseAddr is %q, want the Node InternalIP", cfg.AdvertiseAddr)
	}

	if cfg.BindAddr != bindAddress {
		t.Errorf("BindAddr is %q, want %q", cfg.BindAddr, bindAddress)
	}

	if cfg.BindPort != 8500 || cfg.AdvertisePort != 8500 {
		t.Errorf("ports are bind=%d advertise=%d, want both 8500", cfg.BindPort, cfg.AdvertisePort)
	}

	// The label keeps each NodeGroup in its own gossip network.
	if cfg.Label != "worker" {
		t.Errorf("Label is %q, want the node group name", cfg.Label)
	}

	// Zero would make peers refuse a fenced node that comes back under the
	// same name with a new address.
	if cfg.DeadNodeReclaimTime <= 0 {
		t.Error("DeadNodeReclaimTime must be positive")
	}

	if cfg.Logger == nil || cfg.Events == nil {
		t.Error("logger and event delegate must be wired")
	}
}

func TestNewRejectsZeroTuning(t *testing.T) {
	// No listener is created on this error path (the guard runs before
	// hcml.Create), so there is no port to clean up.
	_, err := New(Config{
		NodeName:      "worker-1",
		NodeGroup:     "worker",
		AdvertiseAddr: "127.0.0.1",
		Port:          0,
	}, log.NewNop())

	if err == nil {
		t.Fatal("expected an error for zero-value Tuning, memberlist must refuse to start")
	}

	if !strings.Contains(err.Error(), "ProbeInterval") {
		t.Errorf("error %q does not mention ProbeInterval", err)
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
