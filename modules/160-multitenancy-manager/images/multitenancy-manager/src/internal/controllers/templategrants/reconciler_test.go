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
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache/informertest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllertest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	grantsv1alpha1 "controller/api/v1alpha1"
	grantsv1alpha2 "controller/api/v1alpha2"
	deckhousev1alpha2 "controller/apis/deckhouse.io/v1alpha2"
)

func newReconciler(t *testing.T, objs ...client.Object) (*Reconciler, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		deckhousev1alpha2.AddToScheme, grantsv1alpha1.AddToScheme, grantsv1alpha2.AddToScheme,
	} {
		require.NoError(t, add(scheme))
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &Reconciler{Client: c, Scheme: scheme}, c
}

func template(name string, spec deckhousev1alpha2.ProjectTemplateSpec) *deckhousev1alpha2.ProjectTemplate {
	return &deckhousev1alpha2.ProjectTemplate{ObjectMeta: metav1.ObjectMeta{Name: name}, Spec: spec}
}

func libraryPolicy(name string, resources ...grantsv1alpha1.GrantResource) *grantsv1alpha2.ClusterResourceGrantPolicy {
	return &grantsv1alpha2.ClusterResourceGrantPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       grantsv1alpha2.ClusterResourceGrantPolicySpec{Resources: resources},
	}
}

func runReconcile(t *testing.T, r *Reconciler, name string) ctrl.Result {
	t.Helper()
	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: name}})
	require.NoError(t, err)
	return res
}

func getPolicy(t *testing.T, c client.Client, name string) *grantsv1alpha2.ClusterResourceGrantPolicy {
	t.Helper()
	p := &grantsv1alpha2.ClusterResourceGrantPolicy{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: name}, p))
	return p
}

// lockedInformer serialises the handler registration the controller does with the events the test
// fires (the fake informer is not safe for concurrent use), and reports when the expected number of
// handlers is in place.
type lockedInformer struct {
	*controllertest.FakeInformer

	mu    sync.Mutex
	want  int
	count int
	ready chan struct{}
}

func newLockedInformer(want int) *lockedInformer {
	return &lockedInformer{FakeInformer: &controllertest.FakeInformer{Synced: true}, want: want, ready: make(chan struct{})}
}

func (l *lockedInformer) AddEventHandlerWithOptions(
	h toolscache.ResourceEventHandler,
	o toolscache.HandlerOptions,
) (toolscache.ResourceEventHandlerRegistration, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	reg, err := l.FakeInformer.AddEventHandlerWithOptions(h, o)
	if l.count++; l.count == l.want {
		close(l.ready)
	}

	return reg, err
}

func (l *lockedInformer) Update(oldObj, newObj metav1.Object) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.FakeInformer.Update(oldObj, newObj)
}

func (l *lockedInformer) Delete(obj metav1.Object) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.FakeInformer.Delete(obj)
}

// The watch SetupWithManager registers is what brings a library policy edit to the templates: a spec
// edit wakes the template, an update that keeps the generation (a status write) is filtered out, and
// a delete passes the predicate and prunes the copy.
func TestSetupWithManager_LibraryPolicyWatch(t *testing.T) {
	lib := libraryPolicy("lib", grantsv1alpha1.GrantResource{ResourceName: "storageclasses", Allowed: []string{"fast", "slow"}})
	r, c := newReconciler(t, template("a", deckhousev1alpha2.ProjectTemplateSpec{GrantPolicies: []string{"lib"}}), lib)
	runReconcile(t, r, "a")

	// Owns and Watches both register a handler on the policy informer.
	policies := newLockedInformer(2)
	informers := &informertest.FakeInformers{Scheme: r.Scheme, InformersByGVK: map[schema.GroupVersionKind]toolscache.SharedIndexInformer{
		grantsv1alpha2.GroupVersion.WithKind("ClusterResourceGrantPolicy"): policies,
		deckhousev1alpha2.SchemeGroupVersion.WithKind("ProjectTemplate"):   &controllertest.FakeInformer{Synced: true},
	}}
	mgr, err := manager.New(&rest.Config{Host: "http://127.0.0.1:1"}, manager.Options{
		Scheme:     r.Scheme,
		Metrics:    metricsserver.Options{BindAddress: "0"},
		Controller: config.Controller{SkipNameValidation: ptr.To(true)},
		MapperProvider: func(*rest.Config, *http.Client) (meta.RESTMapper, error) {
			return meta.NewDefaultRESTMapper(nil), nil
		},
		NewCache:  func(*rest.Config, cache.Options) (cache.Cache, error) { return informers, nil },
		NewClient: func(*rest.Config, client.Options) (client.Client, error) { return c, nil },
	})
	require.NoError(t, err)
	require.NoError(t, r.SetupWithManager(mgr))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go func() { _ = mgr.Start(ctx) }()

	select {
	case <-policies.ready:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "the controller did not register its policy handlers")
	}

	allowed := func() []string { return getPolicy(t, c, PolicyName("a", "lib")).Spec.Resources[0].Allowed }
	edit := func(values ...string) (*grantsv1alpha2.ClusterResourceGrantPolicy, *grantsv1alpha2.ClusterResourceGrantPolicy) {
		cur := getPolicy(t, c, "lib")
		old := cur.DeepCopy()
		cur.Spec.Resources[0].Allowed = values
		require.NoError(t, c.Update(ctx, cur))

		return old, cur
	}

	old, cur := edit("slow")
	cur.Generation = old.Generation + 1
	policies.Update(old, cur)
	require.Eventually(t, func() bool { return len(allowed()) == 1 }, 2*time.Second, 20*time.Millisecond, "a spec edit reaches the copy")

	old, cur = edit("fast", "slow")
	cur.Generation = old.Generation
	policies.Update(old, cur)
	assert.Never(t, func() bool { return len(allowed()) == 2 }, 300*time.Millisecond, 20*time.Millisecond, "an update that keeps the generation is filtered")

	require.NoError(t, c.Delete(ctx, cur))
	policies.Delete(cur)
	assert.Eventually(t, func() bool {
		err := c.Get(ctx, client.ObjectKey{Name: PolicyName("a", "lib")}, &grantsv1alpha2.ClusterResourceGrantPolicy{})
		return apierrors.IsNotFound(err)
	}, 2*time.Second, 20*time.Millisecond, "a deleted library policy takes its copy along")
}

