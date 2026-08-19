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

package derived_status

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func kubernetesSourceConfigMap(desiredVersion string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: clusterConfigSecretNamespace, Name: clusterKubernetesConfigMapName},
		Data:       map[string]string{"spec": "desiredVersion: " + desiredVersion + "\nupdateMode: Manual\n"},
	}
}

func resolvedTarget(t *testing.T, objects ...client.Object) string {
	t.Helper()
	target, err := newTestServiceRaw(t, objects...).readTargetKubernetesVersion(context.Background())
	require.NoError(t, err)
	require.NotNil(t, target)
	return semverMajMin(target)
}

func degradedTarget(t *testing.T, objects ...client.Object) {
	t.Helper()
	target, err := newTestServiceRaw(t, objects...).readTargetKubernetesVersion(context.Background())
	require.NoError(t, err)
	assert.Nil(t, target)
}

// An absent or unusable value degrades to nil rather than erroring, and the caller then falls back to
// the running kube-apiserver version. Only an unreadable ConfigMap is an error, which is the boundary
// every other source in this file draws: absence and shape degrade, unavailability propagates (see
// sources_error_test.go).
func TestReadTargetKubernetesVersion(t *testing.T) {
	t.Run("ConfigMap desiredVersion is the only source", func(t *testing.T) {
		assert.Equal(t, "1.35", resolvedTarget(t, kubernetesSourceConfigMap("1.35")))
	})

	t.Run("missing ConfigMap degrades", func(t *testing.T) {
		degradedTarget(t)
	})

	t.Run("empty desiredVersion degrades", func(t *testing.T) {
		cm := kubernetesSourceConfigMap("")
		cm.Data["spec"] = "desiredVersion: \"\"\nupdateMode: Automatic\n"
		degradedTarget(t, cm)
	})

	t.Run("missing spec key degrades", func(t *testing.T) {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: clusterConfigSecretNamespace, Name: clusterKubernetesConfigMapName},
			Data:       map[string]string{},
		}
		degradedTarget(t, cm)
	})

	t.Run("invalid desiredVersion degrades", func(t *testing.T) {
		degradedTarget(t, kubernetesSourceConfigMap("not-a-version"))
	})

	t.Run("unparsable ConfigMap spec degrades", func(t *testing.T) {
		cm := kubernetesSourceConfigMap("1.35")
		cm.Data["spec"] = "desiredVersion: [broken\n"
		degradedTarget(t, cm)
	})
}

func apiserverPod(name, version string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   apiserverPodNamespace,
			Name:        name,
			Labels:      map[string]string{"component": "kube-apiserver", "tier": "control-plane"},
			Annotations: map[string]string{apiserverVersionAnnKey: version},
		},
	}
}

// Derivation must keep producing a bashible context when the ConfigMap is absent — managed clusters
// have no such object at all.
func TestComputeDegradesWithoutClusterKubernetesConfigMap(t *testing.T) {
	nodeGroup := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}

	derive := func(t *testing.T, objects ...client.Object) Result {
		t.Helper()
		s := newTestServiceRaw(t, objects...)
		result, _, err := s.ComputeWithCloudChecks(context.Background(), nodeGroup, testRegistry(t, s))
		require.NoError(t, err)
		return result
	}

	t.Run("falls back to the kube-apiserver version", func(t *testing.T) {
		result := derive(t, apiserverPod("kube-apiserver-master-0", "1.34.5"))
		assert.Equal(t, "1.34", result.KubernetesVersion)
		assert.Equal(t, "Containerd", result.CRIType)
	})

	t.Run("no ConfigMap and no apiservers leaves the version empty", func(t *testing.T) {
		assert.Empty(t, derive(t).KubernetesVersion)
	})

	t.Run("a broken spec falls back too", func(t *testing.T) {
		cm := kubernetesSourceConfigMap("1.35")
		cm.Data["spec"] = "desiredVersion: \"\"\n"
		result := derive(t, cm, apiserverPod("kube-apiserver-master-0", "1.34.5"))
		assert.Equal(t, "1.34", result.KubernetesVersion)
	})
}
