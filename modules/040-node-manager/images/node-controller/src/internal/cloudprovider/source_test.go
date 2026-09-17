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

package cloudprovider

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/node-controller/internal/common"
)

type countingProviderReader struct {
	client.Reader
	clusterConfigurationReads int
}

func (r *countingProviderReader) Get(
	ctx context.Context,
	key client.ObjectKey,
	object client.Object,
	opts ...client.GetOption,
) error {
	if key.Namespace == "kube-system" && key.Name == "d8-cluster-configuration" {
		r.clusterConfigurationReads++
	}
	return r.Reader.Get(ctx, key, object, opts...)
}

func providerSourceTestClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

func providerSourceTestObjects(clusterTemplate string) []client.Object {
	return []client.Object{
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: commonCloudProviderSecretName(), Namespace: "kube-system"},
			Data: map[string][]byte{
				"type": []byte("openstack"), "region": []byte("region-one"), "zones": []byte(`["zone-a"]`),
				"instanceClassKind": []byte("OpenStackInstanceClass"), "instanceClassAPIVersion": []byte("v1"),
				"machineClassKind": []byte("OpenStackMachineClass"),
				"capiClusterName":  []byte("openstack"), "capiClusterKind": []byte("OpenStackCluster"),
				"capiClusterAPIVersion":         []byte("infrastructure.cluster.x-k8s.io/v1beta1"),
				"capiMachineTemplateKind":       []byte("OpenStackMachineTemplate"),
				"capiMachineTemplateAPIVersion": []byte("infrastructure.cluster.x-k8s.io/v1beta1"),
				"openstack":                     []byte(`{"region":"region-one"}`),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "d8-cluster-configuration", Namespace: "kube-system"},
			Data:       map[string][]byte{"cluster-configuration.yaml": []byte("podSubnetCIDR: 10.111.0.0/16\ncloud:\n  prefix: prod\n")},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: clusterUUIDConfigMapName, Namespace: "kube-system"},
			Data:       map[string]string{clusterUUIDConfigMapKey: "uuid"},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "d8-cloud-provider-openstack-capi", Namespace: "kube-system"},
			Data: map[string][]byte{
				CAPIMachineTemplateKey: []byte("version: v2\nrolloutFields: [flavor]\ntemplate: |\n  apiVersion: infrastructure.cluster.x-k8s.io/v1beta1\n  kind: OpenStackMachineTemplate\n  spec: {}\n"),
				CAPIClusterTemplateKey: []byte(clusterTemplate),
			},
		},
	}
}

func TestSourceLoadsMachineAndClusterIndependently(t *testing.T) {
	t.Run("invalid cluster template does not block machines", func(t *testing.T) {
		objects := providerSourceTestObjects("version: v9\ntemplate: invalid\n")
		source := Source{Reader: providerSourceTestClient(t, objects...)}
		provider, err := source.Load(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "openstack", provider.Registration.Type)
		assert.Equal(t, "uuid", provider.Cluster.UUID)
		assert.Equal(t, "prod", provider.Prefix)

		machine, err := source.LoadCAPIMachineInputs(t.Context(), provider)
		require.NoError(t, err)
		require.NotNil(t, machine.Template)

		_, err = source.LoadCAPIClusterInputs(t.Context(), provider)
		require.Error(t, err)
	})

	t.Run("invalid machine template does not block cluster", func(t *testing.T) {
		objects := providerSourceTestObjects("version: v1\ntemplate: |\n  apiVersion: infrastructure.cluster.x-k8s.io/v1beta1\n  kind: OpenStackCluster\n  metadata: {name: openstack}\n")
		objects[3].(*corev1.Secret).Data[CAPIMachineTemplateKey] = []byte("version: v9\ntemplate: invalid\n")
		source := Source{Reader: providerSourceTestClient(t, objects...)}
		provider, err := source.Load(t.Context())
		require.NoError(t, err)

		cluster, err := source.LoadCAPIClusterInputs(t.Context(), provider)
		require.NoError(t, err)
		require.NotNil(t, cluster.Cluster)

		_, err = source.LoadCAPIMachineInputs(t.Context(), provider)
		require.Error(t, err)
	})
}

