//go:build ai_tests

/*
Copyright 2025 Flant JSC

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

package crdwebhook

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = apiextensionsv1.AddToScheme(s)
	return s
}

// TestAI_SecretWithCACrtPatchesCRDs verifies that when a watched secret with ca.crt
// data is reconciled, the corresponding CRDs are patched with the CA bundle.
func TestAI_SecretWithCACrtPatchesCRDs(t *testing.T) {
	scheme := newTestScheme()

	caData := []byte("my-test-ca-bundle")

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-controller-webhook-tls",
			Namespace: webhookNamespace,
		},
		Data: map[string][]byte{
			"ca.crt": caData,
		},
	}

	// Create the CRD that should be patched.
	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nodegroups.deckhouse.io",
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "deckhouse.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:   "nodegroups",
				Singular: "nodegroup",
				Kind:     "NodeGroup",
			},
			Scope: apiextensionsv1.ClusterScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1",
					Served:  true,
					Storage: true,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
						},
					},
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret, crd).
		Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "node-controller-webhook-tls",
			Namespace: webhookNamespace,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify the CRD was patched with the CA bundle.
	updatedCRD := &apiextensionsv1.CustomResourceDefinition{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "nodegroups.deckhouse.io"}, updatedCRD)
	require.NoError(t, err)

	require.NotNil(t, updatedCRD.Spec.Conversion)
	require.NotNil(t, updatedCRD.Spec.Conversion.Webhook)
	require.NotNil(t, updatedCRD.Spec.Conversion.Webhook.ClientConfig)

	expectedCA := base64.StdEncoding.EncodeToString(caData)
	assert.Equal(t, expectedCA, string(updatedCRD.Spec.Conversion.Webhook.ClientConfig.CABundle))
	assert.Equal(t, apiextensionsv1.WebhookConverter, updatedCRD.Spec.Conversion.Strategy)
	assert.Equal(t, "node-controller-webhook", updatedCRD.Spec.Conversion.Webhook.ClientConfig.Service.Name)
}

// TestAI_SecretNotFound verifies that reconciling a non-existent secret
// returns no error.
func TestAI_SecretNotFound(t *testing.T) {
	scheme := newTestScheme()

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "node-controller-webhook-tls",
			Namespace: webhookNamespace,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// TestAI_SecretWithoutCACrt verifies that a secret without the ca.crt key
// is skipped without error.
func TestAI_SecretWithoutCACrt(t *testing.T) {
	scheme := newTestScheme()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-controller-webhook-tls",
			Namespace: webhookNamespace,
		},
		Data: map[string][]byte{
			"tls.crt": []byte("some-cert"),
			"tls.key": []byte("some-key"),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret).
		Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "node-controller-webhook-tls",
			Namespace: webhookNamespace,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// TestAI_UnwatchedSecretSkipped verifies that a secret not in the watchedSecrets set
// is skipped.
func TestAI_UnwatchedSecretSkipped(t *testing.T) {
	scheme := newTestScheme()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-other-secret",
			Namespace: webhookNamespace,
		},
		Data: map[string][]byte{
			"ca.crt": []byte("some-ca"),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret).
		Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "some-other-secret",
			Namespace: webhookNamespace,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// TestAI_WrongNamespaceSkipped verifies that a secret in a different namespace
// is skipped.
func TestAI_WrongNamespaceSkipped(t *testing.T) {
	scheme := newTestScheme()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-controller-webhook-tls",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"ca.crt": []byte("some-ca"),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret).
		Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "node-controller-webhook-tls",
			Namespace: "default",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}
