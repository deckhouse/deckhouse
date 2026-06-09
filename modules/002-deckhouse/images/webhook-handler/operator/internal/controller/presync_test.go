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
	"log/slog"
	"os"
	"testing"
	"time"

	deckhouseiov1alpha1 "deckhouse.io/webhook/api/v1alpha1"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestPresyncWritesConversionWebhookFiles(t *testing.T) {
	os.RemoveAll("hooks")
	defer os.RemoveAll("hooks")

	sch := runtime.NewScheme()
	require.NoError(t, deckhouseiov1alpha1.AddToScheme(sch))

	cwh, err := getConversionStructFromYamlFile("testdata/conversion/example.deckhouse.io.yaml")
	require.NoError(t, err)

	k8sClient := fake.NewClientBuilder().WithScheme(sch).WithObjects(cwh).Build()

	conversionTpl, err := os.ReadFile("templates/conversionwebhook.tpl")
	require.NoError(t, err)
	validationTpl, err := os.ReadFile("templates/validationwebhook.tpl")
	require.NoError(t, err)

	logger := log.NewLogger(log.WithLevel(slog.LevelDebug))

	err = PresyncWebhookFiles(context.TODO(), k8sClient, string(conversionTpl), string(validationTpl), logger)
	require.NoError(t, err)

	// Verify the file was written
	ref, err := os.ReadFile("testdata/conversion/golden/example.deckhouse.io.py")
	require.NoError(t, err)

	res, err := os.ReadFile("hooks/example.deckhouse.io/webhooks/conversion/example.deckhouse.io.py")
	require.NoError(t, err)

	assert.Equal(t, string(ref), string(res))
}

func TestPresyncWritesValidationWebhookFiles(t *testing.T) {
	os.RemoveAll("hooks")
	defer os.RemoveAll("hooks")

	sch := runtime.NewScheme()
	require.NoError(t, deckhouseiov1alpha1.AddToScheme(sch))

	vh, err := getStructFromYamlFile("testdata/validating/validationwebhook-sample.yaml")
	require.NoError(t, err)

	k8sClient := fake.NewClientBuilder().WithScheme(sch).WithObjects(vh).Build()

	conversionTpl, err := os.ReadFile("templates/conversionwebhook.tpl")
	require.NoError(t, err)
	validationTpl, err := os.ReadFile("templates/validationwebhook.tpl")
	require.NoError(t, err)

	logger := log.NewLogger(log.WithLevel(slog.LevelDebug))

	err = PresyncWebhookFiles(context.TODO(), k8sClient, string(conversionTpl), string(validationTpl), logger)
	require.NoError(t, err)

	// Verify the file was written
	ref, err := os.ReadFile("testdata/validating/golden/validationwebhook-sample.py")
	require.NoError(t, err)

	res, err := os.ReadFile("hooks/validationwebhook-sample/webhooks/validating/validationwebhook-sample.py")
	require.NoError(t, err)

	assert.Equal(t, string(ref), string(res))
}

func TestPresyncPreventsReloadOnFirstReconcile(t *testing.T) {
	os.RemoveAll("hooks")
	defer os.RemoveAll("hooks")

	sch := runtime.NewScheme()
	require.NoError(t, deckhouseiov1alpha1.AddToScheme(sch))

	vh, err := getStructFromYamlFile("testdata/validating/validationwebhook-sample.yaml")
	require.NoError(t, err)

	k8sClient := fake.NewClientBuilder().WithScheme(sch).WithObjects(vh).Build()

	conversionTpl, err := os.ReadFile("templates/conversionwebhook.tpl")
	require.NoError(t, err)
	validationTpl, err := os.ReadFile("templates/validationwebhook.tpl")
	require.NoError(t, err)

	logger := log.NewLogger(log.WithLevel(slog.LevelDebug))

	// Step 1: presync writes the file
	err = PresyncWebhookFiles(context.TODO(), k8sClient, string(conversionTpl), string(validationTpl), logger)
	require.NoError(t, err)

	// Pin the file's mtime to a known past value so that any rewrite during
	// reconcile is reliably detected regardless of filesystem timestamp
	// resolution.
	webhookFile := "hooks/validationwebhook-sample/webhooks/validating/validationwebhook-sample.py"
	pinnedTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, os.Chtimes(webhookFile, pinnedTime, pinnedTime))

	// Step 2: now create a reconciler and reconcile the same CR.
	// reloadFn will be called because the finalizer is not yet set (presync
	// doesn't add it), but the file must NOT be rewritten.
	reloadFn := func(_ context.Context) error { return nil }
	reconciler := NewValidationWebhookReconciler(
		k8sClient,
		sch,
		logger,
		string(validationTpl),
		reloadFn,
	)

	_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: vh.Name},
	})
	require.NoError(t, err)

	// The file must not be rewritten because presync already wrote the
	// identical content.  A rewrite would advance ModTime past pinnedTime.
	infoAfter, err := os.Stat(webhookFile)
	require.NoError(t, err)
	assert.True(t, infoAfter.ModTime().Equal(pinnedTime),
		"reconciler should not rewrite the file that presync already wrote (mtime should remain %v, got %v)",
		pinnedTime, infoAfter.ModTime())
}

func TestPresyncIsIdempotent(t *testing.T) {
	os.RemoveAll("hooks")
	defer os.RemoveAll("hooks")

	sch := runtime.NewScheme()
	require.NoError(t, deckhouseiov1alpha1.AddToScheme(sch))

	cwh, err := getConversionStructFromYamlFile("testdata/conversion/example.deckhouse.io.yaml")
	require.NoError(t, err)

	k8sClient := fake.NewClientBuilder().WithScheme(sch).WithObjects(cwh).Build()

	conversionTpl, err := os.ReadFile("templates/conversionwebhook.tpl")
	require.NoError(t, err)
	validationTpl, err := os.ReadFile("templates/validationwebhook.tpl")
	require.NoError(t, err)

	logger := log.NewLogger(log.WithLevel(slog.LevelDebug))

	// Run presync twice — second call should be a no-op
	err = PresyncWebhookFiles(context.TODO(), k8sClient, string(conversionTpl), string(validationTpl), logger)
	require.NoError(t, err)

	filePath := "hooks/example.deckhouse.io/webhooks/conversion/example.deckhouse.io.py"
	info1, err := os.Stat(filePath)
	require.NoError(t, err)

	err = PresyncWebhookFiles(context.TODO(), k8sClient, string(conversionTpl), string(validationTpl), logger)
	require.NoError(t, err)

	info2, err := os.Stat(filePath)
	require.NoError(t, err)

	// ModTime should not change if content is identical (file not rewritten)
	assert.Equal(t, info1.ModTime(), info2.ModTime(),
		"presync should not rewrite an unchanged file")
}

func TestPresyncHandlesNoCRs(t *testing.T) {
	os.RemoveAll("hooks")
	defer os.RemoveAll("hooks")

	sch := runtime.NewScheme()
	require.NoError(t, deckhouseiov1alpha1.AddToScheme(sch))

	k8sClient := fake.NewClientBuilder().WithScheme(sch).Build()

	conversionTpl, err := os.ReadFile("templates/conversionwebhook.tpl")
	require.NoError(t, err)
	validationTpl, err := os.ReadFile("templates/validationwebhook.tpl")
	require.NoError(t, err)

	logger := log.NewLogger(log.WithLevel(slog.LevelDebug))

	// Should succeed with no CRs
	err = PresyncWebhookFiles(context.TODO(), k8sClient, string(conversionTpl), string(validationTpl), logger)
	assert.NoError(t, err)
}
