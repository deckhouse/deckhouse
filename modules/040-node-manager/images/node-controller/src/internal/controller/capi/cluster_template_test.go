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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func clusterTemplateContractData(body string) []byte {
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	for i := range lines {
		lines[i] = "  " + lines[i]
	}
	return []byte("version: v1\ntemplate: |\n" + strings.Join(lines, "\n") + "\n")
}

func testClusterTemplateContext() clusterTemplateContext {
	return clusterTemplateContext{
		Provider: map[string]any{
			"connection": map[string]any{"username": "cloud-user"},
		},
		Cluster: clusterTemplateClusterContext{
			Name:          "openstack",
			Namespace:     capiNamespace,
			PodSubnet:     "10.111.0.0/16",
			ServiceSubnet: "10.222.0.0/16",
			Domain:        "cluster.local",
			Prefix:        "prod",
			MasterEndpoints: []map[string]interface{}{
				{"address": "192.0.2.10", "kubeApiPort": 6443},
			},
			MasterAddresses: []string{"192.0.2.10:6443"},
		},
	}
}

func TestParseClusterTemplateContract(t *testing.T) {
	contract, err := parseClusterTemplateContract(clusterTemplateContractData("apiVersion: v1\nkind: Secret\n"))
	require.NoError(t, err)
	assert.Equal(t, clusterTemplateContractVersion, contract.Version)

	tests := []struct {
		name     string
		data     string
		expected string
	}{
		{name: "unknown field", data: "version: v1\ntemplate: x\nextra: x\n", expected: `unknown field "extra"`},
		{name: "wrong version", data: "version: v2\ntemplate: x\n", expected: "unsupported cluster-template contract version"},
		{name: "empty template", data: "version: v1\ntemplate: '  '\n", expected: "empty template"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseClusterTemplateContract([]byte(tc.data))
			require.ErrorContains(t, err, tc.expected)
		})
	}
}

func TestRenderClusterTemplateExposesStableContextAndMultipleObjects(t *testing.T) {
	contract, err := parseClusterTemplateContract(clusterTemplateContractData(`apiVersion: v1
kind: Secret
metadata:
  name: capi-user-credentials
  namespace: {{ .cluster.namespace }}
stringData:
  username: {{ .provider.connection.username | quote }}
  masters: {{ join "," .cluster.masterAddresses | quote }}
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: OpenStackCluster
metadata:
  name: {{ .cluster.name }}
spec:
  prefix: {{ .cluster.prefix }}
  podSubnet: {{ .cluster.podSubnet }}
  serviceSubnet: {{ .cluster.serviceSubnet }}
  domain: {{ .cluster.domain }}
  controlPlaneHost: {{ (first .cluster.masterEndpoints).address }}`))
	require.NoError(t, err)

	objects, err := renderClusterTemplate(contract, testClusterTemplateContext())
	require.NoError(t, err)
	require.Len(t, objects, 2)

	assert.Equal(t, "cloud-user", objects[0].Object["stringData"].(map[string]interface{})["username"])
	assert.Equal(t, "192.0.2.10:6443", objects[0].Object["stringData"].(map[string]interface{})["masters"])
	assert.Equal(t, capiNamespace, objects[1].GetNamespace(), "namespace defaults to the CAPI namespace")

	spec := objects[1].Object["spec"].(map[string]interface{})
	assert.Equal(t, "prod", spec["prefix"])
	assert.Equal(t, "10.111.0.0/16", spec["podSubnet"])
	assert.Equal(t, "10.222.0.0/16", spec["serviceSubnet"])
	assert.Equal(t, "cluster.local", spec["domain"])
	assert.Equal(t, "192.0.2.10", spec["controlPlaneHost"])
}

func TestRenderClusterTemplateFailsOnUnknownContextPath(t *testing.T) {
	contract, err := parseClusterTemplateContract(clusterTemplateContractData(`apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: DeckhouseCluster
metadata:
  name: {{ .cluster.missing }}`))
	require.NoError(t, err)

	_, err = renderClusterTemplate(contract, testClusterTemplateContext())
	require.ErrorContains(t, err, `map has no entry for key "missing"`)
}

