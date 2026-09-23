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
	assert.Equal(t, "keep", adopted.Annotations["helm.sh/resource-policy"])
	// Helm's ownership metadata stays: nothing needs it removed, and the keep annotation above
	// is what stops the prune.
	assert.Equal(t, "Helm", adopted.Labels[helmManagedByLabel])
	assert.Equal(t, "node-manager", adopted.Annotations[helmReleaseNameAnnotation])
}

// The provider that renders the endpoint renders it from the apiserver addresses this controller
// discovers, so its value is the fresher one. Keeping the live value here would pin the cluster to
// the address of a master that may already be gone.
func TestApplyClusterObjectPrefersTheRenderedControlPlaneEndpoint(t *testing.T) {
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
	assert.Equal(t, "192.0.2.20", host)
	subnets, _, err := unstructured.NestedStringSlice(actual.Object, "spec", "managedSubnets")
	require.NoError(t, err)
	assert.Equal(t, []string{"new"}, subnets)
}

// The mirror case, and the one every provider except OpenStack is in: the template renders no
// endpoint because only the provider controller knows it. Applying the object must not wipe it.
func TestApplyClusterObjectPreservesAnEndpointTheTemplateDoesNotRender(t *testing.T) {
	existing := clusterHandoverTestObject(
		"infrastructure.cluster.x-k8s.io/v1beta1", "OpenStackCluster", capiNamespace, "openstack",
	)
	existing.Object["spec"] = map[string]any{
		"controlPlaneEndpoint": map[string]any{"host": "192.0.2.10", "port": int64(6443)},
	}
	base := fakeReconciler(t, existing)
	reconciler := &ClusterReconciler{BaseWithReader: base.BaseWithReader}
	reconciler.APIReader = reconciler.Client

	desired := clusterHandoverTestObject(
		"infrastructure.cluster.x-k8s.io/v1beta1", "OpenStackCluster", capiNamespace, "openstack",
	)
	desired.Object["spec"] = map[string]any{"managedSubnets": []any{"new"}}
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
}

func TestApplyClusterObjectInitializesNullSpecBeforePreservingEndpoint(t *testing.T) {
	existing := clusterHandoverTestObject(
		"infrastructure.cluster.x-k8s.io/v1alpha1", "HuaweiCloudCluster", capiNamespace, "huaweicloud",
	)
	existing.Object["spec"] = map[string]any{
		"controlPlaneEndpoint": map[string]any{"host": "192.0.2.10", "port": int64(6443)},
	}
	base := fakeReconciler(t, existing)
	reconciler := &ClusterReconciler{BaseWithReader: base.BaseWithReader}
	reconciler.APIReader = reconciler.Client

	desired := clusterHandoverTestObject(
		"infrastructure.cluster.x-k8s.io/v1alpha1", "HuaweiCloudCluster", capiNamespace, "huaweicloud",
	)
	desired.Object["spec"] = nil
	require.NoError(t, reconciler.applyClusterObject(t.Context(), desired))

	actual := clusterHandoverTestObject(
		"infrastructure.cluster.x-k8s.io/v1alpha1", "HuaweiCloudCluster", capiNamespace, "huaweicloud",
	)
	require.NoError(t, reconciler.Client.Get(t.Context(), client.ObjectKeyFromObject(actual), actual))
	host, _, err := unstructured.NestedString(actual.Object, "spec", "controlPlaneEndpoint", "host")
	require.NoError(t, err)
	assert.Equal(t, "192.0.2.10", host)
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

	// An empty desired name means the provider stopped shipping credentials.yaml, so the Secret
	// rendered from it is stale too: nothing recreates it, and leaving it behind keeps a live
	// cloud account in the cluster. Secrets without the label stay, they are not this sweep's.
	require.NoError(t, reconciler.removeStaleProviderCredentials(t.Context(), ""))
	require.True(t, apierrors.IsNotFound(
		reconciler.Client.Get(t.Context(), client.ObjectKeyFromObject(current), &corev1.Secret{})))
	require.NoError(t, reconciler.Client.Get(t.Context(), client.ObjectKeyFromObject(unrelated), &corev1.Secret{}))
}

