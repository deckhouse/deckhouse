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

package config

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

func newReconciler(t *testing.T, objects ...client.Object) (*Reconciler, client.Client) {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, registryv1alpha1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		WithStatusSubresource(&registryv1alpha1.RegistryConfig{}).
		Build()

	r := &Reconciler{}
	r.InjectClient(fakeClient)
	return r, fakeClient
}

func requestFor(name string) ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{Name: name}}
}

func getConfig(t *testing.T, c client.Client, name string) *registryv1alpha1.RegistryConfig {
	t.Helper()

	cfg := &registryv1alpha1.RegistryConfig{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: name}, cfg))
	return cfg
}

func TestReconcileMarksAValidConfigValid(t *testing.T) {
	cfg := &registryv1alpha1.RegistryConfig{
		ObjectMeta: metav1.ObjectMeta{Name: registryv1alpha1.SingletonName, Generation: 3},
		Spec: registryv1alpha1.RegistryConfigSpec{
			Mode:    registryv1alpha1.ModeManaged,
			Primary: registryv1alpha1.PrimarySource{Upstream: upstream()},
			Storage: registryv1alpha1.StorageConfig{Cache: true, Size: "50Gi"},
		},
	}

	r, c := newReconciler(t, cfg)

	result, err := r.Reconcile(context.Background(), requestFor(cfg.Name))
	require.NoError(t, err)
	assert.Zero(t, result.RequeueAfter)

	got := getConfig(t, c, cfg.Name)
	assert.EqualValues(t, 3, got.Status.ObservedGeneration)

	condition := apimeta.FindStatusCondition(got.Status.Conditions, registryv1alpha1.ConditionValid)
	require.NotNil(t, condition)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, registryv1alpha1.ReasonResolved, condition.Reason)
	assert.EqualValues(t, 3, condition.ObservedGeneration)
}

func TestReconcileReportsAnInvalidConfig(t *testing.T) {
	// A Managed registry with neither an upstream nor a cache: the schema would
	// reject this, but an administrator editing the CR during an incident can
	// reach the controller with anything.
	cfg := &registryv1alpha1.RegistryConfig{
		ObjectMeta: metav1.ObjectMeta{Name: registryv1alpha1.SingletonName, Generation: 1},
		Spec:       registryv1alpha1.RegistryConfigSpec{Mode: registryv1alpha1.ModeManaged},
	}

	r, c := newReconciler(t, cfg)

	// No error returned: the spec will not fix itself, so requeueing would only
	// spin. The state is reported instead.
	result, err := r.Reconcile(context.Background(), requestFor(cfg.Name))
	require.NoError(t, err)
	assert.Zero(t, result.RequeueAfter)

	condition := apimeta.FindStatusCondition(
		getConfig(t, c, cfg.Name).Status.Conditions, registryv1alpha1.ConditionValid)
	require.NotNil(t, condition)
	assert.Equal(t, metav1.ConditionFalse, condition.Status)
	assert.Equal(t, registryv1alpha1.ReasonInvalidSpec, condition.Reason)
	assert.Contains(t, condition.Message, "needs a source of images")
}

func TestReconcileToleratesADeletedConfig(t *testing.T) {
	// Components keep serving images from their last applied configuration, which
	// is the whole point of the design, so a missing object is not an error.
	r, _ := newReconciler(t)

	result, err := r.Reconcile(context.Background(), requestFor(registryv1alpha1.SingletonName))
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// TestReconcileIsIdempotent guards the write-skipping in patchStatus. RegistryConfig
// is watched by the storage syncer and, indirectly, by every node, so a status
// rewritten on every reconcile would turn into a cluster-wide wake-up loop.
func TestReconcileIsIdempotent(t *testing.T) {
	cfg := &registryv1alpha1.RegistryConfig{
		ObjectMeta: metav1.ObjectMeta{Name: registryv1alpha1.SingletonName, Generation: 1},
		Spec: registryv1alpha1.RegistryConfigSpec{
			Mode:    registryv1alpha1.ModeManaged,
			Primary: registryv1alpha1.PrimarySource{Upstream: upstream()},
		},
	}

	r, c := newReconciler(t, cfg)
	ctx := context.Background()

	require.NoError(t, first(r.Reconcile(ctx, requestFor(cfg.Name))))
	afterFirst := getConfig(t, c, cfg.Name).ResourceVersion

	require.NoError(t, first(r.Reconcile(ctx, requestFor(cfg.Name))))
	afterSecond := getConfig(t, c, cfg.Name).ResourceVersion

	assert.Equal(t, afterFirst, afterSecond, "an unchanged status must not be written again")

	// Prove the assertion above is not vacuous: a real change must still be
	// written, so the resource version has to move here.
	updated := getConfig(t, c, cfg.Name)
	updated.Generation = 2
	require.NoError(t, c.Update(ctx, updated))
	beforeThird := getConfig(t, c, cfg.Name).ResourceVersion

	require.NoError(t, first(r.Reconcile(ctx, requestFor(cfg.Name))))
	assert.NotEqual(t, beforeThird, getConfig(t, c, cfg.Name).ResourceVersion,
		"a changed observedGeneration must be persisted")
}

func TestReconcileRejectsANonSingletonObject(t *testing.T) {
	// A second object that looks configured but does nothing is a trap; say so.
	cfg := &registryv1alpha1.RegistryConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "my-registry", Generation: 1},
		Spec: registryv1alpha1.RegistryConfigSpec{
			Mode:    registryv1alpha1.ModeManaged,
			Primary: registryv1alpha1.PrimarySource{Upstream: upstream()},
		},
	}

	r, c := newReconciler(t, cfg)

	_, err := r.Reconcile(context.Background(), requestFor(cfg.Name))
	require.NoError(t, err)

	condition := apimeta.FindStatusCondition(
		getConfig(t, c, cfg.Name).Status.Conditions, registryv1alpha1.ConditionValid)
	require.NotNil(t, condition)
	assert.Equal(t, metav1.ConditionFalse, condition.Status)
	assert.Contains(t, condition.Message, "only the singleton object")
}

func first(_ ctrl.Result, err error) error { return err }