func TestDecodeClusterTemplateObjectsValidatesIdentity(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		expected string
	}{
		{name: "empty", manifest: "---\n", expected: "rendered no Kubernetes objects"},
		{name: "no apiVersion", manifest: "kind: Secret\nmetadata:\n  name: x\n", expected: "no apiVersion"},
		{name: "no kind", manifest: "apiVersion: v1\nmetadata:\n  name: x\n", expected: "no kind"},
		{name: "no name", manifest: "apiVersion: v1\nkind: Secret\n", expected: "no metadata.name"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeClusterTemplateObjects([]byte(tc.manifest))
			require.ErrorContains(t, err, tc.expected)
		})
	}
}

func clusterTemplateObject(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	object := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
	}}
	return object
}

func TestValidateClusterTemplateObjects(t *testing.T) {
	infra := func(namespace, name string) *unstructured.Unstructured {
		return clusterTemplateObject(
			"infrastructure.cluster.x-k8s.io/v1beta1",
			"OpenStackCluster",
			namespace,
			name,
		)
	}
	secret := func() *unstructured.Unstructured {
		return clusterTemplateObject("v1", "Secret", capiNamespace, "capi-user-credentials")
	}
	validate := func(objects ...*unstructured.Unstructured) error {
		return validateClusterTemplateObjects(
			objects,
			"infrastructure.cluster.x-k8s.io/v1beta1",
			"OpenStackCluster",
			"openstack",
		)
	}

	require.NoError(t, validate(secret(), infra(capiNamespace, "openstack")))

	tests := []struct {
		name     string
		objects  []*unstructured.Unstructured
		expected string
	}{
		{name: "missing infrastructure", objects: []*unstructured.Unstructured{secret()}, expected: "did not render expected"},
		{name: "wrong name", objects: []*unstructured.Unstructured{infra(capiNamespace, "other")}, expected: `must be named "openstack"`},
		{name: "wrong namespace", objects: []*unstructured.Unstructured{infra("other", "openstack")}, expected: "must be in namespace"},
		{name: "unsupported object", objects: []*unstructured.Unstructured{
			infra(capiNamespace, "openstack"),
			clusterTemplateObject("v1", "ConfigMap", capiNamespace, "extra"),
		}, expected: "cannot create v1 ConfigMap"},
		{name: "duplicate object", objects: []*unstructured.Unstructured{secret(), secret(), infra(capiNamespace, "openstack")}, expected: "duplicate object"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorContains(t, validate(tc.objects...), tc.expected)
		})
	}
}

func TestPrepareClusterTemplateObjectPreservesProviderMetadataAndProtectsHelmHandover(t *testing.T) {
	object := clusterTemplateObject(
		"infrastructure.cluster.x-k8s.io/v1beta1",
		"OpenStackCluster",
		capiNamespace,
		"openstack",
	)
	object.SetLabels(map[string]string{"app": "capo-controller-manager"})
	object.SetAnnotations(map[string]string{"provider.example/key": "value"})

	prepareClusterTemplateObject(object)

	assert.Equal(t, map[string]string{
		"app":      "capo-controller-manager",
		"heritage": "deckhouse",
		"module":   "node-manager",
	}, object.GetLabels())
	assert.Equal(t, map[string]string{
		"helm.sh/resource-policy": "keep",
		"provider.example/key":    "value",
	}, object.GetAnnotations())
}

func TestBuildClusterTemplateContextReadsRuntimeSources(t *testing.T) {
	https := "https"
	port := int32(6443)
	r := fakeReconciler(t,
		clusterConfigSecret("cloud:\n  prefix: prod\n"),
		&discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{Name: "kubernetes", Namespace: "default"},
			Ports:      []discoveryv1.EndpointPort{{Name: &https, Port: &port}},
			Endpoints:  []discoveryv1.Endpoint{{Addresses: []string{"192.0.2.10"}}},
		},
	)
	clusterReconciler := &ClusterReconciler{BaseWithReader: r.BaseWithReader}
	clusterReconciler.APIReader = clusterReconciler.Client

	context, err := clusterReconciler.buildClusterTemplateContext(
		t.Context(),
		map[string]any{"region": "region-one"},
		&clusterConfiguration{
			PodSubnetCIDR:     "10.111.0.0/16",
			ServiceSubnetCIDR: "10.222.0.0/16",
			ClusterDomain:     "cluster.local",
		},
		"openstack",
	)
	require.NoError(t, err)

	assert.Equal(t, map[string]any{"region": "region-one"}, context.Provider)
	assert.Equal(t, "openstack", context.Cluster.Name)
	assert.Equal(t, "prod", context.Cluster.Prefix)
	assert.Equal(t, []string{"192.0.2.10:6443"}, context.Cluster.MasterAddresses)
	require.Len(t, context.Cluster.MasterEndpoints, 1)
	assert.Equal(t, "192.0.2.10", context.Cluster.MasterEndpoints[0]["address"])
	assert.Equal(t, 6443, context.Cluster.MasterEndpoints[0]["kubeApiPort"])
}

