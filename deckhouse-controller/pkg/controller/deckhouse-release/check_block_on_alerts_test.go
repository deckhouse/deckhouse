/*
Copyright 2024 Flant JSC

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

package deckhouse_release

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/go_lib/project"
)

var clusterAlertGVK = schema.GroupVersionKind{
	Group:   "deckhouse.io",
	Version: "v1alpha1",
	Kind:    "ClusterAlert",
}

func makeClusterAlert(name string, severityLevel interface{}) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(clusterAlertGVK)
	obj.SetName(name)

	if severityLevel != nil {
		_ = unstructured.SetNestedField(obj.Object, severityLevel, "alert", "severityLevel")
	}
	_ = unstructured.SetNestedField(obj.Object, name+"-alert", "alert", "name")

	return obj
}

func newReconcilerWithAlerts(t *testing.T, alerts ...*unstructured.Unstructured) *deckhouseReleaseReconciler {
	t.Helper()

	sc, err := project.Scheme()
	require.NoError(t, err)

	builder := fake.NewClientBuilder().WithScheme(sc)
	for _, a := range alerts {
		builder = builder.WithObjects(a)
	}
	cl := builder.Build()

	return &deckhouseReleaseReconciler{client: cl}
}

func TestCheckBlockOnAlerts(t *testing.T) {
	ctx := context.Background()

	t.Run("no alerts – update is not blocked", func(t *testing.T) {
		rec := newReconcilerWithAlerts(t)
		require.NoError(t, rec.checkBlockOnAlerts(ctx, 4))
	})

	t.Run("alert severity equals threshold – blocked", func(t *testing.T) {
		rec := newReconcilerWithAlerts(t, makeClusterAlert("alert-eq", "4"))
		err := rec.checkBlockOnAlerts(ctx, 4)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "alert-eq")
	})

	t.Run("alert severity below threshold – not blocked", func(t *testing.T) {
		rec := newReconcilerWithAlerts(t, makeClusterAlert("alert-low", "2"))
		require.NoError(t, rec.checkBlockOnAlerts(ctx, 4))
	})

	t.Run("alert severity above threshold – blocked", func(t *testing.T) {
		rec := newReconcilerWithAlerts(t, makeClusterAlert("alert-high", "7"))
		err := rec.checkBlockOnAlerts(ctx, 4)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "alert-high")
		assert.Contains(t, err.Error(), "7")
	})

	t.Run("severityLevel stored as int64 – not blocked (unhandled type skipped)", func(t *testing.T) {
		rec := newReconcilerWithAlerts(t, makeClusterAlert("alert-int", int64(9)))
		require.NoError(t, rec.checkBlockOnAlerts(ctx, 4))
	})

	t.Run("severityLevel absent – alert is skipped, not blocked", func(t *testing.T) {
		rec := newReconcilerWithAlerts(t, makeClusterAlert("alert-no-sev", nil))
		require.NoError(t, rec.checkBlockOnAlerts(ctx, 4))
	})

	t.Run("zero threshold – severity 5 is blocked", func(t *testing.T) {
		rec := newReconcilerWithAlerts(t, makeClusterAlert("alert-default", "5"))
		err := rec.checkBlockOnAlerts(ctx, 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "5")
	})

	t.Run("zero threshold – severity 4 is blocked", func(t *testing.T) {
		rec := newReconcilerWithAlerts(t, makeClusterAlert("alert-default-eq", "4"))
		err := rec.checkBlockOnAlerts(ctx, 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "4")
	})

	t.Run("multiple alerts: one above threshold – blocked on first violator", func(t *testing.T) {
		rec := newReconcilerWithAlerts(t,
			makeClusterAlert("safe-alert", "3"),
			makeClusterAlert("blocking-alert", "8"),
		)
		err := rec.checkBlockOnAlerts(ctx, 4)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "blocking-alert")
	})

	t.Run("custom threshold 6 – severity 5 is not blocked", func(t *testing.T) {
		rec := newReconcilerWithAlerts(t, makeClusterAlert("alert-under-custom", "5"))
		require.NoError(t, rec.checkBlockOnAlerts(ctx, 6))
	})

	t.Run("custom threshold 6 – severity 7 is blocked", func(t *testing.T) {
		rec := newReconcilerWithAlerts(t, makeClusterAlert("alert-over-custom", "7"))
		err := rec.checkBlockOnAlerts(ctx, 6)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "7")
	})
}