func TestSourceReturnsErrNoCloudProvider(t *testing.T) {
	source := Source{Reader: providerSourceTestClient(t)}
	_, err := source.Load(t.Context())
	require.True(t, errors.Is(err, ErrNoCloudProvider))
}

func TestSourceReadsClusterConfigurationOnce(t *testing.T) {
	reader := &countingProviderReader{Reader: providerSourceTestClient(t, providerSourceTestObjects(`version: v1
template: |
  apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
  kind: OpenStackCluster
  metadata: {name: openstack}
`)...)}

	_, err := (Source{Reader: reader}).Load(t.Context())

	require.NoError(t, err)
	assert.Equal(t, 1, reader.clusterConfigurationReads)
}

func TestSourceUsesProvidedClusterConfiguration(t *testing.T) {
	reader := &countingProviderReader{Reader: providerSourceTestClient(t, providerSourceTestObjects(`version: v1
template: |
  apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
  kind: OpenStackCluster
  metadata: {name: openstack}
`)...)}
	configuration := common.ClusterConfiguration{PodSubnetCIDR: "10.111.0.0/16"}
	configuration.Cloud.Prefix = "provided"

	provider, err := (Source{Reader: reader}).LoadWithClusterConfiguration(t.Context(), configuration)

	require.NoError(t, err)
	assert.Equal(t, 0, reader.clusterConfigurationReads)
	assert.Equal(t, "provided", provider.Prefix)
}

func TestSourcePrefersModuleConfigPrefix(t *testing.T) {
	moduleConfig := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModuleConfig",
		"metadata":   map[string]any{"name": "global"},
		"spec":       map[string]any{"settings": map[string]any{"prefix": "from-module-config"}},
	}}
	objects := append(providerSourceTestObjects(`version: v1
template: |
  apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
  kind: OpenStackCluster
  metadata: {name: openstack}
`), moduleConfig)

	provider, err := (Source{Reader: providerSourceTestClient(t, objects...)}).Load(t.Context())

	require.NoError(t, err)
	assert.Equal(t, "from-module-config", provider.Prefix)
}

func TestSourceValidatesCommonInputs(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([]client.Object) []client.Object
		error  string
	}{
		{
			name: "provider type",
			mutate: func(objects []client.Object) []client.Object {
				objects[0].(*corev1.Secret).Data["type"] = nil
				return objects
			},
			error: "type",
		},
		{
			name: "region",
			mutate: func(objects []client.Object) []client.Object {
				objects[0].(*corev1.Secret).Data["region"] = nil
				return objects
			},
			error: "region",
		},
		{
			name: "blank zone name",
			mutate: func(objects []client.Object) []client.Object {
				objects[0].(*corev1.Secret).Data["zones"] = []byte(`[""]`)
				return objects
			},
			error: "zones",
		},
		{
			name: "instance class kind",
			mutate: func(objects []client.Object) []client.Object {
				objects[0].(*corev1.Secret).Data["instanceClassKind"] = nil
				return objects
			},
			error: "instanceClassKind",
		},
		{
			name: "instance class API version",
			mutate: func(objects []client.Object) []client.Object {
				objects[0].(*corev1.Secret).Data["instanceClassAPIVersion"] = nil
				return objects
			},
			error: "instanceClassAPIVersion",
		},
		{
			name: "provider subtree",
			mutate: func(objects []client.Object) []client.Object {
				delete(objects[0].(*corev1.Secret).Data, "openstack")
				return objects
			},
			error: "provider subtree openstack",
		},
		{
			name: "cluster configuration",
			mutate: func(objects []client.Object) []client.Object {
				return append(objects[:1], objects[2:]...)
			},
			error: "get cluster-configuration secret",
		},
		{
			name: "pod subnet",
			mutate: func(objects []client.Object) []client.Object {
				objects[1].(*corev1.Secret).Data["cluster-configuration.yaml"] = []byte("cloud: {}\n")
				return objects
			},
			error: "no podSubnetCIDR",
		},
		{
			name: "cluster UUID",
			mutate: func(objects []client.Object) []client.Object {
				objects[2].(*corev1.ConfigMap).Data[clusterUUIDConfigMapKey] = ""
				return objects
			},
			error: "no cluster-uuid",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			objects := testCase.mutate(providerSourceTestObjects("version: v1\ntemplate: |\n  apiVersion: infrastructure.cluster.x-k8s.io/v1beta1\n  kind: OpenStackCluster\n  metadata: {name: openstack}\n"))
			_, err := (Source{Reader: providerSourceTestClient(t, objects...)}).Load(t.Context())
			require.ErrorContains(t, err, testCase.error)
		})
	}
}

