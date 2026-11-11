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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/testenv"
)

func masterSpec(servers ...string) internalv1alpha1.NodeSpec {
	return internalv1alpha1.NodeSpec{
		NodeName: "master-0",
		Network:  internalv1alpha1.Network{Hostname: "master-0", NTP: internalv1alpha1.NTP{Servers: servers}},
	}
}

// New servers alone are a change on a master: the cached object must not read as up to date.
func TestNewNTPServersAreAChangeOnAMaster(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, internalv1alpha1.AddToScheme(scheme))
	cached := &internalv1alpha1.NodeConfig{ObjectMeta: metav1.ObjectMeta{Name: "master-0"}, Spec: masterSpec("old.example")}
	r := &Reconciler{}
	r.Client = fake.NewClientBuilder().WithScheme(scheme).WithObjects(cached).Build()
	desired := &internalv1alpha1.NodeConfig{ObjectMeta: metav1.ObjectMeta{Name: "master-0"}, Spec: masterSpec("new.example")}

	require.Nil(t, r.unchanged(t.Context(), immutableGroup("master"), desired), "a master given new servers is not unchanged")
	require.NotNil(t, r.unchanged(t.Context(), immutableGroup("worker"), desired), "elsewhere the servers are the machine's")
}

// A master that received the new servers and has not applied them holds its rollout slot;
// one still on the old servers does not.
func TestAMasterApplyingNewNTPServersHoldsItsSlot(t *testing.T) {
	want := map[string]internalv1alpha1.NodeSpec{"master-0": masterSpec("new.example")}

	updating := membersOfUpdating([]internalv1alpha1.NodeConfig{notApplied("master-0", masterSpec("new.example"))}, want, immutableGroup("master"))
	require.Contains(t, updating, "master-0")

	updating = membersOfUpdating([]internalv1alpha1.NodeConfig{notApplied("master-0", masterSpec("old.example"))}, want, immutableGroup("master"))
	require.NotContains(t, updating, "master-0")
}

// The watch predicate passes the ConfigMap of the boot-time NTP servers, like the release's own inputs.
func TestTheWatchPredicatePassesTheRenderInputConfigMaps(t *testing.T) {
	cm := func(namespace, name string) *corev1.ConfigMap {
		return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
	}

	for _, name := range []string{imagesDigestsConfigMapName, platformTemplatesConfigMapName, bootTimeSyncConfigMapName} {
		require.True(t, isRenderInputConfigMap(cm(cloudInstanceManagerNS, name)), name)
	}

	require.False(t, isRenderInputConfigMap(cm(cloudInstanceManagerNS, "unrelated")))
	require.False(t, isRenderInputConfigMap(cm("kube-system", bootTimeSyncConfigMapName)))
}

// The chrony ConfigMap affects master renders only, including when it cannot be parsed.
func TestTheBootTimeNTPServersReachTheMastersRender(t *testing.T) {
	for _, tc := range []struct {
		name      string
		published string
		wantError bool
	}{
		{name: "valid servers", published: `["pool.ntp.org"]`},
		{name: "invalid JSON", published: `[`, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cluster := mutableCluster(t, testenv.ImagesDigestsJSON(testenv.TestContainerdDigest),
				dnsService(kubeDNSServiceName, "kube-dns", "10.0.0.10"),
				clusterCAConfigMapObject(testenv.TestClusterCA),
				clusterConfigSecret("clusterDomain: cluster.local\n"),
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: cloudInstanceManagerNS, Name: bootTimeSyncConfigMapName},
					Data:       map[string]string{bootTimeSyncServersKey: tc.published},
				},
			)
			r := &Reconciler{sources: sourceReaderOver(cluster)}
			p := newPass()
			p.derivedVersion = &versionResult{version: testenv.TestKubernetesVersion}

			master, err := r.renderNode(t.Context(), immutableGroup("master"), nodeWith(nil), nil, p)
			if tc.wantError {
				require.ErrorContains(t, err, bootTimeSyncConfigMapName)
			} else {
				require.NoError(t, err)
				require.Equal(t, []string{"pool.ntp.org"}, master.Network.NTP.Servers)
			}

			worker, err := r.renderNode(t.Context(), immutableGroup("worker"), nodeWith(nil), nil, p)
			require.NoError(t, err)
			require.Empty(t, worker.Network.NTP.Servers)
		})
	}
}