// A changed grant policy wakes exactly the templates that reference it; a managed copy wakes none,
// because Owns already maps it to its template.
func TestTemplatesReferencing(t *testing.T) {
	r, _ := newReconciler(t,
		template("a", deckhousev1alpha2.ProjectTemplateSpec{GrantPolicies: []string{"lib", "other"}}),
		template("b", deckhousev1alpha2.ProjectTemplateSpec{GrantPolicies: []string{"lib"}}),
		template("c", deckhousev1alpha2.ProjectTemplateSpec{GrantPolicies: []string{"other"}}),
		template("d", deckhousev1alpha2.ProjectTemplateSpec{}),
	)
	// The label decides, not the name: an object carrying it is skipped even under a referenced name.
	managedCopy := libraryPolicy("lib")
	managedCopy.Labels = map[string]string{LabelManagedByTemplate: "a"}

	tests := []struct {
		name     string
		policy   *grantsv1alpha2.ClusterResourceGrantPolicy
		expected []string
	}{
		{name: "a library policy referenced by two templates", policy: libraryPolicy("lib"), expected: []string{"a", "b"}},
		{name: "a policy no template references", policy: libraryPolicy("unused")},
		{name: "an object with the managed-by-template label", policy: managedCopy},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			names := []string{}
			for _, req := range r.templatesReferencing(context.Background(), tt.policy) {
				names = append(names, req.Name)
			}
			assert.ElementsMatch(t, tt.expected, names)
		})
	}
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
	require.NotNil(t, inline.Spec.NamespaceSelector)
	assert.Equal(t, "grants-demo", inline.Spec.NamespaceSelector.MatchLabels[deckhousev1alpha2.ResourceLabelTemplate])
	assert.Nil(t, inline.Spec.ProjectSelector)
	require.Len(t, inline.Spec.Resources, 1)
	assert.Equal(t, "storageclasses", inline.Spec.Resources[0].ResourceName)
	require.Len(t, inline.OwnerReferences, 1)
	assert.Equal(t, "grants-demo", inline.OwnerReferences[0].Name)
	assert.Equal(t, deckhousev1alpha2.ProjectTemplateKind, inline.OwnerReferences[0].Kind)

	fromLib := getPolicy(t, c, "template-grants-demo-library-issuers")
	assert.Equal(t, grantSourcePolicy, fromLib.Labels[LabelGrantSource])
	require.Len(t, fromLib.Spec.Resources, 1)
	assert.Equal(t, "clusterissuers", fromLib.Spec.Resources[0].ResourceName)
	require.NotNil(t, fromLib.Spec.NamespaceSelector)
	assert.Equal(t, "grants-demo", fromLib.Spec.NamespaceSelector.MatchLabels[deckhousev1alpha2.ResourceLabelTemplate])
	assert.Nil(t, fromLib.Spec.ProjectSelector)
}

// A template with neither inline resources nor grantPolicies owns no managed policies.
func TestReconcile_NoSourcesNoPolicies(t *testing.T) {
	r, c := newReconciler(t, template("empty", deckhousev1alpha2.ProjectTemplateSpec{}))
	require.False(t, runReconcile(t, r, "empty").Requeue)

	list := &grantsv1alpha2.ClusterResourceGrantPolicyList{}
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
	err := c.Get(context.Background(), client.ObjectKey{Name: "template-demo-lib"}, &grantsv1alpha2.ClusterResourceGrantPolicy{})
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
	assert.Positive(t, res.RequeueAfter, "a missing reference must requeue")

	getPolicy(t, c, "template-demo-inline") // inline still materialized
	err = c.Get(context.Background(), client.ObjectKey{Name: "template-demo-absent"}, &grantsv1alpha2.ClusterResourceGrantPolicy{})
	assert.True(t, apierrors.IsNotFound(err))
}