func TestEnsureProviderInfrastructureRequiresClusterContract(t *testing.T) {
	providerSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "d8-cloud-provider-dvp-capi",
			Namespace: providerTemplateSecretNamespace,
		},
		Data: map[string][]byte{},
	}
	r := fakeReconciler(t, providerSecret)

	clusterReconciler := &ClusterReconciler{BaseWithReader: r.BaseWithReader}
	clusterReconciler.APIReader = clusterReconciler.Client
	err := clusterReconciler.ensureProviderInfrastructure(
		t.Context(),
		"dvp",
		map[string]any{},
		&clusterConfiguration{},
		"infrastructure.cluster.x-k8s.io/v1alpha1",
		"DeckhouseCluster",
		"dvp",
	)
	require.ErrorContains(t, err, `template "cluster.yaml" not found`)
}

func TestRemoveLegacyHelmMetadata(t *testing.T) {
	object := clusterTemplateObject("v1", "Secret", capiNamespace, "credentials")
	object.SetLabels(map[string]string{
		"app":              "provider-controller",
		helmManagedByLabel: "Helm",
	})
	object.SetAnnotations(map[string]string{
		helmReleaseNameAnnotation:      "node-manager",
		helmReleaseNamespaceAnnotation: "d8-system",
		werfFailModeAnnotation:         "IgnoreAndContinueDeployProcess",
		werfTrackTerminationAnnotation: "NonBlocking",
		"helm.sh/resource-policy":      "keep",
		"provider.example/key":         "value",
	})

	require.True(t, removeLegacyHelmMetadata(object))
	assert.Equal(t, map[string]string{"app": "provider-controller"}, object.GetLabels())
	assert.Equal(t, map[string]string{
		"helm.sh/resource-policy": "keep",
		"provider.example/key":    "value",
	}, object.GetAnnotations())
	require.False(t, removeLegacyHelmMetadata(object))
}

func TestApplyClusterObjectAdoptsHelmManagedObject(t *testing.T) {
	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "capi-user-credentials",
			Namespace: capiNamespace,
			Labels: map[string]string{
				"app":              "provider-controller",
				helmManagedByLabel: "Helm",
			},
			Annotations: map[string]string{
				helmReleaseNameAnnotation:      "node-manager",
				helmReleaseNamespaceAnnotation: "d8-system",
				werfFailModeAnnotation:         "IgnoreAndContinueDeployProcess",
				werfTrackTerminationAnnotation: "NonBlocking",
				"helm.sh/resource-policy":      "keep",
			},
		},
		Data: map[string][]byte{"token": []byte("old")},
	}
	r := fakeReconciler(t, existing)
	clusterReconciler := &ClusterReconciler{BaseWithReader: r.BaseWithReader}
	clusterReconciler.APIReader = clusterReconciler.Client

	desired := clusterTemplateObject("v1", "Secret", capiNamespace, "capi-user-credentials")
	desired.SetLabels(map[string]string{"app": "provider-controller"})
	desired.Object["data"] = map[string]interface{}{"token": "bmV3"}

	require.NoError(t, clusterReconciler.applyClusterObject(t.Context(), desired))
	adopted := &corev1.Secret{}
	require.NoError(t, clusterReconciler.Client.Get(t.Context(), types.NamespacedName{
		Name:      "capi-user-credentials",
		Namespace: capiNamespace,
	}, adopted))
	assert.Equal(t, []byte("new"), adopted.Data["token"])
	assert.Equal(t, "node-manager", adopted.Labels["module"])
	assert.NotContains(t, adopted.Labels, helmManagedByLabel)
	assert.NotContains(t, adopted.Annotations, helmReleaseNameAnnotation)
	assert.NotContains(t, adopted.Annotations, helmReleaseNamespaceAnnotation)
	assert.NotContains(t, adopted.Annotations, werfFailModeAnnotation)
	assert.NotContains(t, adopted.Annotations, werfTrackTerminationAnnotation)
	assert.Equal(t, "keep", adopted.Annotations["helm.sh/resource-policy"])
}

