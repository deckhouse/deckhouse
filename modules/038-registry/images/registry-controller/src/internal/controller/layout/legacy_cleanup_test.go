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

package layout

import (
	"testing"

	"github.com/stretchr/testify/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// TestLegacyPullPathMayBeRemoved is the policy behind removing what the previous implementation
// left serving the in-cluster address.
//
// Those objects carry `helm.sh/resource-policy: keep` in the release before this one, precisely so
// that they OUTLIVE the upgrade: on a `Direct` cluster they are what answers the address the nodes
// pull through, and the release that stops rendering them would otherwise delete them the moment
// the handover happens. Measured, before the annotation existed: deletion at 08:13, the node's
// containerd rewritten to the in-cluster name at 08:14 with no agent to answer it, a new etcd
// digest at 08:16, `lookup registry.d8-system.svc: no such host`, and a cluster that could not
// repair itself because the API was gone with etcd.
//
// So the question is not "has the handover happened" but "does the node agent actually serve the
// pull path on every node". Every node, not the first: a node whose bashible has not run yet still
// depends on the old objects, and removing them would cut exactly that node off.
func TestLegacyPullPathMayBeRemoved(t *testing.T) {
	ready := func(name string) registryv1alpha1.RegistryNode {
		return registryv1alpha1.RegistryNode{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Status: registryv1alpha1.RegistryNodeStatus{
				Reconciled:     true,
				ProxyListening: true,
			},
		}
	}

	for _, tc := range []struct {
		name  string
		nodes []registryv1alpha1.RegistryNode
		want  bool
	}{
		{
			name:  "no layouts at all: the agent owns nothing yet",
			nodes: nil,
			want:  false,
		},
		{
			name:  "every node reconciled and serving",
			nodes: []registryv1alpha1.RegistryNode{ready("master-0"), ready("worker-0")},
			want:  true,
		},
		{
			name: "one node has applied the configuration but is not serving yet",
			nodes: []registryv1alpha1.RegistryNode{
				ready("master-0"),
				{ObjectMeta: metav1.ObjectMeta{Name: "worker-0"}, Status: registryv1alpha1.RegistryNodeStatus{Reconciled: true}},
			},
			want: false,
		},
		{
			name: "one node has not applied it at all",
			nodes: []registryv1alpha1.RegistryNode{
				ready("master-0"),
				{ObjectMeta: metav1.ObjectMeta{Name: "worker-0"}},
			},
			want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, legacyPullPathMayBeRemoved(tc.nodes))
		})
	}
}