// Deleting the template is a no-op for the reconciler (owner references garbage-collect the policies).
func TestReconcile_TemplateGoneIsNoop(t *testing.T) {
	r, _ := newReconciler(t)
	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "ghost"}})
	require.NoError(t, err)
	assert.False(t, res.Requeue)
	assert.Zero(t, res.RequeueAfter)
}

// TestReconcile_ManagedPolicyOwnsBothSelectors: the reconciler owns both selectors of the policies it
// materializes. A managed policy an earlier build wrote with a projectSelector on the Project label is
// moved to the namespaceSelector, not left with both.
func TestReconcile_ManagedPolicyOwnsBothSelectors(t *testing.T) {
	tmpl := template("grants-demo", deckhousev1alpha2.ProjectTemplateSpec{
		Resources: []grantsv1alpha1.GrantResource{{ResourceName: "storageclasses", Allowed: []string{"fast"}}},
	})
	earlier := &grantsv1alpha2.ClusterResourceGrantPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "template-grants-demo-inline"},
		Spec: grantsv1alpha2.ClusterResourceGrantPolicySpec{
			ProjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{deckhousev1alpha2.ResourceLabelTemplate: "grants-demo"}},
			Resources:       []grantsv1alpha1.GrantResource{{ResourceName: "storageclasses", Allowed: []string{"fast"}}},
		},
	}
	r, c := newReconciler(t, tmpl, earlier)
	runReconcile(t, r, "grants-demo")

	got := getPolicy(t, c, "template-grants-demo-inline")
	require.NotNil(t, got.Spec.NamespaceSelector)
	assert.Equal(t, "grants-demo", got.Spec.NamespaceSelector.MatchLabels[deckhousev1alpha2.ResourceLabelTemplate])
	assert.Nil(t, got.Spec.ProjectSelector, "the projectSelector of an earlier build must be cleared")
}

// An edit of a library policy reaches the managed copy of every template that references it: the
// watch maps the policy to those templates, and their reconcile copies the new resources.
func TestReconcile_ManagedCopyFollowsLibraryEdit(t *testing.T) {
	lib := libraryPolicy("lib", grantsv1alpha1.GrantResource{ResourceName: "storageclasses", Allowed: []string{"fast", "slow"}})
	r, c := newReconciler(t,
		template("a", deckhousev1alpha2.ProjectTemplateSpec{GrantPolicies: []string{"lib"}}),
		template("b", deckhousev1alpha2.ProjectTemplateSpec{GrantPolicies: []string{"lib"}}),
		lib,
	)
	runReconcile(t, r, "a")
	runReconcile(t, r, "b")

	cur := &grantsv1alpha2.ClusterResourceGrantPolicy{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "lib"}, cur))
	cur.Spec.Resources[0].Allowed = []string{"slow"}
	require.NoError(t, c.Update(context.Background(), cur))

	requests := r.templatesReferencing(context.Background(), cur)
	require.Len(t, requests, 2)
	for _, req := range requests {
		runReconcile(t, r, req.Name)
	}

	for _, tmpl := range []string{"a", "b"} {
		managed := getPolicy(t, c, PolicyName(tmpl, "lib"))
		require.Len(t, managed.Spec.Resources, 1, tmpl)
		assert.Equal(t, []string{"slow"}, managed.Spec.Resources[0].Allowed, tmpl)
	}
}

// Deleting a library policy prunes its managed copy at once: the deleted object still maps to the
// templates that reference it, and their reconcile finds the reference gone.
func TestReconcile_ManagedCopyGoesWithDeletedLibrary(t *testing.T) {
	lib := libraryPolicy("lib", grantsv1alpha1.GrantResource{ResourceName: "storageclasses", Allowed: []string{"fast"}})
	r, c := newReconciler(t, template("a", deckhousev1alpha2.ProjectTemplateSpec{GrantPolicies: []string{"lib"}}), lib)
	runReconcile(t, r, "a")
	getPolicy(t, c, PolicyName("a", "lib"))

	require.NoError(t, c.Delete(context.Background(), lib))
	requests := r.templatesReferencing(context.Background(), lib)
	require.Len(t, requests, 1)
	res := runReconcile(t, r, requests[0].Name)

	assert.Positive(t, res.RequeueAfter, "the template still references the deleted policy")
	err := c.Get(context.Background(), client.ObjectKey{Name: PolicyName("a", "lib")}, &grantsv1alpha2.ClusterResourceGrantPolicy{})
	assert.True(t, apierrors.IsNotFound(err), "the managed copy of a deleted library policy must be pruned")
}
