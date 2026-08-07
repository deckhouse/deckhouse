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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/config"
	"fencing-agent/internal/domain"
)

// Every timing below is distinct so a swapped assignment cannot pass.
func testAgent() *Agent {
	return New(
		&config.Config{NodeGroup: "worker", MemberlistPort: 8500},
		Deps{},
		domain.NodeIdentity{Name: "worker-1", UID: "uid-1", IP: "10.0.0.1"},
		v1alpha1.FencingSLAProfileSpec{
			Memberlist: v1alpha1.FencingSLAProfileMemberlist{
				ProbeInterval: metav1.Duration{Duration: 300 * time.Millisecond},
				ProbeTimeout:  metav1.Duration{Duration: 120 * time.Millisecond},
				SuspicionMult: 3,
			},
			Fallback: v1alpha1.FencingSLAProfileFallback{
				Heartbeat:            metav1.Duration{Duration: 1 * time.Second},
				TTL:                  metav1.Duration{Duration: 4 * time.Second},
				KubernetesAPITimeout: metav1.Duration{Duration: 2 * time.Second},
			},
			Rejoin: v1alpha1.FencingSLAProfileRejoin{
				Interval:    metav1.Duration{Duration: 5 * time.Second},
				MaxInterval: metav1.Duration{Duration: 30 * time.Second},
			},
			Watchdog: v1alpha1.FencingSLAProfileWatchdog{
				FeedInterval: metav1.Duration{Duration: 6 * time.Second},
				Timeout:      metav1.Duration{Duration: 60 * time.Second},
			},
		},
		log.NewNop(),
	)
}

func TestMemberlistConfigCarriesIdentityAndTuning(t *testing.T) {
	cfg := testAgent().memberlistConfig()

	if cfg.NodeName != "worker-1" {
		t.Errorf("NodeName is %q, want worker-1", cfg.NodeName)
	}

	// Peers reach the agent through the hostPort on the Node address.
	if cfg.AdvertiseAddr != "10.0.0.1" {
		t.Errorf("AdvertiseAddr is %q, want the Node InternalIP", cfg.AdvertiseAddr)
	}

	if cfg.NodeGroup != "worker" || cfg.Port != 8500 {
		t.Errorf("group/port are %q/%d, want worker/8500", cfg.NodeGroup, cfg.Port)
	}

	if cfg.Tuning.ProbeInterval.Duration != 300*time.Millisecond ||
		cfg.Tuning.ProbeTimeout.Duration != 120*time.Millisecond ||
		cfg.Tuning.SuspicionMult != 3 {
		t.Errorf("memberlist tuning does not come from the profile: %+v", cfg.Tuning)
	}
}

// The join loop takes three different timings from three different profile
// sections; a swap compiles and would only show up as a wrong retry pace.
func TestJoinParamsTakeTimingsFromTheirOwnProfileSections(t *testing.T) {
	params := testAgent().joinParams()

	if params.APITimeout != 2*time.Second {
		t.Errorf("APITimeout is %s, want fallback.kubernetesAPITimeout (2s)", params.APITimeout)
	}

	if params.RetryInterval != 5*time.Second {
		t.Errorf("RetryInterval is %s, want rejoin.interval (5s)", params.RetryInterval)
	}

	if params.MaxRetryInterval != 30*time.Second {
		t.Errorf("MaxRetryInterval is %s, want rejoin.maxInterval (30s)", params.MaxRetryInterval)
	}

	if params.NodeName != "worker-1" || params.NodeIP != "10.0.0.1" ||
		params.NodeGroup != "worker" || params.MemberlistPort != 8500 {
		t.Errorf("identity is not wired: %+v", params)
	}
}
