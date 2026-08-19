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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/cloudprovider"
	ngcommon "github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
)

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	scheme.AddKnownTypeWithName(ngcommon.MCMMachineDeploymentGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(ngcommon.MCMMachineDeploymentGVK.GroupVersion().WithKind("MachineDeploymentList"), &unstructured.UnstructuredList{})
	return scheme
}

func newTestService(t *testing.T, objs ...client.Object) *Service {
	t.Helper()
	if !hasClusterKubernetesConfigMap(objs) {
		objs = append([]client.Object{kubernetesSourceConfigMap("1.32")}, objs...)
	}
	return newTestServiceRaw(t, objs...)
}

func newTestServiceRaw(t *testing.T, objs ...client.Object) *Service {
	t.Helper()
	c := fake.NewClientBuilder().WithScheme(newTestScheme(t)).WithObjects(objs...).Build()
	return &Service{Client: c}
}

func hasClusterKubernetesConfigMap(objs []client.Object) bool {
	for _, obj := range objs {
		cm, ok := obj.(*corev1.ConfigMap)
		if ok && cm.Name == clusterKubernetesConfigMapName && cm.Namespace == clusterConfigSecretNamespace {
			return true
		}
	}
	return false
}

func testSecret(ns, name string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Data:       data,
	}
}

// providerSecret builds a registration the way a provider module publishes one: labelled, because
// the label is how it is found at all.
func providerSecret(data map[string][]byte) *corev1.Secret {
	secret := testSecret(cloudprovider.SecretNamespace, cloudprovider.SecretNamePrefix, data)
	secret.Labels = map[string]string{cloudprovider.SecretLabel: ""}
	return secret
}

func testRegistry(t *testing.T, s *Service) cloudprovider.Providers {
	t.Helper()
	providers, err := cloudprovider.Load(context.Background(), s.Client)
	require.NoError(t, err)
	return providers
}

// An unpublished version must reach the operator as a NodeGroup validation error rather than as a
// reconcile error: every consumer already handles a validation error (rendering is skipped, the
// bashible context keeps its previous entry), whereas a reconcile error stops the whole pass and
// freezes the status of every NodeGroup, Static ones included.
func TestRunCloudChecks_UnpublishedAPIVersionIsAValidationError(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudEphemeral,
			CloudInstances: &v1.CloudInstancesSpec{
				ClassReference: v1.ClassReference{Kind: "YandexInstanceClass", Name: "worker"},
			},
		},
	}

	check := Validate(ng, Snapshot{
		Provider: cloudprovider.Provider{InstanceClassKind: "YandexInstanceClass"},
	})

	assert.Contains(t, check.Error, "has not published instanceClassAPIVersion")
	assert.False(t, check.Processed)
}

func TestReadStatic_ParsesInternalNetworkCIDRs(t *testing.T) {
	s := newTestService(t, testSecret(staticConfigSecretNamespace, staticConfigSecretName, map[string][]byte{
		staticConfigKey: []byte("apiVersion: deckhouse.io/v1\nkind: StaticClusterConfiguration\ninternalNetworkCIDRs:\n- 172.18.200.0/24\n"),
	}))
	got, err := s.readStatic(context.Background())
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{
		"internalNetworkCIDRs": []interface{}{"172.18.200.0/24"},
	}, got)
}

func TestReadStatic_AbsentReturnsNil(t *testing.T) {
	got, err := newTestService(t).readStatic(context.Background())
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestReadDefaultZonesIncludesExistingMCMMachineDeploymentZones(t *testing.T) {
	md := &unstructured.Unstructured{}
	md.SetGroupVersionKind(ngcommon.MCMMachineDeploymentGVK)
	md.SetName("worker-a")
	md.SetNamespace(ngcommon.MachineNamespace)
	md.SetAnnotations(map[string]string{"zone": "zone-a"})

	s := newTestService(t, md)
	got, err := s.readDefaultZones(context.Background(), cloudprovider.Provider{Zones: []string{"zone-b", "zone-a"}})
	require.NoError(t, err)

	assert.Equal(t, []string{"zone-a", "zone-b"}, got)
}

func TestResolveNodeGroup_StaticWiresNameRolloutAndStatic(t *testing.T) {
	s := newTestService(t, testSecret(staticConfigSecretNamespace, staticConfigSecretName, map[string][]byte{
		staticConfigKey: []byte("internalNetworkCIDRs:\n- 172.18.200.0/24\n"),
	}))
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "static1",
			Annotations: map[string]string{manualRolloutIDAnnotation: "test"},
		},
		Spec: v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}

	resolved, errStr, err := s.ResolveNodeGroup(context.Background(), ng, testRegistry(t, s))
	require.NoError(t, err)
	assert.Empty(t, errStr)
	assert.Equal(t, "static1", resolved.Name)
	assert.Equal(t, "test", resolved.ManualRolloutID)
	assert.Equal(t, v1.NodeTypeStatic, resolved.NodeType)
	assert.Equal(t, map[string]interface{}{
		"internalNetworkCIDRs": []interface{}{"172.18.200.0/24"},
	}, resolved.Static)
	assert.NotContains(t, resolved.ToMap(), "instanceClass", "static NG must not receive cloud overlays")
}

func TestResolveNodeGroup_CloudKindMismatchErrors(t *testing.T) {
	s := newTestService(t, providerSecret(map[string][]byte{
		"type":                    []byte(`yandex`),
		"instanceClassKind":       []byte(`"YandexInstanceClass"`),
		"instanceClassAPIVersion": []byte("v1alpha1"),
	}))
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudEphemeral,
			CloudInstances: &v1.CloudInstancesSpec{
				ClassReference: v1.ClassReference{Kind: "AWSInstanceClass", Name: "worker"},
			},
		},
	}

	resolved, errStr, err := s.ResolveNodeGroup(context.Background(), ng, testRegistry(t, s))
	require.NoError(t, err)
	assert.Contains(t, errStr, "Invalid classReference.kind 'AWSInstanceClass'. Expected 'YandexInstanceClass'.")
	assert.NotContains(t, resolved.ToMap(), "instanceClass", "failed check must drop cloud overlays")
}