// exampleProviderFixture is a minimal but complete provider registration: the cluster and
// credentials templates, the cluster configuration and the UUID every render reads.
func exampleProviderFixture() (*corev1.Secret, *corev1.Secret, *corev1.ConfigMap, *corev1.Secret) {
	registration := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: cloudprovider.RegistrationSecretBaseName, Namespace: cloudprovider.RegistrationSecretNamespace,
		},
		Data: map[string][]byte{
			"type":                                   []byte("example"),
			"region":                                 []byte("test-region"),
			"zones":                                  []byte(`["test-zone"]`),
			cloudprovider.InstanceClassKindKey:       []byte("ExampleInstanceClass"),
			"capiClusterName":                        []byte("example"),
			"capiClusterKind":                        []byte("ExampleCluster"),
			"capiClusterAPIVersion":                  []byte("infrastructure.cluster.x-k8s.io/v1alpha1"),
			"capiMachineTemplateKind":                []byte("ExampleMachineTemplate"),
			"capiMachineTemplateAPIVersion":          []byte("infrastructure.cluster.x-k8s.io/v1alpha1"),
			cloudprovider.InstanceClassAPIVersionKey: []byte("v1alpha1"),
			"example":                                []byte(`{"region":"test"}`),
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
    name: capi-user-credentials
  data:
    region: {{ .provider.region | b64enc }}
`),
		},
	}

	return registration, clusterConfiguration, clusterUUID, providerTemplates
}

func TestEnsureCloudClusterUsesSharedProviderContract(t *testing.T) {
	registration, clusterConfiguration, clusterUUID, providerTemplates := exampleProviderFixture()

	base := fakeReconciler(t, registration, clusterConfiguration, clusterUUID, providerTemplates)
	reconciler := &ClusterReconciler{BaseWithReader: base.BaseWithReader}
	reconciler.APIReader = reconciler.Client

	configuration, err := common.ReadClusterConfiguration(t.Context(), reconciler.Client)
	require.NoError(t, err)
	require.NoError(t, reconciler.ensureCloudCluster(t.Context(), configuration))

	credentials := &corev1.Secret{}
	require.NoError(t, reconciler.Client.Get(t.Context(), types.NamespacedName{
		Name: cloudprovider.CAPIClusterCredentialsSecretName, Namespace: capiNamespace,
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
		Name: cloudprovider.CAPIClusterCredentialsSecretName, Namespace: capiNamespace,
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
    name: capi-user-credentials
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
	for _, object := range managedObjects[1:] {
		require.NoError(t, reconciler.Client.Delete(t.Context(), object))
	}
	require.ErrorContains(t, reconciler.ensureCloudCluster(t.Context(), configuration), "registration declares")
	for _, object := range managedObjects[1:] {
		require.NoError(t, reconciler.Client.Get(t.Context(), client.ObjectKeyFromObject(object), object),
			"a broken provider template must not block common CAPI resources")
	}
	require.NoError(t, reconciler.Client.Get(t.Context(), types.NamespacedName{
		Name: cloudprovider.CAPIClusterCredentialsSecretName, Namespace: capiNamespace,
	}, credentials))
	assert.Equal(t, []byte("test"), credentials.Data["region"], "invalid infrastructure must not partially update credentials")
}

// A provider that stops shipping capi/credentials.yaml leaves its Secret behind with a live cloud
// account in it, and nothing else in the cluster owns that Secret.
func TestEnsureCloudClusterRemovesCredentialsTheProviderStoppedShipping(t *testing.T) {
	registration, clusterConfiguration, clusterUUID, providerTemplates := exampleProviderFixture()

	base := fakeReconciler(t, registration, clusterConfiguration, clusterUUID, providerTemplates)
	reconciler := &ClusterReconciler{BaseWithReader: base.BaseWithReader}
	reconciler.APIReader = reconciler.Client

	configuration, err := common.ReadClusterConfiguration(t.Context(), reconciler.Client)
	require.NoError(t, err)
	require.NoError(t, reconciler.ensureCloudCluster(t.Context(), configuration))

	credentialsKey := types.NamespacedName{
		Name: cloudprovider.CAPIClusterCredentialsSecretName, Namespace: capiNamespace,
	}
	require.NoError(t, reconciler.Client.Get(t.Context(), credentialsKey, &corev1.Secret{}))

	currentTemplates := &corev1.Secret{}
	require.NoError(t, reconciler.Client.Get(t.Context(), client.ObjectKeyFromObject(providerTemplates), currentTemplates))
	delete(currentTemplates.Data, cloudprovider.CAPICredentialsTemplateKey)
	require.NoError(t, reconciler.Client.Update(t.Context(), currentTemplates))

	require.NoError(t, reconciler.ensureCloudCluster(t.Context(), configuration))
	require.True(t, apierrors.IsNotFound(reconciler.Client.Get(t.Context(), credentialsKey, &corev1.Secret{})),
		"the Secret rendered from a credentials.yaml that is gone must go with it")

	cluster := clusterHandoverTestObject("infrastructure.cluster.x-k8s.io/v1alpha1", "ExampleCluster", capiNamespace, "example")
	require.NoError(t, reconciler.Client.Get(t.Context(), client.ObjectKeyFromObject(cluster), cluster),
		"the infrastructure cluster does not depend on credentials.yaml")
}

func TestDeckhouseControlPlaneObject(t *testing.T) {
	controlPlane := deckhouseControlPlane("openstack")
	assert.Equal(t, "infrastructure.cluster.x-k8s.io/v1alpha1", controlPlane.GetAPIVersion())
	assert.Equal(t, "DeckhouseControlPlane", controlPlane.GetKind())
	assert.Equal(t, "openstack-control-plane", controlPlane.GetName())
	assert.Equal(t, capiNamespace, controlPlane.GetNamespace())
}
