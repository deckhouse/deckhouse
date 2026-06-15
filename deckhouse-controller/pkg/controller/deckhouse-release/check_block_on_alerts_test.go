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
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/go_lib/project"
	"github.com/deckhouse/deckhouse/pkg/log"
)

var clusterAlertGVK = schema.GroupVersionKind{
	Group:   "deckhouse.io",
	Version: "v1alpha1",
	Kind:    "ClusterAlert",
}

func makeClusterAlert(name string, severityLevel int) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(clusterAlertGVK)
	obj.SetName(name)

	_ = unstructured.SetNestedField(obj.Object, strconv.Itoa(severityLevel), "alert", "severityLevel")
	_ = unstructured.SetNestedField(obj.Object, name+"-alert", "alert", "name")

	return obj
}

func makeClusterAlertWithoutSeverity(name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(clusterAlertGVK)
	obj.SetName(name)

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

	return &deckhouseReleaseReconciler{client: cl, logger: log.NewNop()}
}

func TestCheckBlockOnAlerts(t *testing.T) {
	ctx := context.Background()

	t.Run("no alerts – update is not blocked", func(t *testing.T) {
		rec := newReconcilerWithAlerts(t)
		require.NoError(t, rec.checkBlockOnAlerts(ctx, 4))
	})

	t.Run("alert severity equals threshold – blocked", func(t *testing.T) {
		rec := newReconcilerWithAlerts(t, makeClusterAlert("alert-eq", 4))
		err := rec.checkBlockOnAlerts(ctx, 4)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "alert-eq")
	})

	t.Run("alert severity below threshold – blocked", func(t *testing.T) {
		rec := newReconcilerWithAlerts(t, makeClusterAlert("alert-low", 2))
		err := rec.checkBlockOnAlerts(ctx, 4)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "alert-low")
		assert.Contains(t, err.Error(), "2")
	})

	t.Run("alert severity above threshold – not blocked", func(t *testing.T) {
		rec := newReconcilerWithAlerts(t, makeClusterAlert("alert-high", 7))
		require.NoError(t, rec.checkBlockOnAlerts(ctx, 4))
	})

	t.Run("severityLevel absent – alert is skipped, not blocked", func(t *testing.T) {
		rec := newReconcilerWithAlerts(t, makeClusterAlertWithoutSeverity("alert-no-sev"))
		require.NoError(t, rec.checkBlockOnAlerts(ctx, 4))
	})

	t.Run("zero threshold – severity 5 is not blocked", func(t *testing.T) {
		rec := newReconcilerWithAlerts(t, makeClusterAlert("alert-default", 5))
		require.NoError(t, rec.checkBlockOnAlerts(ctx, 0))
	})

	t.Run("zero threshold – severity 0 is blocked", func(t *testing.T) {
		rec := newReconcilerWithAlerts(t, makeClusterAlert("alert-default-eq", 0))
		err := rec.checkBlockOnAlerts(ctx, 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "0")
	})

	t.Run("multiple alerts: one below threshold – blocked on first violator", func(t *testing.T) {
		rec := newReconcilerWithAlerts(t,
			makeClusterAlert("blocking-alert", 3),
			makeClusterAlert("safe-alert", 8),
		)
		err := rec.checkBlockOnAlerts(ctx, 4)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "blocking-alert")
	})

	t.Run("custom threshold 6 – severity 5 is blocked", func(t *testing.T) {
		rec := newReconcilerWithAlerts(t, makeClusterAlert("alert-under-custom", 5))
		err := rec.checkBlockOnAlerts(ctx, 6)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "5")
	})

	t.Run("custom threshold 6 – severity 7 is not blocked", func(t *testing.T) {
		rec := newReconcilerWithAlerts(t, makeClusterAlert("alert-over-custom", 7))
		require.NoError(t, rec.checkBlockOnAlerts(ctx, 6))
	})
}
