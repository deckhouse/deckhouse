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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"
)

func setupTestReconciler() (*ValidationWebhookReconciler, client.Client) {
	// create fake kubernetes client
	sch := runtime.NewScheme()
	if err := deckhouseiov1alpha1.AddToScheme(sch); err != nil {
		panic(err)
	}
	k8sClient := fake.NewClientBuilder().WithScheme(sch).Build()

	// init template file
	tpl, err := os.ReadFile("templates/validationwebhook.tpl")
	if err != nil {
		panic(err)
	}

	var isReloadShellNeed atomic.Bool
	isReloadShellNeed.Store(false)

	reconciler := NewValidationWebhookReconciler(
		k8sClient,
		sch,
		log.NewLogger(log.WithLevel(slog.LevelDebug)),
		string(tpl),
		&isReloadShellNeed,
	)

	return reconciler, k8sClient
}

func getStructFromYamlFile(filename string) (*deckhouseiov1alpha1.ValidationWebhook, error) {
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
	var vh *deckhouseiov1alpha1.ValidationWebhook
	err = json.Unmarshal(jsonData, &vh)
	if err != nil {
		return nil, err
	}

	return vh, nil
}

// ------------------
// --- TEST-CASES ---
// ------------------

func TestTemplateNoError(t *testing.T) {
	// hooks/002-deckhouse/webhooks/validating
	// os.MkdirAll("/hooks/"+vh.Name+"/webhooks/validating/", 0777)

	// setup
	r, k8sClient := setupTestReconciler()
	ctx := context.TODO()

	vh, err := getStructFromYamlFile("testdata/validating/validationwebhook-sample.yaml")
	assert.NoError(t, err)

	err = k8sClient.Create(ctx, vh)
	assert.NoError(t, err)

	_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: vh.Namespace, Name: vh.Name}})
	assert.NoError(t, err)

	// test equality
	ref, err := os.ReadFile("testdata/validating/golden/validationwebhook-sample.py")
	assert.NoError(t, err)

	res, err := os.ReadFile("hooks/validationwebhook-sample/webhooks/validating/validationwebhook-sample.py")
	assert.NoError(t, err)
	assert.Equal(t, string(ref), string(res))

	// test delete
	err = k8sClient.Get(ctx, types.NamespacedName{Namespace: vh.Namespace, Name: vh.Name}, vh)
	assert.NoError(t, err)

	err = k8sClient.Delete(ctx, vh)
	assert.NoError(t, err)

	_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: vh.Namespace, Name: vh.Name}})
	assert.NoError(t, err)

	err = k8sClient.Get(ctx, types.NamespacedName{Namespace: vh.Namespace, Name: vh.Name}, vh)
	assert.True(t, apierrors.IsNotFound(err))

	_, err = os.ReadFile("hooks/validationwebhook-sample/webhooks/validating/validationwebhook-sample.py")
	assert.True(t, os.IsNotExist(err))
}

func TestTemplateNoContext(t *testing.T) {
	r, k8sClient := setupTestReconciler()

	vh, err := getStructFromYamlFile("testdata/validating/sample_without_context.yaml")
	assert.NoError(t, err)

	err = k8sClient.Create(context.Background(), vh)
	assert.NoError(t, err)

	_, err = r.handleProcessValidatingWebhook(context.TODO(), vh)
	assert.NoError(t, err)

	// test equality
	ref, err := os.ReadFile("testdata/validating/golden/sample_without_context.py")
	assert.NoError(t, err)
	res, err := os.ReadFile("hooks/validationwebhook-sample/webhooks/validating/validationwebhook-sample.py")
	assert.NoError(t, err)
	assert.Equal(t, string(ref), string(res))
}

func TestTemplateTwoContext(t *testing.T) {
	r, k8sClient := setupTestReconciler()

	vh, err := getStructFromYamlFile("testdata/validating/sample_two_context.yaml")
	assert.NoError(t, err)

	err = k8sClient.Create(context.Background(), vh)
	assert.NoError(t, err)

	_, err = r.handleProcessValidatingWebhook(context.TODO(), vh)
	assert.NoError(t, err)

	// test equality
	ref, err := os.ReadFile("testdata/validating/golden/sample_two_context.py")
	assert.NoError(t, err)
	res, err := os.ReadFile("hooks/validationwebhook-sample/webhooks/validating/validationwebhook-sample.py")
	assert.NoError(t, err)
	assert.Equal(t, string(ref), string(res))
}

func TestTemplateEqual(t *testing.T) {
	r, k8sClient := setupTestReconciler()

	vh, err := getStructFromYamlFile("testdata/validating/prometheusremotewrite.yaml")
	assert.NoError(t, err)

	err = k8sClient.Create(context.Background(), vh)
	assert.NoError(t, err)

	_, err = r.handleProcessValidatingWebhook(context.TODO(), vh)
	assert.NoError(t, err)

	ref, err := os.ReadFile("testdata/validating/golden/prometheusremotewrite.py")
	assert.NoError(t, err)

	res, err := os.ReadFile("hooks/prometheusremotewrite/webhooks/validating/prometheusremotewrite.py")
	assert.NoError(t, err)

	assert.Equal(t, string(ref), string(res))
}

func TestTemplateIncludeSnapshotsFrom(t *testing.T) {
	r, k8sClient := setupTestReconciler()

	vh, err := getStructFromYamlFile("testdata/validating/publicdomaintemplate.yaml")
	assert.NoError(t, err)

	err = k8sClient.Create(context.Background(), vh)
	assert.NoError(t, err)

	_, err = r.handleProcessValidatingWebhook(context.TODO(), vh)
	assert.NoError(t, err)

	ref, err := os.ReadFile("testdata/validating/golden/publicdomaintemplate.py")
	assert.NoError(t, err)

	res, err := os.ReadFile("hooks/public-domain-template/webhooks/validating/public-domain-template.py")
	assert.NoError(t, err)

	assert.Equal(t, string(ref), string(res))
}
