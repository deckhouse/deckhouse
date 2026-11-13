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

package fsm

import (
	"testing"
	"time"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
)

// The timings of the normal profile: the heartbeat TTL is 2 * 2s + 5s, the
// attested taint comes 15s + 750ms after the attestation, and the silent taint
// comes 25.95s after the last sign of life.
var normal = Params{
	Armed:                  true,
	HeartbeatTTL:           9 * time.Second,
	AttestedNodeTaintAfter: 15750 * time.Millisecond,
	HeartbeatInterval:      2 * time.Second,
	SilentNodeTaintAfter:   25950 * time.Millisecond,
}

var now = time.Date(2026, time.June, 2, 15, 0, 30, 0, time.UTC)

func TestNext(t *testing.T) {
	for name, tc := range map[string]struct {
		phase          v1alpha1.FencingFailedNodeStatePhase
		unarmed        bool
		incidentAgo    time.Duration
		heartbeatAgo   time.Duration
		attestationAgo time.Duration
		wantPhase      v1alpha1.FencingFailedNodeStatePhase
		wantAfter      time.Duration
	}{
		"a new incident is suspected": {
			wantPhase: v1alpha1.PhaseSuspected,
		},
		"a heartbeat makes a new incident fallback alive": {
			heartbeatAgo: time.Second,
			wantPhase:    v1alpha1.PhaseFallbackAlive,
			wantAfter:    8 * time.Second,
		},
		"a suspected incident without a heartbeat stays suspected": {
			phase:     v1alpha1.PhaseSuspected,
			wantPhase: v1alpha1.PhaseSuspected,
		},
		"a suspected incident with a heartbeat is fallback alive": {
			phase:        v1alpha1.PhaseSuspected,
			heartbeatAgo: 2 * time.Second,
			wantPhase:    v1alpha1.PhaseFallbackAlive,
			wantAfter:    7 * time.Second,
		},
		"a heartbeat younger than the TTL keeps fallback alive": {
			phase:        v1alpha1.PhaseFallbackAlive,
			heartbeatAgo: 8 * time.Second,
			wantPhase:    v1alpha1.PhaseFallbackAlive,
			wantAfter:    time.Second,
		},
		"a heartbeat seen 10s ago returns the incident to suspected": {
			phase:        v1alpha1.PhaseFallbackAlive,
			heartbeatAgo: 10 * time.Second,
			wantPhase:    v1alpha1.PhaseSuspected,
		},
		"a heartbeat as old as the TTL returns the incident to suspected": {
			phase:        v1alpha1.PhaseFallbackAlive,
			heartbeatAgo: 9 * time.Second,
			wantPhase:    v1alpha1.PhaseSuspected,
		},
		"an attestation proves the failure after watchdog.timeout + watchdog.margin": {
			phase:          v1alpha1.PhaseSuspected,
			attestationAgo: 15750 * time.Millisecond,
			wantPhase:      v1alpha1.PhaseProven,
		},
		"an attestation younger than the deadline does not prove the failure yet": {
			phase:          v1alpha1.PhaseSuspected,
			attestationAgo: 15 * time.Second,
			wantPhase:      v1alpha1.PhaseSuspected,
			wantAfter:      750 * time.Millisecond,
		},
		"an attestation of an unarmed node never proves the failure": {
			phase:          v1alpha1.PhaseSuspected,
			unarmed:        true,
			attestationAgo: time.Hour,
			wantPhase:      v1alpha1.PhaseSuspected,
		},
		"a heartbeat after the attestation stops the proof": {
			phase:          v1alpha1.PhaseSuspected,
			attestationAgo: time.Hour,
			heartbeatAgo:   time.Hour - time.Second,
			wantPhase:      v1alpha1.PhaseSuspected,
		},
		"a heartbeat seen in the same reconcile as the attestation stops the proof": {
			phase:          v1alpha1.PhaseFallbackAlive,
			attestationAgo: 16 * time.Second,
			heartbeatAgo:   16 * time.Second,
			wantPhase:      v1alpha1.PhaseSuspected,
		},
		"a heartbeat before the attestation keeps fallback alive until its TTL": {
			phase:          v1alpha1.PhaseFallbackAlive,
			heartbeatAgo:   3 * time.Second,
			attestationAgo: 2 * time.Second,
			wantPhase:      v1alpha1.PhaseFallbackAlive,
			wantAfter:      6 * time.Second,
		},
		"a silent node is proven when the silent delay passed after the controller saw the object": {
			phase:       v1alpha1.PhaseSuspected,
			incidentAgo: 25950 * time.Millisecond,
			wantPhase:   v1alpha1.PhaseProven,
		},
		"a silent node is suspected until the silent delay passed": {
			phase:       v1alpha1.PhaseSuspected,
			incidentAgo: 25900 * time.Millisecond,
			wantPhase:   v1alpha1.PhaseSuspected,
			wantAfter:   50 * time.Millisecond,
		},
		"a node that writes heartbeats is not proven": {
			phase:        v1alpha1.PhaseFallbackAlive,
			incidentAgo:  time.Hour,
			heartbeatAgo: time.Second,
			wantPhase:    v1alpha1.PhaseFallbackAlive,
			wantAfter:    8 * time.Second,
		},
		"the silent delay starts at the last heartbeat plus heartbeat.interval": {
			phase:        v1alpha1.PhaseSuspected,
			incidentAgo:  time.Hour,
			heartbeatAgo: 27 * time.Second,
			wantPhase:    v1alpha1.PhaseSuspected,
			wantAfter:    950 * time.Millisecond,
		},
		"a node silent after its last heartbeat is proven": {
			phase:        v1alpha1.PhaseSuspected,
			incidentAgo:  time.Hour,
			heartbeatAgo: 27950 * time.Millisecond,
			wantPhase:    v1alpha1.PhaseProven,
		},
		"a node alive after the attestation is proven by the silent delay": {
			phase:          v1alpha1.PhaseSuspected,
			incidentAgo:    time.Hour,
			attestationAgo: time.Hour,
			heartbeatAgo:   27950 * time.Millisecond,
			wantPhase:      v1alpha1.PhaseProven,
		},
		"the attestation proves the failure before the silent delay": {
			phase:          v1alpha1.PhaseSuspected,
			incidentAgo:    16 * time.Second,
			attestationAgo: 15750 * time.Millisecond,
			wantPhase:      v1alpha1.PhaseProven,
		},
		"the silence of an unarmed node never proves the failure": {
			phase:       v1alpha1.PhaseSuspected,
			unarmed:     true,
			incidentAgo: time.Hour,
			wantPhase:   v1alpha1.PhaseSuspected,
		},
		"a proven incident keeps its phase": {
			phase:        v1alpha1.PhaseProven,
			heartbeatAgo: time.Second,
			wantPhase:    v1alpha1.PhaseProven,
		},
		"an evicting incident keeps its phase": {
			phase:     v1alpha1.PhaseEvicting,
			wantPhase: v1alpha1.PhaseEvicting,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var seen Observations
			if tc.incidentAgo > 0 {
				seen.Incident = now.Add(-tc.incidentAgo)
			}

			if tc.heartbeatAgo > 0 {
				seen.Heartbeat = now.Add(-tc.heartbeatAgo)
			}

			if tc.attestationAgo > 0 {
				seen.Attestation = now.Add(-tc.attestationAgo)
			}

			params := normal
			params.Armed = !tc.unarmed

			phase, after := Next(tc.phase, seen, params, now)

			if phase != tc.wantPhase {
				t.Errorf("phase is %q, want %q", phase, tc.wantPhase)
			}

			if after != tc.wantAfter {
				t.Errorf("next check after %s, want %s", after, tc.wantAfter)
			}
		})
	}
}
