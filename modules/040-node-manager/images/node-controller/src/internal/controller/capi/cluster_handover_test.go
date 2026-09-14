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

package capi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/deckhouse/node-controller/internal/cloudprovider"
	"github.com/deckhouse/node-controller/internal/common"
)

func TestClusterReconcilerIsSerialized(t *testing.T) {
	assert.Equal(t, 1, (&ClusterReconciler{}).MaxConcurrentReconciles())
}

func TestClusterReconcilerSchedulesPeriodicRepair(t *testing.T) {
	clusterConfiguration := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: common.ClusterConfigSecretName, Namespace: common.ClusterConfigSecretNamespace,
		},
		Data: map[string][]byte{"cluster-configuration.yaml": []byte(`
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
clusterDomain: cluster.local
`)},
	}
	base := fakeReconciler(t, clusterConfiguration)
	reconciler := &ClusterReconciler{BaseWithReader: base.BaseWithReader}
	reconciler.APIReader = reconciler.Client

	result, err := reconciler.Reconcile(t.Context(), ctrl.Request{})
	require.NoError(t, err)
	assert.Equal(t, clusterRepairInterval, result.RequeueAfter)
}

func TestAPIServerPodEndpointChanged(t *testing.T) {
	predicate := apiServerPodEndpointChanged()
	base := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Namespace: common.KubeSystemNamespace,
		Labels:    map[string]string{"component": "kube-apiserver", "tier": "control-plane"},
	}}

	assert.True(t, predicate.Create(event.CreateEvent{Object: base}))
	assert.False(t, predicate.Create(event.CreateEvent{Object: &corev1.Pod{}}))

	statusOnly := base.DeepCopy()
	statusOnly.Status.Phase = corev1.PodRunning
	assert.False(t, predicate.Update(event.UpdateEvent{ObjectOld: base, ObjectNew: statusOnly}))

	previouslyIrrelevant := statusOnly.DeepCopy()
	previouslyIrrelevant.Labels = map[string]string{"component": "other"}
	assert.True(t, predicate.Update(event.UpdateEvent{ObjectOld: previouslyIrrelevant, ObjectNew: statusOnly}))

	withIP := statusOnly.DeepCopy()
	withIP.Status.PodIP = "192.0.2.10"
	assert.True(t, predicate.Update(event.UpdateEvent{ObjectOld: statusOnly, ObjectNew: withIP}))

	ready := withIP.DeepCopy()
	ready.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
	assert.True(t, predicate.Update(event.UpdateEvent{ObjectOld: withIP, ObjectNew: ready}))
}

func clusterHandoverTestObject(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
		},
	}}
}

func TestPrepareClusterTemplateObject(t *testing.T) {
	object := clusterHandoverTestObject("v1", "Secret", capiNamespace, "credentials")
	object.SetLabels(map[string]string{"app": "provider-controller"})
	object.SetAnnotations(map[string]string{"provider.example/key": "value"})

	prepareClusterTemplateObject(object)

	assert.Equal(t, map[string]string{
		"app": "provider-controller", "heritage": "deckhouse", "module": "node-manager",
	}, object.GetLabels())
	assert.Equal(t, map[string]string{
		"helm.sh/resource-policy": "keep", "provider.example/key": "value",
	}, object.GetAnnotations())
}

func TestRemoveLegacyHelmMetadata(t *testing.T) {
	object := clusterHandoverTestObject("v1", "Secret", capiNamespace, "credentials")
	object.SetLabels(map[string]string{"app": "provider-controller", helmManagedByLabel: "Helm"})
	object.SetAnnotations(map[string]string{
		helmReleaseNameAnnotation:      "node-manager",
		helmReleaseNamespaceAnnotation: "d8-system",
		werfFailModeAnnotation:         "IgnoreAndContinueDeployProcess",
		werfTrackTerminationAnnotation: "NonBlocking",
		"helm.sh/resource-policy":      "keep",
	})

	require.True(t, removeLegacyHelmMetadata(object))
	assert.NotContains(t, object.GetLabels(), helmManagedByLabel)
	assert.NotContains(t, object.GetAnnotations(), helmReleaseNameAnnotation)
	assert.Equal(t, "keep", object.GetAnnotations()["helm.sh/resource-policy"])
	require.False(t, removeLegacyHelmMetadata(object))
}

