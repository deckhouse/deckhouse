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

func clusterConfigurationSecret(body string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: SecretNamespace, Name: clusterConfigSecretName},
		Data:       map[string][]byte{clusterConfigSecretKey: []byte(body)},
	}
}

func cloudCluster(provider string) *corev1.Secret {
	return clusterConfigurationSecret("clusterType: Cloud\ncloud:\n  provider: " + provider + "\n  prefix: test\n")
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

func TestLoad(t *testing.T) {
	aws := registrationSecret(SecretNamePrefix+"-aws", map[string][]byte{"type": []byte("aws")})
	yandexData := map[string][]byte{
		"type":                    []byte("yandex"),
		"instanceClassKind":       []byte("YandexInstanceClass"),
		"instanceClassAPIVersion": []byte("v1"),
	}
	unlabelled := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: SecretNamespace, Name: "some-other-secret"},
		Data:       map[string][]byte{"type": []byte("notaprovider")},
	}

	tests := []struct {
		name  string
		objs  []client.Object
		types []string
	}{
		{
			// Every provider module renders its registration twice — under the legacy fixed name
			// and under a per-provider one. Both carry the label, so without deduplication one
			// provider would read as two.
			name: "the legacy and the per-provider copy are one provider",
			objs: []client.Object{
				registrationSecret(SecretNamePrefix, yandexData),
				registrationSecret(SecretNamePrefix+"-yandex", yandexData),
				cloudCluster("Yandex"),
			},
			types: []string{"yandex"},
		},
		{
			// Selection is by label, not by name: the per-provider Secret of a second provider is
			// the whole point of this package, and an unlabelled Secret is not a registration.
			name: "every labelled registration is seen, ordered by type",
			objs: []client.Object{
				registrationSecret(SecretNamePrefix+"-yandex", yandexData),
				aws,
				unlabelled,
				cloudCluster("Yandex"),
			},
			types: []string{"aws", "yandex"},
		},
		{
			name: "a cluster with no cloud provider at all",
		},
		{
			name: "a static cluster names no provider",
			objs: []client.Object{clusterConfigurationSecret("clusterType: Static\npodSubnetCIDR: 10.111.0.0/16\n")},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			providers := loadFrom(t, tc.objs...)

			var got []string
			for _, p := range providers.All() {
				got = append(got, p.Type)
			}
			assert.Equal(t, tc.types, got)
			assert.Equal(t, len(tc.types) == 0, providers.Empty())
		})
	}
}

// An unreadable source must not read as "no cloud provider": an empty registry publishes
// NodeGroups without instanceClass, which shifts the configuration checksum of every node, and a
// CloudPermanent group resolves through the cluster's provider name and nothing else — so a
// configured provider that published no registration would render the master without its steps.
func TestLoad_Errors(t *testing.T) {
	tests := []struct {
		name    string
		objs    []client.Object
		deny    bool
		wantErr string
	}{
		{
			name:    "the registrations cannot be listed",
			deny:    true,
			wantErr: "list cloud provider registration secrets",
		},
		{
			name:    "the configuration key is missing",
			objs:    []client.Object{&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: SecretNamespace, Name: clusterConfigSecretName}}},
			wantErr: `has no "cluster-configuration.yaml" key`,
		},
		{
			name:    "the document does not parse",
			objs:    []client.Object{clusterConfigurationSecret("cloud:\n\tprovider: [Yandex\n")},
			wantErr: `unmarshal "cluster-configuration.yaml"`,
		},
		{
			// The schema requires cloud.provider whenever clusterType is Cloud.
			name:    "a cloud cluster names no provider",
			objs:    []client.Object{clusterConfigurationSecret("clusterType: Cloud\ncloud:\n  prefix: test\n")},
			wantErr: "names no cloud.provider",
		},
		{
			name: "the cluster provider published no registration",
			objs: []client.Object{
				registrationSecret(SecretNamePrefix+"-aws", map[string][]byte{"type": []byte("aws")}),
				cloudCluster("Yandex"),
			},
			wantErr: `"yandex" of the cluster configuration published no registration`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(tc.objs...)
			if tc.deny {
				// The last-resort tool: producing a Forbidden needs RBAC that envtest does not run.
				builder = builder.WithInterceptorFuncs(interceptor.Funcs{
					List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
						return apierrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, "", context.Canceled)
					},
				})
			}

			_, err := Load(context.Background(), builder.Build())

			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

