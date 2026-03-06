// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"

	deckhouseiov1alpha1 "deckhouse.io/webhook/api/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"
)

func setupTestConversionReconciler() (*ConversionWebhookReconciler, client.Client) {
	// create fake kubernetes client
	sch := runtime.NewScheme()
	if err := deckhouseiov1alpha1.AddToScheme(sch); err != nil {
		panic(err)
	}
	if err := apiextensionsv1.AddToScheme(sch); err != nil {
		panic(err)
	}
	k8sClient := fake.NewClientBuilder().WithScheme(sch).Build()

	// init template file
	tpl, err := os.ReadFile("templates/conversionwebhook.tpl")
	if err != nil {
		panic(err)
	}

	var isReloadShellNeed atomic.Bool
	isReloadShellNeed.Store(false)

	reconciler := NewConversionWebhookReconciler(
		k8sClient,
		sch,
		log.NewLogger(log.WithLevel(slog.LevelDebug)),
		string(tpl),
		&isReloadShellNeed,
	)

	return reconciler, k8sClient
}

func getConversionStructFromYamlFile(filename string) (*deckhouseiov1alpha1.ConversionWebhook, error) {
	// open sample yaml
	sampleFile, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	// convert sample to json (to unmarshal)
	jsonData, err := yaml.YAMLToJSON(sampleFile)
	if err != nil {
		return nil, err
	}
	// fmt.Println(string(jsonData))

	// unmarshal sample
	var cwh *deckhouseiov1alpha1.ConversionWebhook
	err = json.Unmarshal(jsonData, &cwh)
	if err != nil {
		return nil, err
	}

	return cwh, nil
}

// ------------------
// --- TEST-CASES ---
// ------------------

func TestConversionTemplateNoError(t *testing.T) {
	// setup
	r, k8sClient := setupTestConversionReconciler()
	ctx := context.TODO()

	cwh, err := getConversionStructFromYamlFile("testdata/conversion/example.deckhouse.io.yaml")
	assert.NoError(t, err)

	err = k8sClient.Create(ctx, cwh)
	assert.NoError(t, err)

	_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: cwh.Namespace, Name: cwh.Name}})
	assert.NoError(t, err)

	// test equality
	ref, err := os.ReadFile("testdata/conversion/golden/example.deckhouse.io.py")
	assert.NoError(t, err)

	res, err := os.ReadFile("hooks/example.deckhouse.io/webhooks/conversion/example.deckhouse.io.py")
	assert.NoError(t, err)
	assert.Equal(t, string(ref), string(res))

	// test delete (two-pass: CRD cleanup, then FS cleanup)
	err = k8sClient.Get(ctx, types.NamespacedName{Namespace: cwh.Namespace, Name: cwh.Name}, cwh)
	assert.NoError(t, err)

	err = k8sClient.Delete(ctx, cwh)
	assert.NoError(t, err)

	// Pass 1: CRD cleanup finalizer removed (no target CRD exists in this test, so it's a no-op skip)
	_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: cwh.Namespace, Name: cwh.Name}})
	assert.NoError(t, err)

	// Pass 2: FS cleanup finalizer removed
	_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: cwh.Namespace, Name: cwh.Name}})
	assert.NoError(t, err)

	err = k8sClient.Get(ctx, types.NamespacedName{Namespace: cwh.Namespace, Name: cwh.Name}, cwh)
	assert.True(t, apierrors.IsNotFound(err))

	_, err = os.ReadFile("hooks/example.deckhouse.io/webhooks/conversion/example.deckhouse.io.py")
	assert.True(t, os.IsNotExist(err))
}

