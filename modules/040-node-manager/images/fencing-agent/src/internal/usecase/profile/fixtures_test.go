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

package profile

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
)

func dur(d time.Duration) metav1.Duration {
	return metav1.Duration{Duration: d}
}

func sec(s float64) time.Duration {
	return time.Duration(s * float64(time.Second))
}

func newSpec(
	controlPlaneLostAfter, probeInterval, probeTimeout time.Duration,
	suspicionMult, suspicionMaxTimeoutMult int32,
	gossipInterval, witnessFresherBy, watchdogTimeout, feedInterval, margin,
	heartbeatInterval, apiTimeout, writerWaveInterval time.Duration,
) v1alpha1.FencingSLAProfileSpec {
	return v1alpha1.FencingSLAProfileSpec{
		ControlPlaneLostAfter: dur(controlPlaneLostAfter),
		Memberlist: v1alpha1.FencingSLAProfileMemberlist{
			ProbeInterval:           dur(probeInterval),
			ProbeTimeout:            dur(probeTimeout),
			SuspicionMult:           suspicionMult,
			SuspicionMaxTimeoutMult: suspicionMaxTimeoutMult,
			IndirectChecks:          3,
			AwarenessMaxMultiplier:  8,
			GossipInterval:          dur(gossipInterval),
			GossipNodes:             3,
			RetransmitMult:          4,
			DeadNodeReclaimTime:     dur(time.Second),
			GossipToTheDeadTime:     dur(30 * time.Second),
		},
		Rejoin:    v1alpha1.FencingSLAProfileRejoin{Interval: dur(5 * time.Second)},
		Poll:      v1alpha1.FencingSLAProfilePoll{WitnessFresherBy: dur(witnessFresherBy)},
		Watchdog:  v1alpha1.FencingSLAProfileWatchdog{Timeout: dur(watchdogTimeout), FeedInterval: dur(feedInterval), Margin: dur(margin)},
		Heartbeat: v1alpha1.FencingSLAProfileHeartbeat{Interval: dur(heartbeatInterval), APITimeout: dur(apiTimeout)},
		Eviction:  v1alpha1.FencingSLAProfileEviction{WriterWaveInterval: dur(writerWaveInterval)},
	}
}

// validSpec returns a profile spec that satisfies every rule of Validate. A
// test changes one value to break one rule.
func validSpec() v1alpha1.FencingSLAProfileSpec {
	ms := time.Millisecond

	return newSpec(sec(4.15), 200*ms, 80*ms, 3, 4, 200*ms, sec(2.35), 5*time.Second, 250*ms, 300*ms, time.Second, 2*time.Second, 2*time.Second)
}

func profileOf(spec v1alpha1.FencingSLAProfileSpec) *v1alpha1.FencingSLAProfile {
	return &v1alpha1.FencingSLAProfile{Spec: spec}
}