// An empty zone list is a normal state, not a broken registration: OpenStack in hybrid mode
// publishes none until cloud-data discovery has run, and the CAPI reconciler skips the affected
// NodeGroups on its own. Rejecting the registration here would take down every other provider path.
func TestLoadAcceptsEmptyZones(t *testing.T) {
	objects := providerSourceTestObjects("version: v1\ntemplate: |\n  apiVersion: infrastructure.cluster.x-k8s.io/v1beta1\n  kind: OpenStackCluster\n  metadata: {name: openstack}\n")
	objects[0].(*corev1.Secret).Data["zones"] = []byte(`[]`)

	provider, err := (Source{Reader: providerSourceTestClient(t, objects...)}).Load(t.Context())

	require.NoError(t, err)
	require.Empty(t, provider.Registration.Zones)
}

// Cleanup runs after the provider module may already be half-gone. It needs machineClassKind and
// the machine template GVK, so an otherwise incomplete registration must not stop it: a deleted
// NodeGroup would keep its finalizer forever.
func TestLoadRegistrationSkipsValidation(t *testing.T) {
	objects := providerSourceTestObjects("version: v1\ntemplate: |\n  apiVersion: infrastructure.cluster.x-k8s.io/v1beta1\n  kind: OpenStackCluster\n  metadata: {name: openstack}\n")
	secret := objects[0].(*corev1.Secret)
	secret.Data["region"] = nil
	secret.Data["zones"] = []byte(`[]`)
	delete(secret.Data, "openstack")

	registration, err := (Source{Reader: providerSourceTestClient(t, objects...)}).LoadRegistration(t.Context())

	require.NoError(t, err)
	require.Equal(t, "OpenStackMachineClass", registration.MachineClassKind)
}

func TestScopedTemplateInputsValidateOnlyTheirOwnContract(t *testing.T) {
	objects := providerSourceTestObjects("version: v1\ntemplate: |\n  apiVersion: infrastructure.cluster.x-k8s.io/v1beta1\n  kind: OpenStackCluster\n  metadata: {name: openstack}\n")
	source := Source{Reader: providerSourceTestClient(t, objects...)}
	provider, err := source.Load(t.Context())
	require.NoError(t, err)

	withoutMachineGVK := provider
	withoutMachineGVK.Registration.CAPIMachineTemplateKind = ""
	_, err = source.LoadCAPIMachineInputs(t.Context(), withoutMachineGVK)
	require.ErrorContains(t, err, "capiMachineTemplateKind")

	withoutClusterGVK := provider
	withoutClusterGVK.Registration.CAPIClusterAPIVersion = ""
	_, err = source.LoadCAPIClusterInputs(t.Context(), withoutClusterGVK)
	require.ErrorContains(t, err, "capiClusterAPIVersion")

	withoutMCM := provider
	withoutMCM.Registration.MachineClassKind = ""
	_, err = source.LoadMCMInputs(t.Context(), withoutMCM)
	require.ErrorContains(t, err, "machineClassKind")
}

func commonCloudProviderSecretName() string {
	return "d8-node-manager-cloud-provider"
}