func TestConversionDeleteCleansCRD(t *testing.T) {
	r, k8sClient := setupTestConversionReconciler()
	ctx := context.TODO()

	crdName := "example.deckhouse.io"

	// Create a target CRD with webhook conversion strategy
	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: crdName},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "deckhouse.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:   "examples",
				Singular: "example",
				Kind:     "Example",
			},
			Scope: apiextensionsv1.ClusterScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{Name: "v1alpha1", Served: true, Storage: true, Schema: &apiextensionsv1.CustomResourceValidation{
					OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{Type: "object"},
				}},
			},
			Conversion: &apiextensionsv1.CustomResourceConversion{
				Strategy: apiextensionsv1.WebhookConverter,
				Webhook: &apiextensionsv1.WebhookConversion{
					ClientConfig: &apiextensionsv1.WebhookClientConfig{
						Service: &apiextensionsv1.ServiceReference{
							Name:      "conversion-webhook-handler",
							Namespace: "d8-system",
						},
					},
					ConversionReviewVersions: []string{"v1"},
				},
			},
		},
	}
	err := k8sClient.Create(ctx, crd)
	assert.NoError(t, err)

	// Create ConversionWebhook CR
	cwh, err := getConversionStructFromYamlFile("testdata/conversion/example.deckhouse.io.yaml")
	assert.NoError(t, err)

	err = k8sClient.Create(ctx, cwh)
	assert.NoError(t, err)

	// Reconcile to write file and add finalizers
	_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cwh.Name}})
	assert.NoError(t, err)

	// Delete the ConversionWebhook
	err = k8sClient.Get(ctx, types.NamespacedName{Name: cwh.Name}, cwh)
	assert.NoError(t, err)
	err = k8sClient.Delete(ctx, cwh)
	assert.NoError(t, err)

	// Pass 1: CRD cleanup — should reset conversion strategy to None
	_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cwh.Name}})
	assert.NoError(t, err)

	// Verify CRD conversion was reset
	err = k8sClient.Get(ctx, types.NamespacedName{Name: crdName}, crd)
	assert.NoError(t, err)
	assert.Equal(t, apiextensionsv1.NoneConverter, crd.Spec.Conversion.Strategy)
	assert.Nil(t, crd.Spec.Conversion.Webhook)

	// Pass 2: FS cleanup
	_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cwh.Name}})
	assert.NoError(t, err)

	// Verify resource is fully deleted
	err = k8sClient.Get(ctx, types.NamespacedName{Name: cwh.Name}, cwh)
	assert.True(t, apierrors.IsNotFound(err))

	_, err = os.ReadFile("hooks/example.deckhouse.io/webhooks/conversion/example.deckhouse.io.py")
	assert.True(t, os.IsNotExist(err))
}

func TestConversionDeleteCRDNotFound(t *testing.T) {
	r, k8sClient := setupTestConversionReconciler()
	ctx := context.TODO()

	// Create ConversionWebhook CR without a target CRD
	cwh, err := getConversionStructFromYamlFile("testdata/conversion/example.deckhouse.io.yaml")
	assert.NoError(t, err)

	err = k8sClient.Create(ctx, cwh)
	assert.NoError(t, err)

	// Reconcile to write file and add finalizers
	_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cwh.Name}})
	assert.NoError(t, err)

	// Delete the ConversionWebhook
	err = k8sClient.Get(ctx, types.NamespacedName{Name: cwh.Name}, cwh)
	assert.NoError(t, err)
	err = k8sClient.Delete(ctx, cwh)
	assert.NoError(t, err)

	// Pass 1: CRD cleanup — target CRD doesn't exist, should succeed gracefully
	_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cwh.Name}})
	assert.NoError(t, err)

	// Pass 2: FS cleanup
	_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: cwh.Name}})
	assert.NoError(t, err)

	// Verify resource is fully deleted
	err = k8sClient.Get(ctx, types.NamespacedName{Name: cwh.Name}, cwh)
	assert.True(t, apierrors.IsNotFound(err))
}

func TestConversionTemplateEqual(t *testing.T) {
	// setup
	r, k8sClient := setupTestConversionReconciler()
	ctx := context.TODO()

	cwh, err := getConversionStructFromYamlFile("testdata/conversion/nodegroups.deckhouse.io.yaml")
	assert.NoError(t, err)

	err = k8sClient.Create(ctx, cwh)
	assert.NoError(t, err)

	_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: cwh.Namespace, Name: cwh.Name}})
	assert.NoError(t, err)

	// test equality
	ref, err := os.ReadFile("testdata/conversion/golden/nodegroups.deckhouse.io.py")
	assert.NoError(t, err)

	res, err := os.ReadFile("hooks/nodegroups.deckhouse.io/webhooks/conversion/nodegroups.deckhouse.io.py")
	assert.NoError(t, err)
	assert.Equal(t, string(ref), string(res))
}