func TestApplyClusterObjectAdoptsHelmManagedObject(t *testing.T) {
	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "capi-user-credentials", Namespace: capiNamespace,
			Labels: map[string]string{"app": "provider-controller", helmManagedByLabel: "Helm"},
			Annotations: map[string]string{
				helmReleaseNameAnnotation: "node-manager", "helm.sh/resource-policy": "keep",
			},
		},
		Data: map[string][]byte{"token": []byte("old")},
	}
	base := fakeReconciler(t, existing)
	reconciler := &ClusterReconciler{BaseWithReader: base.BaseWithReader}
	reconciler.APIReader = reconciler.Client

	desired := clusterHandoverTestObject("v1", "Secret", capiNamespace, "capi-user-credentials")
	desired.SetLabels(map[string]string{"app": "provider-controller"})
	desired.Object["data"] = map[string]any{"token": "bmV3"}

	require.NoError(t, reconciler.applyClusterObject(t.Context(), desired))
	adopted := &corev1.Secret{}
	require.NoError(t, reconciler.Client.Get(t.Context(), types.NamespacedName{
		Name: "capi-user-credentials", Namespace: capiNamespace,
	}, adopted))
	assert.Equal(t, []byte("new"), adopted.Data["token"])
	assert.Equal(t, "node-manager", adopted.Labels["module"])
	assert.NotContains(t, adopted.Labels, helmManagedByLabel)
	assert.NotContains(t, adopted.Annotations, helmReleaseNameAnnotation)
}

func TestApplyClusterObjectPreservesControlPlaneEndpoint(t *testing.T) {
	existing := clusterHandoverTestObject(
		"infrastructure.cluster.x-k8s.io/v1beta1", "OpenStackCluster", capiNamespace, "openstack",
	)
	existing.Object["spec"] = map[string]any{
		"controlPlaneEndpoint": map[string]any{"host": "192.0.2.10", "port": int64(6443)},
		"managedSubnets":       []any{"old"},
	}
	base := fakeReconciler(t, existing)
	reconciler := &ClusterReconciler{BaseWithReader: base.BaseWithReader}
	reconciler.APIReader = reconciler.Client

	desired := clusterHandoverTestObject(
		"infrastructure.cluster.x-k8s.io/v1beta1", "OpenStackCluster", capiNamespace, "openstack",
	)
	desired.Object["spec"] = map[string]any{
		"controlPlaneEndpoint": map[string]any{"host": "192.0.2.20", "port": int64(6443)},
		"managedSubnets":       []any{"new"},
	}
	require.NoError(t, reconciler.applyClusterObject(t.Context(), desired))

	actual := clusterHandoverTestObject(
		"infrastructure.cluster.x-k8s.io/v1beta1", "OpenStackCluster", capiNamespace, "openstack",
	)
	require.NoError(t, reconciler.Client.Get(t.Context(), types.NamespacedName{
		Name: "openstack", Namespace: capiNamespace,
	}, actual))
	host, _, err := unstructured.NestedString(actual.Object, "spec", "controlPlaneEndpoint", "host")
	require.NoError(t, err)
	assert.Equal(t, "192.0.2.10", host)
	subnets, _, err := unstructured.NestedStringSlice(actual.Object, "spec", "managedSubnets")
	require.NoError(t, err)
	assert.Equal(t, []string{"new"}, subnets)
}

func TestRemoveStaleProviderCredentials(t *testing.T) {
	managedSecret := func(name string) *corev1.Secret {
		return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: capiNamespace,
			Labels: map[string]string{capiClusterCredentialsLabel: capiClusterCredentialsManaged},
		}}
	}
	current := managedSecret("current-credentials")
	stale := managedSecret("stale-credentials")
	unrelated := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "unrelated", Namespace: capiNamespace}}
	base := fakeReconciler(t, current, stale, unrelated)
	reconciler := &ClusterReconciler{BaseWithReader: base.BaseWithReader}
	reconciler.APIReader = reconciler.Client

	require.NoError(t, reconciler.removeStaleProviderCredentials(t.Context(), current.Name))
	require.NoError(t, reconciler.Client.Get(t.Context(), client.ObjectKeyFromObject(current), &corev1.Secret{}))
	require.NoError(t, reconciler.Client.Get(t.Context(), client.ObjectKeyFromObject(unrelated), &corev1.Secret{}))
	err := reconciler.Client.Get(t.Context(), client.ObjectKeyFromObject(stale), &corev1.Secret{})
	require.True(t, apierrors.IsNotFound(err))

	require.NoError(t, reconciler.removeStaleProviderCredentials(t.Context(), ""))
	err = reconciler.Client.Get(t.Context(), client.ObjectKeyFromObject(current), &corev1.Secret{})
	require.True(t, apierrors.IsNotFound(err))
}