func TestDeckhouseControlPlaneObject(t *testing.T) {
	controlPlane := deckhouseControlPlane("openstack")
	assert.Equal(t, "infrastructure.cluster.x-k8s.io/v1alpha1", controlPlane.GetAPIVersion())
	assert.Equal(t, "DeckhouseControlPlane", controlPlane.GetKind())
	assert.Equal(t, "openstack-control-plane", controlPlane.GetName())
	assert.Equal(t, capiNamespace, controlPlane.GetNamespace())
}

func TestEnsureProviderInfrastructureRendersAndAppliesContract(t *testing.T) {
	https := "https"
	port := int32(6443)
	providerSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "d8-cloud-provider-openstack-capi",
			Namespace: providerTemplateSecretNamespace,
		},
		Data: map[string][]byte{
			clusterTemplateContractKey: clusterTemplateContractData(`apiVersion: v1
kind: Secret
metadata:
  name: capi-user-credentials
data:
  username: {{ .provider.username | b64enc | quote }}
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: OpenStackCluster
metadata:
  name: {{ .cluster.name }}
  labels:
    app: capo-controller-manager
spec:
  prefix: {{ .cluster.prefix | quote }}`),
		},
	}
	r := fakeReconciler(t,
		providerSecret,
		clusterConfigSecret("cloud:\n  prefix: prod\n"),
		&discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{Name: "kubernetes", Namespace: "default"},
			Ports:      []discoveryv1.EndpointPort{{Name: &https, Port: &port}},
			Endpoints:  []discoveryv1.Endpoint{{Addresses: []string{"192.0.2.10"}}},
		},
	)
	clusterReconciler := &ClusterReconciler{BaseWithReader: r.BaseWithReader}
	clusterReconciler.APIReader = clusterReconciler.Client

	require.NoError(t, clusterReconciler.ensureProviderInfrastructure(
		t.Context(),
		"openstack",
		map[string]any{"username": "cloud-user"},
		&clusterConfiguration{},
		"infrastructure.cluster.x-k8s.io/v1beta1",
		"OpenStackCluster",
		"openstack",
	))

	credentials := &corev1.Secret{}
	require.NoError(t, clusterReconciler.Client.Get(t.Context(), types.NamespacedName{
		Name:      "capi-user-credentials",
		Namespace: capiNamespace,
	}, credentials))
	assert.Equal(t, []byte("cloud-user"), credentials.Data["username"])
	assert.Equal(t, "keep", credentials.Annotations["helm.sh/resource-policy"])

	infra := newUnstructured("infrastructure.cluster.x-k8s.io", "v1beta1", "OpenStackCluster")
	require.NoError(t, clusterReconciler.Client.Get(t.Context(), types.NamespacedName{
		Name:      "openstack",
		Namespace: capiNamespace,
	}, infra))
	assert.Equal(t, "prod", infra.Object["spec"].(map[string]interface{})["prefix"])
	assert.Equal(t, "capo-controller-manager", infra.GetLabels()["app"])
	assert.Equal(t, "node-manager", infra.GetLabels()["module"])

	require.NoError(t, clusterReconciler.ensureProviderInfrastructure(
		t.Context(),
		"openstack",
		map[string]any{"username": "rotated-user"},
		&clusterConfiguration{},
		"infrastructure.cluster.x-k8s.io/v1beta1",
		"OpenStackCluster",
		"openstack",
	))
	require.NoError(t, clusterReconciler.Client.Get(t.Context(), types.NamespacedName{
		Name:      "capi-user-credentials",
		Namespace: capiNamespace,
	}, credentials))
	assert.Equal(t, []byte("rotated-user"), credentials.Data["username"])
}
