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
			Namespace: RegistrationSecretNamespace,
			Name:      name,
			Labels:    map[string]string{RegistrationSecretLabel: ""},
		},
		Data: data,
	}
}

func loadFrom(t *testing.T, objs ...client.Object) Catalog {
	t.Helper()
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(objs...).Build()
	pCatalog, err := GetCatalog(context.Background(), c)
	require.NoError(t, err)
	return pCatalog
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

func TestGetCatalog(t *testing.T) {
	aws := registrationSecret(RegistrationSecretBaseName+"-aws", map[string][]byte{"type": []byte("aws")})
	yandexData := map[string][]byte{
		"type":                    []byte("yandex"),
		"instanceClassKind":       []byte("YandexInstanceClass"),
		"instanceClassAPIVersion": []byte("v1"),
	}
	unlabelled := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: RegistrationSecretNamespace, Name: "some-other-secret"},
		Data:       map[string][]byte{"type": []byte("notaprovider")},
	}

	tests := []struct {
		name        string
		objs        []client.Object
		types       []string
		defaultType string
	}{
		{
			// Every provider module renders its registration twice — under the fixed name and
			// under a per-provider one. Both carry the label, so without deduplication one
			// provider would read as two.
			name: "the fixed-name and the per-provider copy are one provider",
			objs: []client.Object{
				registrationSecret(RegistrationSecretBaseName, yandexData),
				registrationSecret(RegistrationSecretBaseName+"-yandex", yandexData),
			},
			types:       []string{"yandex"},
			defaultType: "yandex",
		},
		{
			// The whole point of the fixed name: with two providers registered, the default is the
			// one published under it, not the first by type and not the last read.
			name: "the default is the fixed-name registration",
			objs: []client.Object{
				aws,
				registrationSecret(RegistrationSecretBaseName, yandexData),
			},
			types:       []string{"aws", "yandex"},
			defaultType: "yandex",
		},
		{
			// Selection is by label, not by name: the per-provider Secret of a second provider is
			// the whole point of this package, and an unlabelled Secret is not a registration.
			// Nothing holds the fixed name here, so no NodeGroup resolves to a default.
			name: "every labelled registration is seen, ordered by type",
			objs: []client.Object{
				registrationSecret(RegistrationSecretBaseName+"-yandex", yandexData),
				aws,
				unlabelled,
			},
			types: []string{"aws", "yandex"},
		},
		{
			name: "a cluster with no cloud provider at all",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pCatalog := loadFrom(t, tc.objs...)

			var got []string
			for _, p := range pCatalog.All() {
				got = append(got, p.Type)
			}
			assert.Equal(t, tc.types, got)
			assert.Equal(t, tc.defaultType, pCatalog.Default().Type)
			assert.Equal(t, tc.defaultType == "", pCatalog.Default().IsStatic())
		})
	}
}

// An unreadable source must not read as "no cloud provider": an empty registry publishes
// NodeGroups without instanceClass, which shifts the configuration checksum of every node, and
// every non-Static group resolves through the default registration and nothing else — so a
// provider whose Secret could not be read would render the master without its steps.
func TestGetCatalog_Errors(t *testing.T) {
	tests := []struct {
		name     string
		denyList bool
		denyGet  bool
		wantErr  string
	}{
		{
			name:     "the registrations cannot be listed",
			denyList: true,
			wantErr:  "list cloud provider registration secrets",
		},
		{
			// NotFound is the legitimate "no cloud" answer; anything else is not.
			name:    "the default registration cannot be read",
			denyGet: true,
			wantErr: `get secret "d8-node-manager-cloud-provider"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(testScheme(t))
			// The last-resort tool: producing a Forbidden needs RBAC that envtest does not run.
			funcs := interceptor.Funcs{}
			if tc.denyList {
				funcs.List = func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
					return apierrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, "", context.Canceled)
				}
			}
			if tc.denyGet {
				funcs.Get = func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
					return apierrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, "", context.Canceled)
				}
			}

			_, err := GetCatalog(context.Background(), builder.WithInterceptorFuncs(funcs).Build())

			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

// A provider that names a kind but no version contributes no GVK: guessing a version renames the
// immutable MachineTemplate the instance-class checksum points at.
func TestInstanceClassGVKs(t *testing.T) {
	pCatalog := NewCatalog([]Provider{
		{Type: "aws", InstanceClassKind: "AWSInstanceClass", InstanceClassAPIVersion: "v1"},
		{Type: "yandex", InstanceClassKind: "YandexInstanceClass"},
	}, Provider{})

	gvks := pCatalog.InstanceClassGVKs()

	require.Len(t, gvks, 1)
	assert.Equal(t, schema.GroupVersionKind{
		Group: v1.GroupVersion.Group, Version: "v1", Kind: "AWSInstanceClass",
	}, gvks[0])
}