// The cluster runs one cloud, so the node type alone decides: everything but Static belongs to the
// provider the cluster configuration names, whatever InstanceClass kind the group references.
func TestForNodeGroup(t *testing.T) {
	yandex := Provider{Type: "yandex", InstanceClassKind: "YandexInstanceClass"}
	aws := Provider{Type: "aws", InstanceClassKind: "AWSInstanceClass"}
	// A registration that published no type is kept on load for the InstanceClass kind it carries.
	// Its empty type must not meet the empty name of a cluster that configures no provider.
	nameless := Provider{InstanceClassKind: "VsphereInstanceClass"}

	inYandexCloud := NewProviders([]Provider{aws, yandex, nameless}, "Yandex")
	staticCluster := NewProviders([]Provider{nameless}, "")

	tests := []struct {
		name      string
		providers Providers
		ng        *v1.NodeGroup
		want      string
	}{
		{
			// The kind a group references does not pick its provider: a kind mismatch is a verdict
			// about the NodeGroup, and derived_status.RunCloudChecks is what reports it.
			name:      "CloudEphemeral takes the cluster provider, not the one its kind belongs to",
			providers: inYandexCloud, ng: cloudEphemeral("worker-aws", "AWSInstanceClass"), want: "yandex",
		},
		{
			name:      "CloudEphemeral without a classReference takes the cluster provider",
			providers: inYandexCloud, ng: nodeGroupOfType("worker", v1.NodeTypeCloudEphemeral), want: "yandex",
		},
		{
			// CloudPermanent nodes are created by the installer and reference no InstanceClass, so
			// the cluster configuration is the only thing left to name their provider.
			name:      "CloudPermanent takes the cluster provider",
			providers: inYandexCloud, ng: nodeGroupOfType("master", v1.NodeTypeCloudPermanent), want: "yandex",
		},
		{
			// CloudStatic nodes do run in the cluster's cloud, Deckhouse just does not order them:
			// they still need the provider steps and the cloud variables.
			name:      "CloudStatic takes the cluster provider",
			providers: inYandexCloud, ng: nodeGroupOfType("cloudstatic", v1.NodeTypeCloudStatic), want: "yandex",
		},
		{
			// The whole point of the per-NodeGroup provider: a Static node lives outside every
			// cloud, so the provider steps must not reach it even in a cloud cluster.
			name:      "Static resolves to no provider in a cloud cluster",
			providers: inYandexCloud, ng: nodeGroupOfType("static", v1.NodeTypeStatic),
		},
		{
			name:      "a cluster that names no provider resolves to nothing",
			providers: staticCluster, ng: nodeGroupOfType("cloudstatic", v1.NodeTypeCloudStatic),
		},
		{
			// ClusterConfiguration spells providers OpenStack and vSphere; the Secrets spell them
			// lower case.
			name:      "the cluster provider matches case-insensitively",
			providers: NewProviders([]Provider{{Type: "openstack"}}, "OpenStack"),
			ng:        nodeGroupOfType("master", v1.NodeTypeCloudPermanent), want: "openstack",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := tc.providers.ForNodeGroup(tc.ng)

			assert.Equal(t, tc.want != "", ok)
			assert.Equal(t, tc.want, got.Type)
		})
	}
}

// A provider that names a kind but no version contributes no GVK: guessing a version renames the
// immutable MachineTemplate the instance-class checksum points at.
func TestInstanceClassGVKs(t *testing.T) {
	providers := NewProviders([]Provider{
		{Type: "aws", InstanceClassKind: "AWSInstanceClass", InstanceClassAPIVersion: "v1"},
		{Type: "yandex", InstanceClassKind: "YandexInstanceClass"},
	}, "aws")

	gvks := providers.InstanceClassGVKs()

	require.Len(t, gvks, 1)
	assert.Equal(t, schema.GroupVersionKind{
		Group: v1.GroupVersion.Group, Version: "v1", Kind: "AWSInstanceClass",
	}, gvks[0])
}

// The field declares an answer rather than picking one, so the predicate compares it against what
// the NodeGroup already resolved to. Empty always agrees — the field is optional.
func TestDeclarationError(t *testing.T) {
	yandex := Provider{Type: "yandex"}
	none := Provider{}

	for _, tc := range []struct {
		name     string
		declared string
		resolved Provider
		wantErr  bool
	}{
		{name: "empty agrees with a provider", resolved: yandex},
		{name: "empty agrees with no provider", resolved: none},
		{name: "the resolved provider", declared: "yandex", resolved: yandex},
		{name: "case does not matter", declared: "Yandex", resolved: yandex},
		{name: "None where there is none", declared: "None", resolved: none},
		{name: "none is spelled case-insensitively too", declared: "none", resolved: none},

		{name: "another provider", declared: "aws", resolved: yandex, wantErr: true},
		{name: "a provider where there is none", declared: "yandex", resolved: none, wantErr: true},
		{name: "None where there is one", declared: "None", resolved: yandex, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := DeclarationError(tc.declared, tc.resolved)
			if tc.wantErr {
				assert.NotEmpty(t, got)
				return
			}
			assert.Empty(t, got)
		})
	}
}
