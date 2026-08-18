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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))
	return scheme
}

func registrationSecret(name string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: SecretNamespace,
			Name:      name,
			Labels:    map[string]string{SecretLabel: ""},
		},
		Data: data,
	}
}

func clusterConfigurationSecret(provider string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: SecretNamespace, Name: clusterConfigSecretName},
		Data: map[string][]byte{
			clusterConfigSecretKey: []byte("cloud:\n  provider: " + provider + "\n  prefix: test\n"),
		},
	}
}

func loadFrom(t *testing.T, objs ...client.Object) Providers {
	t.Helper()
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(objs...).Build()
	providers, err := Load(context.Background(), c)
	require.NoError(t, err)
	return providers
}

func cloudEphemeral(name, kind string) *v1.NodeGroup {
	return &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudEphemeral,
			CloudInstances: &v1.CloudInstancesSpec{
				ClassReference: v1.ClassReference{Kind: kind, Name: name},
			},
		},
	}
}

func nodeGroupOfType(name string, nodeType v1.NodeType) *v1.NodeGroup {
	return &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       v1.NodeGroupSpec{NodeType: nodeType},
	}
}

// Every provider module renders its registration twice — under the legacy fixed name and under a
// per-provider one. Both carry the label, so a providers that did not deduplicate would report one
// provider as two and make every "exactly one provider" check fail on a perfectly normal cluster.
func TestLoad_DeduplicatesTheLegacyAndPerProviderCopies(t *testing.T) {
	data := map[string][]byte{
		"type":                    []byte("yandex"),
		"instanceClassKind":       []byte("YandexInstanceClass"),
		"instanceClassAPIVersion": []byte("v1"),
	}

	providers := loadFrom(t,
		registrationSecret(SecretNamePrefix, data),
		registrationSecret(SecretNamePrefix+"-yandex", data),
	)

	require.Len(t, providers.All(), 1)
	assert.Equal(t, "yandex", providers.All()[0].Type)
}

// Selection is by label, not by name: the per-provider Secret of a second provider is the whole
// point of this package, and a Secret that carries no registration label is not one.
func TestLoad_SelectsByLabelAndSeesEveryProvider(t *testing.T) {
	unlabelled := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: SecretNamespace, Name: "some-other-secret"},
		Data:       map[string][]byte{"type": []byte("notaprovider")},
	}

	providers := loadFrom(t,
		registrationSecret(SecretNamePrefix+"-aws", map[string][]byte{"type": []byte("aws")}),
		registrationSecret(SecretNamePrefix+"-yandex", map[string][]byte{"type": []byte("yandex")}),
		unlabelled,
	)

	require.Len(t, providers.All(), 2)
	assert.Equal(t, "aws", providers.All()[0].Type)
	assert.Equal(t, "yandex", providers.All()[1].Type)
}

// An unreadable provider must not read as "no cloud provider": an empty one publishes
// NodeGroups without instanceClass, which shifts the configuration checksum of every node. This is
// why Load returns the error rather than an empty providers — the callers abort the reconcile.
func TestLoad_ForbiddenListIsAnError(t *testing.T) {
	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
				return apierrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, "", context.Canceled)
			},
		}).
		Build()

	_, err := Load(context.Background(), c)

	require.ErrorContains(t, err, "list cloud provider registration secrets")
}

// A cluster with no cloud provider at all is a legitimate state, not a failure.
func TestLoad_NoProvidersIsEmptyNotAnError(t *testing.T) {
	providers := loadFrom(t)

	assert.True(t, providers.Empty())
	assert.Empty(t, providers.All())
}

// A cloud cluster whose provider cannot be read would publish its CloudPermanent NodeGroups without
// one, stripping the provider steps from the master's bundle — so it is an error, not an empty name.
func TestLoad_UnreadableClusterProviderIsAnError(t *testing.T) {
	clusterConfig := func(data map[string][]byte) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: SecretNamespace, Name: clusterConfigSecretName},
			Data:       data,
		}
	}

	tests := []struct {
		name    string
		secret  *corev1.Secret
		wantErr string
	}{
		{
			name:    "the configuration key is missing",
			secret:  clusterConfig(map[string][]byte{"other.yaml": []byte("{}")}),
			wantErr: `has no "cluster-configuration.yaml" key`,
		},
		{
			name: "the document does not parse",
			secret: clusterConfig(map[string][]byte{
				clusterConfigSecretKey: []byte("cloud:\n\tprovider: [Yandex\n"),
			}),
			wantErr: `unmarshal "cluster-configuration.yaml"`,
		},
		{
			// The schema requires cloud.provider whenever clusterType is Cloud.
			name: "a cloud cluster names no provider",
			secret: clusterConfig(map[string][]byte{
				clusterConfigSecretKey: []byte("clusterType: Cloud\ncloud:\n  prefix: test\n"),
			}),
			wantErr: "names no cloud.provider",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(tc.secret).Build()

			_, err := Load(context.Background(), c)

			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

// The cluster names a provider that published nothing: CloudPermanent resolves through that name
// and nothing else, so the master would render without provider steps and no one would say why.
func TestLoad_ClusterProviderWithoutRegistrationIsAnError(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(
		registrationSecret(SecretNamePrefix+"-aws", map[string][]byte{"type": []byte("aws")}),
		clusterConfigurationSecret("Yandex"),
	).Build()

	_, err := Load(context.Background(), c)

	require.ErrorContains(t, err, `"yandex" of the cluster configuration published no registration`)
}

// A static cluster names no provider, and that is not a failure.
func TestLoad_StaticClusterHasNoProvider(t *testing.T) {
	providers := loadFrom(t, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: SecretNamespace, Name: clusterConfigSecretName},
		Data: map[string][]byte{
			clusterConfigSecretKey: []byte("clusterType: Static\npodSubnetCIDR: 10.111.0.0/16\n"),
		},
	})

	_, ok := providers.ForNodeGroup(nodeGroupOfType("master", v1.NodeTypeCloudPermanent))
	assert.False(t, ok)
}

