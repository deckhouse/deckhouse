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

package v2

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGateDecide(t *testing.T) {
	cases := []struct {
		name    string
		gate    gate
		enabled bool
		// blocked is a fragment of the reason, empty when nothing should be blocked.
		blocked string
	}{{
		// A cluster the previous implementation has never touched. There is no choice to
		// offer and nothing to hand over, so this one manages it from the start.
		name:    "a cluster with no previous state at all",
		gate:    gate{},
		enabled: true,
	}, {
		name:    "a cluster the previous implementation has let go of",
		gate:    gate{Legacy: &legacyState{Mode: "Unmanaged"}},
		enabled: true,
	}, {
		// The case the handover exists for. Both implementations would write
		// /etc/containerd/registry.d on the same nodes.
		name:    "a cluster the previous implementation still manages",
		gate:    gate{Legacy: &legacyState{Mode: "Proxy"}},
		enabled: false,
		blocked: `the cluster is in the "Proxy" mode`,
	}, {
		name:    "a cluster mid-transition inside the previous implementation",
		gate:    gate{Legacy: &legacyState{Mode: "Unmanaged", TargetMode: "Local"}},
		enabled: false,
		blocked: `transitioning to "Local"`,
	}, {
		// Settling into Unmanaged is not a reason to wait: the target is where the
		// handover wants the cluster anyway.
		name:    "a cluster settling into Unmanaged",
		gate:    gate{Legacy: &legacyState{Mode: "Unmanaged", TargetMode: "Unmanaged"}},
		enabled: true,
	}, {
		name:    "a previous state that records no mode",
		gate:    gate{Legacy: &legacyState{}},
		enabled: false,
		blocked: "has not recorded a mode",
	}, {
		// Fails closed. Guessing here would enable a second writer on every node.
		name: "a previous state that cannot be read",
		gate: gate{
			LegacyUnreadable: errors.New("decoding the legacy registry state: yaml: line 3: mapping values are not allowed"),
		},
		enabled: false,
		blocked: "cannot be read",
	}, {
		// Once handed over, the question has expired. A module restart must not re-ask it
		// and hand the cluster back — the previous state secret is still there and still
		// says whatever it said.
		name: "after the handover, with the previous state still saying Proxy",
		gate: gate{
			AlreadySwitched: true,
			Legacy:          &legacyState{Mode: "Proxy"},
		},
		enabled: true,
	}, {
		// The marker alone is enough. There is no setting that could contradict it,
		// which is the whole point of removing the choice.
		name:    "after the handover, with no previous state left",
		gate:    gate{AlreadySwitched: true},
		enabled: true,
	}}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			enabled, reason := test.gate.decide()

			assert.Equal(t, test.enabled, enabled)
			if test.blocked == "" {
				assert.Empty(t, reason)
				return
			}
			assert.Contains(t, reason, test.blocked)
		})
	}
}
