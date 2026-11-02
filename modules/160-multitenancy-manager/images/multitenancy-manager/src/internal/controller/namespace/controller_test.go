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

package namespace

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"controller/apis/deckhouse.io/v1alpha3"
	namespacemanager "controller/internal/manager/namespace"
)

func newReconciler(t *testing.T, objs ...client.Object) *reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, v1alpha3.AddToScheme} {
		require.NoError(t, add(scheme))
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &reconciler{
		init:    new(sync.WaitGroup), // counter 0: Wait() returns immediately
		logger:  logr.Discard(),
		client:  c,
		manager: namespacemanager.New(c, logr.Discard()),
	}
}

func TestIsAdoptionCandidate(t *testing.T) {
	cases := []struct {
		name string
		ns   *corev1.Namespace
		want bool
	}{
		{name: "plain user namespace", ns: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}, want: true},
		{name: "default namespace", ns: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}, want: false},
		// The Project of that name is the virtual one: Adopt would find it and leave the namespace out of
		// every project, and the namespace would be reported as waiting for adoption for good.
		{name: "namespace named like the deckhouse virtual project", ns: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "deckhouse"}}, want: false},
		{name: "name that only starts like a virtual project", ns: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "deckhouse-apps"}}, want: true},
		{name: "reserved d8 prefix", ns: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "d8-system"}}, want: false},
		{name: "reserved kube prefix", ns: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}}, want: false},
		{
			name: "deckhouse heritage",
			ns:   &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "module-ns", Labels: map[string]string{v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageDeckhouse}}},
			want: false,
		},
		{
			// A namespace already owned by a project (its main namespace or an additional namespace
			// created by a ProjectNamespace) must never become a separate project.
			name: "project-owned namespace is skipped",
			ns:   &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "owned", Labels: map[string]string{v1alpha3.ResourceLabelProject: "owned", v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageMultitenancy}}},
			want: false,
		},
		{
			name: "additional project namespace is skipped",
			ns:   &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a-backend", Labels: map[string]string{v1alpha3.ResourceLabelProject: "team-a", v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageMultitenancy}}},
			want: false,
		},
		{
			name: "an empty project label keeps the namespace out of adoption",
			ns:   &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team", Labels: map[string]string{v1alpha3.ResourceLabelProject: ""}}},
			want: false,
		},
		{
			name: "upmeter heritage",
			ns:   &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "obsv", Labels: map[string]string{v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageUpmeter}}},
			want: false,
		},
		{
			name: "upmeter name prefix",
			ns:   &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "upmeter-probe-namespace-foo"}},
			want: false,
		},
		{
			name: "name longer than the project limit",
			ns:   &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: strings.Repeat("a", v1alpha3.ProjectNameMaxLength+1)}},
			want: false,
		},
		{
			name: "name at the project limit is still a candidate",
			ns:   &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: strings.Repeat("a", v1alpha3.ProjectNameMaxLength)}},
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isAdoptionCandidate(tc.ns))
		})
	}
}

func TestReconcile_HoldsOffYoungOrphanNamespace(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:              "team-a",
		CreationTimestamp: metav1.Now(),
	}}

	r := newReconciler(t, ns)
	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "team-a"}})
	require.NoError(t, err)
	assert.Greater(t, res.RequeueAfter, time.Duration(0))

	err = r.client.Get(context.Background(), client.ObjectKey{Name: "team-a"}, new(v1alpha3.Project))
	assert.True(t, apierrors.IsNotFound(err), "a young namespace must not be adopted yet")
}

func TestReconcile_AdoptsOrphanNamespace(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   "team-a",
		Labels: map[string]string{"env": "prod"},
	}}

	r := newReconciler(t, ns)
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "team-a"}})
	require.NoError(t, err)

	created := new(v1alpha3.Project)
	require.NoError(t, r.client.Get(context.Background(), client.ObjectKey{Name: "team-a"}, created))
	assert.Equal(t, namespacemanager.TemplateSimple, created.Spec.ProjectTemplateName)
	assert.NotContains(t, created.Spec.Parameters, "namespace", "the labels of the namespace stay its own")
}

func TestReconcile_SkipsNamespaceOwnedByProject(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "team-a",
		Labels: map[string]string{
			v1alpha3.ResourceLabelProject:  "team-a",
			v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageMultitenancy,
		},
	}}

	r := newReconciler(t, ns)
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "team-a"}})
	require.NoError(t, err)

	err = r.client.Get(context.Background(), client.ObjectKey{Name: "team-a"}, new(v1alpha3.Project))
	assert.True(t, apierrors.IsNotFound(err), "an owned namespace must not get a project of its own")
}

func TestReconcile_ClearsRetiredMarkersOnTerminatingNamespace(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "team-a",
		Labels: map[string]string{
			v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace,
		},
		Finalizers: []string{v1alpha3.NamespaceFinalizerManagedProject, "example.com/keep"},
	}}
	r := newReconciler(t, ns)
	require.NoError(t, r.client.Delete(context.Background(), ns))

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "team-a"}})
	require.NoError(t, err)

	updated := new(corev1.Namespace)
	require.NoError(t, r.client.Get(context.Background(), client.ObjectKey{Name: "team-a"}, updated))
	assert.NotContains(t, updated.Labels, v1alpha3.ProjectLabelManagedByNamespace)
	assert.False(t, controllerutil.ContainsFinalizer(updated, v1alpha3.NamespaceFinalizerManagedProject))
	assert.True(t, controllerutil.ContainsFinalizer(updated, "example.com/keep"))

	err = r.client.Get(context.Background(), client.ObjectKey{Name: "team-a"}, new(v1alpha3.Project))
	assert.True(t, apierrors.IsNotFound(err), "a terminating namespace must not be adopted")
}