func TestForNodeGroup(t *testing.T) {
	aws := registrationSecret(SecretNamePrefix+"-aws", map[string][]byte{
		"type":              []byte("aws"),
		"instanceClassKind": []byte("AWSInstanceClass"),
	})
	yandex := registrationSecret(SecretNamePrefix+"-yandex", map[string][]byte{
		"type":              []byte("yandex"),
		"instanceClassKind": []byte("YandexInstanceClass"),
	})
	providers := loadFrom(t, aws, yandex, clusterConfigurationSecret("Yandex"))

	tests := []struct {
		name     string
		ng       *v1.NodeGroup
		expType  string
		expFound bool
	}{
		{
			name:     "CloudEphemeral resolves through the InstanceClass kind it references",
			ng:       cloudEphemeral("worker-aws", "AWSInstanceClass"),
			expType:  "aws",
			expFound: true,
		},
		{
			name:     "a second provider in the same cluster resolves to itself",
			ng:       cloudEphemeral("worker-yandex", "YandexInstanceClass"),
			expType:  "yandex",
			expFound: true,
		},
		{
			// The provider is what decides which kinds exist; an unknown kind is a verdict about
			// the NodeGroup, which derived_status.Validate reports.
			name: "CloudEphemeral referencing a kind nobody registered resolves to nothing",
			ng:   cloudEphemeral("worker", "VsphereInstanceClass"),
		},
		{
			// CloudPermanent nodes are created by the installer and reference no InstanceClass, so
			// the cluster configuration is the only thing left to name their provider.
			name:     "CloudPermanent falls back to the cluster provider",
			ng:       nodeGroupOfType("master", v1.NodeTypeCloudPermanent),
			expType:  "yandex",
			expFound: true,
		},
		{
			// Static groups were rendered with the cluster's provider before providers became
			// per-NodeGroup, and their nodes carry what those steps installed. Dropping the
			// provider here is what would take it away from them.
			name:     "Static falls back to the cluster provider",
			ng:       nodeGroupOfType("static", v1.NodeTypeStatic),
			expType:  "yandex",
			expFound: true,
		},
		{
			// CloudStatic nodes do run in the cluster's cloud, Deckhouse just does not order them:
			// they still need the provider steps and the cloud variables.
			name:     "CloudStatic falls back to the cluster provider",
			ng:       nodeGroupOfType("cloudstatic", v1.NodeTypeCloudStatic),
			expType:  "yandex",
			expFound: true,
		},
		{
			// A CloudEphemeral group is defined by the machines it orders, and it orders them from
			// the InstanceClass. Without one there is no cloud to fall back to.
			name: "CloudEphemeral without a classReference resolves to nothing",
			ng:   nodeGroupOfType("worker", v1.NodeTypeCloudEphemeral),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := providers.ForNodeGroup(tc.ng)

			assert.Equal(t, tc.expFound, ok)
			assert.Equal(t, tc.expType, got.Type)
		})
	}
}

// A registration that published no type is kept on load for the InstanceClass kind it carries, so
// its Type is the empty string — the same empty string a NodeGroup has when it named no provider
// and the cluster names none either. The two must not meet: a static cluster with a registration
// left behind by a disabled provider module would otherwise hand every group a nameless provider.
func TestForNodeGroup_NoNameMatchesNoRegistration(t *testing.T) {
	providers := loadFrom(t, registrationSecret(SecretNamePrefix, map[string][]byte{
		"instanceClassKind": []byte("AWSInstanceClass"),
	}))

	_, ok := providers.ForNodeGroup(nodeGroupOfType("static", v1.NodeTypeStatic))

	assert.False(t, ok)
}

// ClusterConfiguration spells providers OpenStack and vSphere; their Secrets spell them lower case.
func TestForNodeGroup_ClusterProviderMatchesCaseInsensitively(t *testing.T) {
	providers := loadFrom(t,
		registrationSecret(SecretNamePrefix, map[string][]byte{"type": []byte("openstack")}),
		clusterConfigurationSecret("OpenStack"),
	)

	got, ok := providers.ForNodeGroup(nodeGroupOfType("master", v1.NodeTypeCloudPermanent))

	require.True(t, ok)
	assert.Equal(t, "openstack", got.Type)
}

// A provider that names a kind but no version contributes no GVK: guessing a version renames
// the immutable MachineTemplate the instance-class checksum points at.
func TestInstanceClassGVKs_SkipsProvidersWithoutAVersion(t *testing.T) {
	providers := loadFrom(t,
		registrationSecret(SecretNamePrefix+"-aws", map[string][]byte{
			"type":                    []byte("aws"),
			"instanceClassKind":       []byte("AWSInstanceClass"),
			"instanceClassAPIVersion": []byte("v1"),
		}),
		registrationSecret(SecretNamePrefix+"-yandex", map[string][]byte{
			"type":              []byte("yandex"),
			"instanceClassKind": []byte("YandexInstanceClass"),
		}),
	)

	gvks := providers.InstanceClassGVKs()

	require.Len(t, gvks, 1)
	assert.Equal(t, schema.GroupVersionKind{
		Group: v1.GroupVersion.Group, Version: "v1", Kind: "AWSInstanceClass",
	}, gvks[0])
}
