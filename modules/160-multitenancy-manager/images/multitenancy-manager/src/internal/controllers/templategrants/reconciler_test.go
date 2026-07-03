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

package templategrants

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	grantsv1alpha1 "controller/api/v1alpha1"
	deckhousev1alpha2 "controller/apis/deckhouse.io/v1alpha2"
)

func newReconciler(t *testing.T, objs ...client.Object) (*Reconciler, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		deckhousev1alpha2.AddToScheme, grantsv1alpha1.AddToScheme,
	} {
		require.NoError(t, add(scheme))
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &Reconciler{Client: c, Scheme: scheme}, c
}

func template(name string, spec deckhousev1alpha2.ProjectTemplateSpec) *deckhousev1alpha2.ProjectTemplate {
	return &deckhousev1alpha2.ProjectTemplate{ObjectMeta: metav1.ObjectMeta{Name: name}, Spec: spec}
}

func libraryPolicy(name string, resources ...grantsv1alpha1.GrantResource) *grantsv1alpha1.ClusterResourceGrantPolicy {
	return &grantsv1alpha1.ClusterResourceGrantPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       grantsv1alpha1.ClusterResourceGrantPolicySpec{Resources: resources},
	}
}

func runReconcile(t *testing.T, r *Reconciler, name string) ctrl.Result {
	t.Helper()
	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: name}})
	require.NoError(t, err)
	return res
}

func getPolicy(t *testing.T, c client.Client, name string) *grantsv1alpha1.ClusterResourceGrantPolicy {
	t.Helper()
	p := &grantsv1alpha1.ClusterResourceGrantPolicy{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: name}, p))
	return p
}

// One managed policy per source (inline + each grantPolicy), never merged; each targets the template
// label, copies the source resources, and is owned by the template.
func TestReconcile_MaterializesOnePolicyPerSource(t *testing.T) {
	lib := libraryPolicy("library-issuers", grantsv1alpha1.GrantResource{ResourceName: "clusterissuers", Allowed: []string{"letsencrypt"}})
	tmpl := template("grants-demo", deckhousev1alpha2.ProjectTemplateSpec{
		Resources:     []grantsv1alpha1.GrantResource{{ResourceName: "storageclasses", Allowed: []string{"standard"}, Default: "standard"}},
		GrantPolicies: []string{"library-issuers"},
	})
	r, c := newReconciler(t, tmpl, lib)

	require.False(t, runReconcile(t, r, "grants-demo").Requeue)

	inline := getPolicy(t, c, "template-grants-demo-inline")
	assert.Equal(t, "grants-demo", inline.Labels[LabelManagedByTemplate])
	assert.Equal(t, GrantSourceInline, inline.Labels[LabelGrantSource])
	require.NotNil(t, inline.Spec.ProjectSelector)
	assert.Equal(t, "grants-demo", inline.Spec.ProjectSelector.MatchLabels[deckhousev1alpha2.ResourceLabelTemplate])
	require.Len(t, inline.Spec.Resources, 1)
	assert.Equal(t, "storageclasses", inline.Spec.Resources[0].ResourceName)
	require.Len(t, inline.OwnerReferences, 1)
	assert.Equal(t, "grants-demo", inline.OwnerReferences[0].Name)
	assert.Equal(t, deckhousev1alpha2.ProjectTemplateKind, inline.OwnerReferences[0].Kind)

	fromLib := getPolicy(t, c, "template-grants-demo-library-issuers")
	assert.Equal(t, grantSourcePolicy, fromLib.Labels[LabelGrantSource])
	require.Len(t, fromLib.Spec.Resources, 1)
	assert.Equal(t, "clusterissuers", fromLib.Spec.Resources[0].ResourceName)
	require.NotNil(t, fromLib.Spec.ProjectSelector)
	assert.Equal(t, "grants-demo", fromLib.Spec.ProjectSelector.MatchLabels[deckhousev1alpha2.ResourceLabelTemplate])
}

// A template with neither inline resources nor grantPolicies owns no managed policies.
func TestReconcile_NoSourcesNoPolicies(t *testing.T) {
	r, c := newReconciler(t, template("empty", deckhousev1alpha2.ProjectTemplateSpec{}))
	require.False(t, runReconcile(t, r, "empty").Requeue)

	list := &grantsv1alpha1.ClusterResourceGrantPolicyList{}
	require.NoError(t, c.List(context.Background(), list))
	assert.Empty(t, list.Items)
}

// Dropping a source on the next reconcile prunes exactly that managed policy.
func TestReconcile_PrunesRemovedSource(t *testing.T) {
	tmpl := template("demo", deckhousev1alpha2.ProjectTemplateSpec{
		Resources:     []grantsv1alpha1.GrantResource{{ResourceName: "storageclasses"}},
		GrantPolicies: []string{"lib"},
	})
	r, c := newReconciler(t, tmpl, libraryPolicy("lib", grantsv1alpha1.GrantResource{ResourceName: "clusterissuers"}))
	runReconcile(t, r, "demo")
	getPolicy(t, c, "template-demo-lib") // present after first pass

	// Drop the grantPolicies reference and reconcile again.
	cur := &deckhousev1alpha2.ProjectTemplate{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "demo"}, cur))
	cur.Spec.GrantPolicies = nil
	require.NoError(t, c.Update(context.Background(), cur))
	runReconcile(t, r, "demo")

	getPolicy(t, c, "template-demo-inline") // inline survives
	err := c.Get(context.Background(), client.ObjectKey{Name: "template-demo-lib"}, &grantsv1alpha1.ClusterResourceGrantPolicy{})
	assert.True(t, apierrors.IsNotFound(err), "managed policy of the removed source must be pruned")
}

// A missing referenced library policy requeues but still materializes the resolvable sources.
func TestReconcile_MissingReferenceRequeues(t *testing.T) {
	tmpl := template("demo", deckhousev1alpha2.ProjectTemplateSpec{
		Resources:     []grantsv1alpha1.GrantResource{{ResourceName: "storageclasses"}},
		GrantPolicies: []string{"absent"},
	})
	r, c := newReconciler(t, tmpl)

	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "demo"}})
	require.NoError(t, err)
	assert.True(t, res.Requeue, "a missing reference must requeue")

	getPolicy(t, c, "template-demo-inline") // inline still materialized
	err = c.Get(context.Background(), client.ObjectKey{Name: "template-demo-absent"}, &grantsv1alpha1.ClusterResourceGrantPolicy{})
	assert.True(t, apierrors.IsNotFound(err))
}

// Deleting the template is a no-op for the reconciler (owner references garbage-collect the policies).
func TestReconcile_TemplateGoneIsNoop(t *testing.T) {
	r, _ := newReconciler(t)
	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "ghost"}})
	require.NoError(t, err)
	assert.False(t, res.Requeue)
}
