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

package implementation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDecide is the whole of this backport: which clusters may take a release that no longer carries
// the previous implementation.
//
// A table, because the answer is a policy worth reading in one place. The interesting rows are the
// two `Direct` ones — that mode is why this package exists, and it is admitted only when the
// operator has already said where images come from afterwards.
func TestDecide(t *testing.T) {
	for _, tc := range []struct {
		name       string
		legacy     *legacyState
		desired    string
		configured bool
		want       string
	}{
		{
			name:   "no legacy state at all: nothing of the previous implementation to lose",
			legacy: nil,
			want:   ImplementationV2,
		},
		{
			name:   "Unmanaged: the previous implementation has let go of the pull path",
			legacy: &legacyState{Mode: "Unmanaged"},
			want:   ImplementationV2,
		},
		{
			name:   "Unmanaged but on its way elsewhere: nodes are being reconfigured right now",
			legacy: &legacyState{Mode: "Unmanaged", TargetMode: "Local"},
			want:   ImplementationLegacy,
		},
		{
			name:       "Direct with the module configured: the replacement is already named",
			legacy:     &legacyState{Mode: "Direct"},
			configured: true,
			want:       ImplementationV2,
		},
		{
			name:   "Direct with nothing configured: the in-cluster address would be orphaned",
			legacy: &legacyState{Mode: "Direct"},
			want:   ImplementationLegacy,
		},
		{
			name:       "Direct mid-transition: the mode says Direct and the cluster is moving",
			legacy:     &legacyState{Mode: "Direct", TargetMode: "Local"},
			configured: true,
			want:       ImplementationLegacy,
		},
		{
			name:       "Proxy stays refused even when configured: static pods and their PKI live on the nodes",
			legacy:     &legacyState{Mode: "Proxy"},
			configured: true,
			want:       ImplementationLegacy,
		},
		{
			name:       "Local stays refused even when configured: the store on the master disks has no replacement yet",
			legacy:     &legacyState{Mode: "Local"},
			configured: true,
			want:       ImplementationLegacy,
		},
		{
			name:   "a state that records no mode: refused rather than guessed",
			legacy: &legacyState{},
			want:   ImplementationLegacy,
		},

		// The desired mode in the `deckhouse` ModuleConfig runs ahead of the recorded state, which
		// learns of it only on the next render of module `deckhouse`. These rows are that gap.
		{
			name:    "Unmanaged with Proxy asked for: refused before the state has heard of it",
			legacy:  &legacyState{Mode: "Unmanaged"},
			desired: "Proxy",
			want:    ImplementationLegacy,
		},
		{
			name:    "no state at all with Local asked for: refused, the previous implementation is about to take the cluster",
			legacy:  nil,
			desired: "Local",
			want:    ImplementationLegacy,
		},
		{
			name:       "Direct and configured, the case that is admitted, with Proxy asked for: refused",
			legacy:     &legacyState{Mode: "Direct"},
			desired:    "Proxy",
			configured: true,
			want:       ImplementationLegacy,
		},
		{
			name:       "Unmanaged with Direct asked for: refused, a switch into Direct is in flight",
			legacy:     &legacyState{Mode: "Unmanaged"},
			desired:    "Direct",
			configured: true,
			want:       ImplementationLegacy,
		},
		{
			name:       "Direct asked for and already recorded: the state decides",
			legacy:     &legacyState{Mode: "Direct", TargetMode: "Direct"},
			desired:    "Direct",
			configured: true,
			want:       ImplementationV2,
		},
		{
			name:    "Unmanaged asked for and recorded: the state decides",
			legacy:  &legacyState{Mode: "Unmanaged"},
			desired: "Unmanaged",
			want:    ImplementationV2,
		},
		{
			name:       "Direct and configured with Unmanaged asked for: refused, a switch out of Direct is in flight",
			legacy:     &legacyState{Mode: "Direct", TargetMode: "Direct"},
			desired:    "Unmanaged",
			configured: true,
			want:       ImplementationLegacy,
		},
		{
			name:       "Direct and configured with no deckhouse ModuleConfig to read: the state decides",
			legacy:     &legacyState{Mode: "Direct", TargetMode: "Direct"},
			desired:    "",
			configured: true,
			want:       ImplementationV2,
		},
		{
			name:    "no state at all with Unmanaged asked for: nothing to switch",
			legacy:  nil,
			desired: "Unmanaged",
			want:    ImplementationV2,
		},
		{
			name:    "Unmanaged asked for while Proxy is recorded: the switch has not finished",
			legacy:  &legacyState{Mode: "Proxy"},
			desired: "Unmanaged",
			want:    ImplementationLegacy,
		},
		{
			name:    "a desired mode this code does not know: refused rather than guessed",
			legacy:  nil,
			desired: desiredModeUnreadable,
			want:    ImplementationLegacy,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, decide(tc.legacy, tc.desired, tc.configured))
		})
	}
}