func TestEnsureCloudClusterUsesSharedProviderContract(t *testing.T) {
	registration := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: common.CloudProviderSecretName, Namespace: common.CloudProviderSecretNamespace,
		},
		Data: map[string][]byte{
			"type":                            []byte("example"),
			"capiClusterName":                 []byte("example"),
			"capiClusterKind":                 []byte("ExampleCluster"),
			"capiClusterAPIVersion":           []byte("infrastructure.cluster.x-k8s.io/v1alpha1"),
			"capiMachineTemplateKind":         []byte("ExampleMachineTemplate"),
			"capiMachineTemplateAPIVersion":   []byte("infrastructure.cluster.x-k8s.io/v1alpha1"),
			common.InstanceClassAPIVersionKey: []byte("v1alpha1"),
			"example":                         []byte(`{"region":"test"}`),
		},
	}
	clusterConfiguration := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: common.ClusterConfigSecretName, Namespace: common.ClusterConfigSecretNamespace,
		},
		Data: map[string][]byte{"cluster-configuration.yaml": []byte(`
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
clusterDomain: cluster.local
cloud:
  prefix: test
`)},
	}
	clusterUUID := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: clusterUUIDConfigMapName, Namespace: clusterUUIDConfigMapNS},
		Data:       map[string]string{"cluster-uuid": "test-uuid"},
	}
	providerTemplates := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "d8-cloud-provider-example-capi", Namespace: cloudprovider.ProviderTemplateSecretNamespace,
		},
		Data: map[string][]byte{
			cloudprovider.CAPIClusterTemplateKey: []byte(`version: v1
template: |
  apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
  kind: ExampleCluster
  metadata:
    name: {{ .cluster.name }}
  spec:
    region: {{ .provider.region }}
    prefix: {{ .prefix }}
`),
			cloudprovider.CAPICredentialsTemplateKey: []byte(`version: v1
template: |
  apiVersion: v1
  kind: Secret
  metadata:
    name: example-credentials
  data:
    region: {{ .provider.region | b64enc }}
`),
		},
	}

	base := fakeReconciler(t, registration, clusterConfiguration, clusterUUID, providerTemplates)
	reconciler := &ClusterReconciler{BaseWithReader: base.BaseWithReader}
	reconciler.APIReader = reconciler.Client

	configuration, err := common.ReadClusterConfiguration(t.Context(), reconciler.Client)
	require.NoError(t, err)
	require.NoError(t, reconciler.ensureCloudCluster(t.Context(), configuration))

	credentials := &corev1.Secret{}
	require.NoError(t, reconciler.Client.Get(t.Context(), types.NamespacedName{
		Name: "example-credentials", Namespace: capiNamespace,
	}, credentials))
	assert.Equal(t, []byte("test"), credentials.Data["region"])

	managedObjects := []*unstructured.Unstructured{
		clusterHandoverTestObject("infrastructure.cluster.x-k8s.io/v1alpha1", "ExampleCluster", capiNamespace, "example"),
		clusterHandoverTestObject("infrastructure.cluster.x-k8s.io/v1alpha1", "DeckhouseControlPlane", capiNamespace, "example-control-plane"),
		clusterHandoverTestObject("cluster.x-k8s.io/v1beta2", "Cluster", capiNamespace, "example"),
		clusterHandoverTestObject("cluster.x-k8s.io/v1beta2", "MachineHealthCheck", capiNamespace, "example-machine-health-check"),
	}
	for _, object := range managedObjects {
		require.NoError(t, reconciler.Client.Get(t.Context(), client.ObjectKeyFromObject(object), object))
	}

	for _, object := range managedObjects {
		require.NoError(t, reconciler.Client.Delete(t.Context(), object))
	}
	require.NoError(t, reconciler.Client.Delete(t.Context(), credentials))
	require.NoError(t, reconciler.ensureCloudCluster(t.Context(), configuration))
	require.NoError(t, reconciler.Client.Get(t.Context(), types.NamespacedName{
		Name: "example-credentials", Namespace: capiNamespace,
	}, credentials))
	for _, object := range managedObjects {
		require.NoError(t, reconciler.Client.Get(t.Context(), client.ObjectKeyFromObject(object), object))
	}

	currentTemplates := &corev1.Secret{}
	require.NoError(t, reconciler.Client.Get(t.Context(), client.ObjectKeyFromObject(providerTemplates), currentTemplates))
	currentTemplates.Data[cloudprovider.CAPICredentialsTemplateKey] = []byte(`version: v1
template: |
  apiVersion: v1
  kind: Secret
  metadata:
    name: example-credentials
  data:
    region: {{ "changed" | b64enc }}
`)
	currentTemplates.Data[cloudprovider.CAPIClusterTemplateKey] = []byte(`version: v1
template: |
  apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
  kind: ExampleCluster
  metadata:
    name: wrong-name
`)
	require.NoError(t, reconciler.Client.Update(t.Context(), currentTemplates))
	require.ErrorContains(t, reconciler.ensureCloudCluster(t.Context(), configuration), "registration declares")
	require.NoError(t, reconciler.Client.Get(t.Context(), types.NamespacedName{
		Name: "example-credentials", Namespace: capiNamespace,
	}, credentials))
	assert.Equal(t, []byte("test"), credentials.Data["region"], "invalid infrastructure must not partially update credentials")
}

func TestDeckhouseControlPlaneObject(t *testing.T) {
	controlPlane := deckhouseControlPlane("openstack")
	assert.Equal(t, "infrastructure.cluster.x-k8s.io/v1alpha1", controlPlane.GetAPIVersion())
	assert.Equal(t, "DeckhouseControlPlane", controlPlane.GetKind())
	assert.Equal(t, "openstack-control-plane", controlPlane.GetName())
	assert.Equal(t, capiNamespace, controlPlane.GetNamespace())
}
