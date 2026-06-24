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

package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
)

func TestIsInstanceConditionTrue(t *testing.T) {
	t.Parallel()

	conditions := []deckhousev1alpha2.InstanceCondition{
		{Type: "Ready", Status: metav1.ConditionTrue},
		{Type: "Degraded", Status: metav1.ConditionFalse},
	}

	require.True(t, IsInstanceConditionTrue(conditions, "Ready"))
	require.False(t, IsInstanceConditionTrue(conditions, "Degraded"))
	require.False(t, IsInstanceConditionTrue(conditions, "Missing"))
	require.False(t, IsInstanceConditionTrue(nil, "Ready"))
}

func TestGetInstanceConditionByTypeMissing(t *testing.T) {
	t.Parallel()

	conditions := []deckhousev1alpha2.InstanceCondition{
		{Type: "Ready", Status: metav1.ConditionTrue},
	}

	cond, ok := GetInstanceConditionByType(conditions, "Missing")
	require.False(t, ok)
	require.Equal(t, deckhousev1alpha2.InstanceCondition{}, cond)
}

func TestRemoveInstanceControllerFinalizer(t *testing.T) {
	t.Parallel()

	t.Run("no finalizer is a no-op", func(t *testing.T) {
		t.Parallel()

		instance := &deckhousev1alpha2.Instance{
			ObjectMeta: metav1.ObjectMeta{Name: "no-finalizer"},
		}
		c := newFakeInstanceClient(t, instance.DeepCopy())

		require.NoError(t, RemoveInstanceControllerFinalizer(context.Background(), c, instance))
		require.NotContains(t, instance.Finalizers, InstanceControllerFinalizer)
	})

	t.Run("removes finalizer and updates in place", func(t *testing.T) {
		t.Parallel()

		instance := &deckhousev1alpha2.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "with-finalizer",
				Finalizers: []string{InstanceControllerFinalizer, "other"},
			},
		}
		c := newFakeInstanceClient(t, instance.DeepCopy())

		require.NoError(t, RemoveInstanceControllerFinalizer(context.Background(), c, instance))
		require.NotContains(t, instance.Finalizers, InstanceControllerFinalizer)
		require.Contains(t, instance.Finalizers, "other")

		persisted := &deckhousev1alpha2.Instance{}
		require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: instance.Name}, persisted))
		require.NotContains(t, persisted.Finalizers, InstanceControllerFinalizer)
		require.Contains(t, persisted.Finalizers, "other")
	})
}

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, deckhousev1alpha2.AddToScheme(scheme))

	return scheme
}
