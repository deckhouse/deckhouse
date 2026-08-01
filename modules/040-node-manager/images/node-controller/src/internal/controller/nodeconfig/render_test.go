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
// first day-2 render. Its spec came from a dhctl payload, and only the fields
// below have no answer anywhere in the cluster, so only they survive the
// wholesale patch.
//
// Registry and serverTLSBootstrap deliberately do NOT: both are rendered from
// the cluster now. Carrying them over made the master keep whatever the
// bootstrap chose forever — a self-signed kubelet serving certificate with no
// IP in it (so `kubectl exec` and `kubectl logs` failed against every pod on the
// master), and, on the create path where there is no existing object to carry
// anything from, no registry at all.
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
		name       string
		existing   internalv1alpha1.NodeSpec
		expStorage internalv1alpha1.Storage
		expNodeIP  string
	}{
		{
			name:       "bootstrapped master keeps only what the cluster cannot render",
			existing:   bootstrapped,
			expStorage: bootstrapped.Storage,
			expNodeIP:  "10.0.0.10",
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
			rendered := &internalv1alpha1.Registry{Address: "rendered.example.com"}
			desired := internalv1alpha1.NodeSpec{
				NodeName: "master-0",
				Registry: rendered,
				Storage:  renderStorage(),
				Kubelet:  internalv1alpha1.Kubelet{MaxPods: defaultMaxPods},
			}

			keepBootstrapOnlyFields(&desired, &tc.existing)

			require.Equal(t, tc.expStorage, desired.Storage)
			require.Equal(t, tc.expNodeIP, desired.Kubelet.NodeIP)
			require.Equal(t, tc.existing.Kubelet.ResourceReservation, desired.Kubelet.ResourceReservation)

			// The cluster owns these two, so the render wins even when the node
			// was bootstrapped with something else.
			require.Equal(t, rendered, desired.Registry)
			require.Nil(t, desired.Kubelet.ServerTLSBootstrap)

			// Everything else still comes from the render.
			require.Equal(t, "master-0", desired.NodeName)
			require.Equal(t, defaultMaxPods, desired.Kubelet.MaxPods)
		})
	}
}
