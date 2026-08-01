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

package nodeconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// TestKeepBootstrapOnlyFields covers what happens to the first master on its
// first day-2 render. Its spec came from a dhctl payload, and the fields below
// have no NodeGroup behind them, so a rendered spec carries none of them: the
// wholesale patch would turn serverTLSBootstrap back on (kubelet then waits for
// a serving certificate nobody signs), drop the direct registry access the
// control-plane static pods pull with, and replace the by-size disk selector
// that tells the master's two disks apart.
func TestKeepBootstrapOnlyFields(t *testing.T) {
	bootstrapped := internalv1alpha1.NodeSpec{
		Registry: &internalv1alpha1.Registry{Address: "registry.example.com", Path: "/deckhouse/ce"},
		Storage: internalv1alpha1.Storage{
			Disk: internalv1alpha1.Disk{DiskSelector: &internalv1alpha1.DiskSelector{Size: ">=30Gi"}},
		},
		Kubelet: internalv1alpha1.Kubelet{
			ServerTLSBootstrap:  ptr.To(false),
			NodeIP:              "10.0.0.10",
			ResourceReservation: &internalv1alpha1.ResourceReservation{Mode: "Auto"},
		},
	}

	tests := []struct {
		name        string
		existing    internalv1alpha1.NodeSpec
		expStorage  internalv1alpha1.Storage
		expRegistry *internalv1alpha1.Registry
		expTLS      *bool
		expNodeIP   string
	}{
		{
			name:        "bootstrapped master keeps every field the render has no answer for",
			existing:    bootstrapped,
			expStorage:  bootstrapped.Storage,
			expRegistry: bootstrapped.Registry,
			expTLS:      ptr.To(false),
			expNodeIP:   "10.0.0.10",
		},
		{
			name:       "worker with nothing to keep is left as rendered",
			existing:   internalv1alpha1.NodeSpec{Storage: renderStorage()},
			expStorage: renderStorage(),
		},
		{
			name: "an explicit device is as specific as a selector",
			existing: internalv1alpha1.NodeSpec{
				Storage: internalv1alpha1.Storage{Disk: internalv1alpha1.Disk{Device: "/dev/sdb"}},
			},
			expStorage: internalv1alpha1.Storage{Disk: internalv1alpha1.Disk{Device: "/dev/sdb"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			desired := internalv1alpha1.NodeSpec{
				NodeName: "master-0",
				Storage:  renderStorage(),
				Kubelet:  internalv1alpha1.Kubelet{MaxPods: defaultMaxPods},
			}

			keepBootstrapOnlyFields(&desired, &tc.existing)

			require.Equal(t, tc.expStorage, desired.Storage)
			require.Equal(t, tc.expRegistry, desired.Registry)
			require.Equal(t, tc.expTLS, desired.Kubelet.ServerTLSBootstrap)
			require.Equal(t, tc.expNodeIP, desired.Kubelet.NodeIP)
			require.Equal(t, tc.existing.Kubelet.ResourceReservation, desired.Kubelet.ResourceReservation)

			// Everything else still comes from the render.
			require.Equal(t, "master-0", desired.NodeName)
			require.Equal(t, defaultMaxPods, desired.Kubelet.MaxPods)
		})
	}
}