func TestReconcile_ClearsRetiredMarkersWhenAlreadyOwned(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "team-a",
		Labels: map[string]string{
			v1alpha3.ResourceLabelProject:           "team-a",
			v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace,
		},
		Finalizers: []string{v1alpha3.NamespaceFinalizerManagedProject},
	}}
	existing := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a"},
		Spec:       v1alpha3.ProjectSpec{ProjectTemplateName: namespacemanager.TemplateSimple},
	}
	r := newReconciler(t, ns, existing)

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "team-a"}})
	require.NoError(t, err)

	updated := new(corev1.Namespace)
	require.NoError(t, r.client.Get(context.Background(), client.ObjectKey{Name: "team-a"}, updated))
	assert.NotContains(t, updated.Labels, v1alpha3.ProjectLabelManagedByNamespace)
	assert.False(t, controllerutil.ContainsFinalizer(updated, v1alpha3.NamespaceFinalizerManagedProject))
}

func TestReconcile_SkipsTerminatingNamespace(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:       "team-a",
		Finalizers: []string{"example.com/keep"},
	}}
	r := newReconciler(t, ns)
	require.NoError(t, r.client.Delete(context.Background(), ns))

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "team-a"}})
	require.NoError(t, err)

	err = r.client.Get(context.Background(), client.ObjectKey{Name: "team-a"}, new(v1alpha3.Project))
	assert.True(t, apierrors.IsNotFound(err), "a namespace on its way out must not be adopted")
}

func TestPredicate(t *testing.T) {
	orphan := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}
	system := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}}
	owned := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "owned", Labels: map[string]string{v1alpha3.ResourceLabelProject: "owned"}}}

	p := customPredicate[*corev1.Namespace]{logger: logr.Discard()}

	t.Run("orphan namespace is handled", func(t *testing.T) {
		assert.True(t, p.Create(event.TypedCreateEvent[*corev1.Namespace]{Object: orphan}))
	})

	t.Run("system namespace is ignored", func(t *testing.T) {
		assert.False(t, p.Create(event.TypedCreateEvent[*corev1.Namespace]{Object: system}))
	})

	t.Run("project-owned namespace is ignored", func(t *testing.T) {
		assert.False(t, p.Update(event.TypedUpdateEvent[*corev1.Namespace]{ObjectOld: owned, ObjectNew: owned}))
	})

	t.Run("leftover markers still enqueue a project-owned namespace", func(t *testing.T) {
		leftover := owned.DeepCopy()
		leftover.Finalizers = []string{v1alpha3.NamespaceFinalizerManagedProject}
		assert.True(t, p.Update(event.TypedUpdateEvent[*corev1.Namespace]{ObjectOld: leftover, ObjectNew: leftover}))
	})

	t.Run("delete events are never handled", func(t *testing.T) {
		assert.False(t, p.Delete(event.TypedDeleteEvent[*corev1.Namespace]{Object: orphan}))
	})
}

// A namespace whose adoption keeps failing is tried again at least every maxRetryDelay, not after
// the minutes the default backoff grows to, so it joins its project soon after what refused it is gone.
func TestRetryLimiterCapsTheDelay(t *testing.T) {
	options := controllerOptions()
	assert.Equal(t, namespacemanager.WorkLimit, options.MaxConcurrentReconciles)
	limiter := options.RateLimiter
	item := reconcile.Request{NamespacedName: types.NamespacedName{Name: "stuck"}}
	var delay time.Duration
	for range 30 {
		delay = limiter.When(item)
	}
	assert.Equal(t, maxRetryDelay, delay)

	limiter.Forget(item)
	assert.Less(t, limiter.When(item), time.Second, "an adopted namespace starts the backoff anew")
}

// The options apply to the controller that runs the queue: Register makes one controller, under its
// own name, and no other one that the builder would make around it under the name of the kind.
func TestRegisterMakesOneController(t *testing.T) {
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, v1alpha3.AddToScheme} {
		require.NoError(t, add(scheme))
	}
	mgr, err := ctrl.NewManager(&rest.Config{Host: "https://127.0.0.1:1"}, ctrl.Options{Scheme: scheme,
		Metrics: metricsserver.Options{BindAddress: "0"}, HealthProbeBindAddress: "0"})
	require.NoError(t, err)
	require.NoError(t, Register(mgr, logr.Discard(), nil))

	nothing := reconcile.Func(func(context.Context, reconcile.Request) (reconcile.Result, error) { return reconcile.Result{}, nil })
	_, err = controller.New(controllerName, mgr, controller.Options{Reconciler: nothing})
	require.Error(t, err, "the namespace controller runs under its own name")
	_, err = controller.New("namespace", mgr, controller.Options{Reconciler: nothing})
	assert.NoError(t, err, "no second controller under the name of the kind")
}
