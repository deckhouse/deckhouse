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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
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
	c := fake.NewClientBuilder().WithScheme(newTestScheme(t)).WithObjects(objs...).Build()
	return &Service{Client: c}
}

func testSecret(ns, name string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Data:       data,
	}
}

func TestResolveInstanceClassVersion(t *testing.T) {
	t.Run("nil mapper falls back to default version", func(t *testing.T) {
		assert.Equal(t, instanceClassVersion, resolveInstanceClassVersion(nil, "VCDInstanceClass"))
	})

	t.Run("unknown kind falls back to default version", func(t *testing.T) {
		mapper := meta.NewDefaultRESTMapper(nil)
		assert.Equal(t, instanceClassVersion, resolveInstanceClassVersion(mapper, "UnknownInstanceClass"))
	})

	t.Run("v1-only kind resolves to v1", func(t *testing.T) {
		gv := schema.GroupVersion{Group: instanceClassGroup, Version: "v1"}
		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{gv})
		mapper.Add(gv.WithKind("VCDInstanceClass"), meta.RESTScopeRoot)
		assert.Equal(t, "v1", resolveInstanceClassVersion(mapper, "VCDInstanceClass"))
	})

	t.Run("multi-version kind resolves to preferred v1", func(t *testing.T) {
		v1gv := schema.GroupVersion{Group: instanceClassGroup, Version: "v1"}
		alphaGV := schema.GroupVersion{Group: instanceClassGroup, Version: "v1alpha1"}
		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{v1gv, alphaGV})
		mapper.Add(v1gv.WithKind("YandexInstanceClass"), meta.RESTScopeRoot)
		mapper.Add(alphaGV.WithKind("YandexInstanceClass"), meta.RESTScopeRoot)
		assert.Equal(t, "v1", resolveInstanceClassVersion(mapper, "YandexInstanceClass"))
	})
}

func TestReadStatic_ParsesInternalNetworkCIDRs(t *testing.T) {
	s := newTestService(t, testSecret(staticConfigSecretNamespace, staticConfigSecretName, map[string][]byte{
		staticConfigKey: []byte("apiVersion: deckhouse.io/v1\nkind: StaticClusterConfiguration\ninternalNetworkCIDRs:\n- 172.18.200.0/24\n"),
	}))
	got := s.readStatic(context.Background())
	assert.Equal(t, map[string]interface{}{
		"internalNetworkCIDRs": []interface{}{"172.18.200.0/24"},
	}, got)
}

func TestReadStatic_AbsentReturnsNil(t *testing.T) {
	assert.Nil(t, newTestService(t).readStatic(context.Background()))
}

func TestReadDefaultZonesIncludesExistingMCMMachineDeploymentZones(t *testing.T) {
	md := &unstructured.Unstructured{}
	md.SetGroupVersionKind(ngcommon.MCMMachineDeploymentGVK)
	md.SetName("worker-a")
	md.SetNamespace(ngcommon.MachineNamespace)
	md.SetAnnotations(map[string]string{"zone": "zone-a"})

	s := newTestService(t, md)
	got := s.readDefaultZones(context.Background(), map[string]interface{}{
		"zones": []interface{}{"zone-b", "zone-a"},
	})

	assert.Equal(t, []string{"zone-a", "zone-b"}, got)
}

func TestBuildElement_StaticWiresNameRolloutAndStatic(t *testing.T) {
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
	rawSpec := map[string]interface{}{"nodeType": "Static"}

	element, errStr, err := s.BuildElement(context.Background(), ng, rawSpec)
	require.NoError(t, err)
	assert.Empty(t, errStr)
	assert.Equal(t, "static1", element["name"])
	assert.Equal(t, "test", element["manualRolloutID"])
	assert.Equal(t, "Static", element["nodeType"])
	assert.Equal(t, map[string]interface{}{
		"internalNetworkCIDRs": []interface{}{"172.18.200.0/24"},
	}, element["static"])
	assert.NotContains(t, element, "instanceClass", "static NG must not receive cloud overlays")
}

func TestBuildElement_CloudKindMismatchErrors(t *testing.T) {
	s := newTestService(t, testSecret(cloudProviderSecretNamespace, cloudProviderSecretName, map[string][]byte{
		"instanceClassKind": []byte(`"YandexInstanceClass"`),
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
	rawSpec := map[string]interface{}{
		"nodeType": "CloudEphemeral",
		"cloudInstances": map[string]interface{}{
			"classReference": map[string]interface{}{"kind": "AWSInstanceClass", "name": "worker"},
		},
	}

	element, errStr, err := s.BuildElement(context.Background(), ng, rawSpec)
	require.NoError(t, err)
	assert.Contains(t, errStr, "Invalid classReference.kind 'AWSInstanceClass'. Expected 'YandexInstanceClass'.")
	assert.NotContains(t, element, "instanceClass", "failed check must drop cloud overlays")
}
