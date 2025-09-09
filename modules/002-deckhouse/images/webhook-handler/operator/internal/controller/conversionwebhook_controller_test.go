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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"
)

func setupTestConversionReconciler() *ConversionWebhookReconciler {
	// create fake kubernetes client
	sch := runtime.NewScheme()
	deckhouseiov1alpha1.AddToScheme(sch)
	k8sClient := fake.NewClientBuilder().WithScheme(sch).Build()

	// init template file
	tpl, err := os.ReadFile("templates/conversionwebhook.tpl")
	if err != nil {
		panic(err)
	}

	var isReloadShellNeed atomic.Bool
	isReloadShellNeed.Store(false)

	return &ConversionWebhookReconciler{
		IsReloadShellNeed: &isReloadShellNeed,
		Client:            k8sClient,
		Scheme:            sch,
		Logger:            log.NewLogger(log.WithLevel(slog.LevelDebug)),
		PythonTemplate:    string(tpl),
	}
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

func TestTemplateNoError2(t *testing.T) {
	// setup
	r := setupTestConversionReconciler()
	ctx := context.TODO()

	cwh, err := getConversionStructFromYamlFile("testdata/conversion/conversionwebhook-sample.yaml")
	assert.NoError(t, err)

	err = r.Client.Create(ctx, cwh)
	assert.NoError(t, err)

	_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: cwh.Namespace, Name: cwh.Name}})
	assert.NoError(t, err)

	// test equality
	// ref, err := os.ReadFile("testdata/validating/golden/validationwebhook-sample.py")
	// assert.NoError(t, err)

	// res, err := os.ReadFile("hooks/validationwebhook-sample/webhooks/validating/validationwebhook-sample.py")
	// assert.NoError(t, err)
	// assert.Equal(t, string(ref), string(res))

	// // test delete
	// err = r.Client.Get(ctx, types.NamespacedName{Namespace: cwh.Namespace, Name: cwh.Name}, cwh)
	// assert.NoError(t, err)

	// err = r.Client.Delete(ctx, cwh)
	// assert.NoError(t, err)

	// _, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: cwh.Namespace, Name: cwh.Name}})
	// assert.NoError(t, err)

	// err = r.Client.Get(ctx, types.NamespacedName{Namespace: cwh.Namespace, Name: cwh.Name}, cwh)
	// assert.True(t, apierrors.IsNotFound(err))

	// _, err = os.ReadFile("hooks/validationwebhook-sample/webhooks/validating/validationwebhook-sample.py")
	// assert.True(t, os.IsNotExist(err))
}
