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

package webhooks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"controller/api/v1alpha1"
	grantsv1alpha2 "controller/api/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/jsonpath"
)

// calls counts the reads a fake client serves, by method and type.
type calls struct {
	mu sync.Mutex
	n  map[string]int
}

func (c *calls) add(method string, obj runtime.Object) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.n == nil {
		c.n = map[string]int{}
	}
	c.n[fmt.Sprintf("%s %T", method, obj)]++
}

func (c *calls) total() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, v := range c.n {
		n += v
	}
	return n
}

func (c *calls) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n = nil
}

func (c *calls) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return fmt.Sprint(c.n)
}

// countingClient is a fake client holding objs that counts every Get and List.
func countingClient(t *testing.T, objs ...client.Object) (client.Client, *calls) {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		corev1.AddToScheme, storagev1.AddToScheme, v1alpha1.AddToScheme, grantsv1alpha2.AddToScheme, v1alpha3.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	c := &calls{}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			c.add("get", obj)
			return cl.Get(ctx, key, obj, opts...)
		},
		List: func(ctx context.Context, cl client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			c.add("list", list)
			return cl.List(ctx, list, opts...)
		},
	}).Build()
	return cl, c
}

// modelFixture is a ModelReader over a fake cache and a fake API server, both holding objs, with the
// informers synced or not.
type modelFixture struct {
	reader *ModelReader
	cache  *calls
	direct *calls
}

func newModelFixture(t *testing.T, synced bool, objs ...client.Object) *modelFixture {
	t.Helper()
	cacheCl, cacheCalls := countingClient(t, objs...)
	directCl, directCalls := countingClient(t, objs...)
	s := map[modelKind]func() bool{}
	for kind := range modelObjects() {
		s[kind] = func() bool { return synced }
	}
	return &modelFixture{
		reader: &ModelReader{cache: cacheCl, direct: directCl, synced: s},
		cache:  cacheCalls,
		direct: directCalls,
	}
}

func TestModelReader_UnsyncedReadsTheAPIServer(t *testing.T) {
	f := newModelFixture(t, false, projectNS("proj", nil))
	ctx := context.Background()
	if err := f.reader.Get(ctx, client.ObjectKey{Name: "proj"}, &corev1.Namespace{}); err != nil {
		t.Fatal(err)
	}
	if err := f.reader.List(ctx, &v1alpha1.GrantableClusterResourceReferenceList{}); err != nil {
		t.Fatal(err)
	}
	if f.cache.total() != 0 || f.direct.total() != 2 {
		t.Fatalf("an unsynced informer must never be read: cache %v, direct %v", f.cache, f.direct)
	}
}

func TestModelReader_SyncedReadsTheCache(t *testing.T) {
	f := newModelFixture(t, true, projectNS("proj", nil), &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}})
	ctx := context.Background()
	if err := f.reader.Get(ctx, client.ObjectKey{Name: "proj"}, &corev1.Namespace{}); err != nil {
		t.Fatal(err)
	}
	if err := f.reader.Get(ctx, client.ObjectKey{Name: "proj"}, &v1alpha3.Project{}); err != nil {
		t.Fatal(err)
	}
	for _, l := range []client.ObjectList{
		&v1alpha1.GrantableClusterResourceReferenceList{},
		&v1alpha1.GrantableClusterResourceDefinitionList{},
		&grantsv1alpha2.ClusterResourceGrantPolicyList{},
	} {
		if err := f.reader.List(ctx, l); err != nil {
			t.Fatal(err)
		}
	}
	if f.cache.total() != 5 || f.direct.total() != 0 {
		t.Fatalf("a synced model must be read from the cache only: cache %v, direct %v", f.cache, f.direct)
	}
	// A kind outside the model is always read from the API server.
	if err := f.reader.Get(ctx, client.ObjectKey{Name: "x"}, &storagev1.StorageClass{}); err == nil {
		t.Fatal("expected NotFound")
	}
	if f.direct.total() != 1 {
		t.Fatalf("a kind outside the model must be read from the API server: direct %v", f.direct)
	}
}

// A namespace the watch has not delivered yet is asked again from the API server: a NotFound would
// allow the request.
func TestModelReader_CacheMissAsksTheAPIServer(t *testing.T) {
	cacheCl, cacheCalls := countingClient(t)
	directCl, directCalls := countingClient(t, projectNS("proj", nil))
	r := &ModelReader{cache: cacheCl, direct: directCl, synced: map[modelKind]func() bool{modelNamespace: func() bool { return true }}}
	ns := &corev1.Namespace{}
	if err := r.Get(context.Background(), client.ObjectKey{Name: "proj"}, ns); err != nil {
		t.Fatalf("a namespace missing from the cache must be read from the API server: %v", err)
	}
	if cacheCalls.total() != 1 || directCalls.total() != 1 {
		t.Fatalf("cache %v, direct %v", cacheCalls, directCalls)
	}
}

var (
	podGVR    = metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	podGVK    = metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
	deployGVR = metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	deployGVK = metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
)

// rcDef is a value-backed definition of runtime class names: no granted objects are read, so the
// counts below are of the grant model alone.
func rcDef() *v1alpha1.GrantableClusterResourceDefinition {
	return &v1alpha1.GrantableClusterResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "runtimeclasses"},
		Spec:       v1alpha1.GrantableClusterResourceDefinitionSpec{DefaultAvailability: v1alpha1.AvailabilityNone},
	}
}

// rcRefs are the pod and Deployment references of !1409/!1410 in shape: a path in the pod spec and one
// in the pod template, the latter defaulted with FillEmpty.
func rcRefs() []client.Object {
	return []client.Object{
		&v1alpha1.GrantableClusterResourceReference{
			ObjectMeta: metav1.ObjectMeta{Name: "runtimeclasses-core"},
			Spec: v1alpha1.GrantableClusterResourceReferenceSpec{
				GrantableClusterResourceName: "runtimeclasses",
				Rule:                         v1alpha1.UsageRule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"pods"}},
				FieldPaths:                   []v1alpha1.FieldPath{{Path: "$.spec.runtimeClassName", Defaulting: v1alpha1.DefaultingNone}},
			},
		},
		&v1alpha1.GrantableClusterResourceReference{
			ObjectMeta: metav1.ObjectMeta{Name: "runtimeclasses-apps"},
			Spec: v1alpha1.GrantableClusterResourceReferenceSpec{
				GrantableClusterResourceName: "runtimeclasses",
				Rule:                         v1alpha1.UsageRule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"}},
				FieldPaths:                   []v1alpha1.FieldPath{{Path: "$.spec.template.spec.runtimeClassName", Defaulting: v1alpha1.DefaultingFillEmpty}},
			},
		},
	}
}

func rcGrant() *grantsv1alpha2.ClusterResourceGrantPolicy {
	return &grantsv1alpha2.ClusterResourceGrantPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "g"},
		Spec: grantsv1alpha2.ClusterResourceGrantPolicySpec{
			NamespaceSelector: &metav1.LabelSelector{},
			Resources: []v1alpha1.GrantResource{{
				ResourceName: "runtimeclasses",
				Allowed:      []string{"gvisor", "runc"},
				Default:      "runc",
			}},
		},
	}
}

func rcModel() []client.Object {
	return append(rcRefs(), rcDef(), rcGrant(), projectNS("proj", nil), &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}})
}

func pod(t *testing.T, runtimeClass string, annotations map[string]any) []byte {
	t.Helper()
	spec := map[string]any{"containers": []any{map[string]any{"name": "c", "image": "busybox"}}}
	if runtimeClass != "" {
		spec["runtimeClassName"] = runtimeClass
	}
	meta := map[string]any{"name": "p", "namespace": "proj"}
	if annotations != nil {
		meta["annotations"] = annotations
	}
	return raw(t, map[string]any{"apiVersion": "v1", "kind": "Pod", "metadata": meta, "spec": spec})
}

func deployment(t *testing.T, runtimeClass string) []byte {
	t.Helper()
	podSpec := map[string]any{"containers": []any{map[string]any{"name": "c", "image": "busybox"}}}
	if runtimeClass != "" {
		podSpec["runtimeClassName"] = runtimeClass
	}
	return raw(t, map[string]any{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]any{"name": "d", "namespace": "proj"},
		"spec":     map[string]any{"template": map[string]any{"spec": podSpec}},
	})
}

// TestGrantWebhooks_ReadsPerRequest pins the reads of the grant webhooks per admission request, through
// the API server (unsynced, the reader at start) and through the informers (synced). Before the
// ModelReader every request read the API server five times (two lists of the references and the
// definitions, the namespace, its project and a list of the policies), whatever it referenced.
func TestGrantWebhooks_ReadsPerRequest(t *testing.T) {
	cases := []struct {
		name     string
		path     string
		review   func(t *testing.T) admissionv1.AdmissionReview
		allowed  bool
		patched  bool
		unsynced int // reads of the API server while the informers have not synced
	}{
		{
			name: "is-granted pod CREATE without a runtime class",
			path: "/is-granted",
			review: func(t *testing.T) admissionv1.AdmissionReview {
				return review(admissionv1.Create, podGVR, podGVK, "proj", "p", pod(t, "", nil), nil)
			},
			allowed: true, unsynced: 2,
		},
		{
			name: "is-granted pod CREATE with a granted runtime class",
			path: "/is-granted",
			review: func(t *testing.T) admissionv1.AdmissionReview {
				return review(admissionv1.Create, podGVR, podGVK, "proj", "p", pod(t, "gvisor", nil), nil)
			},
			allowed: true, unsynced: 5,
		},
		{
			name: "is-granted pod CREATE with a runtime class not granted",
			path: "/is-granted",
			review: func(t *testing.T) admissionv1.AdmissionReview {
				return review(admissionv1.Create, podGVR, podGVK, "proj", "p", pod(t, "kata", nil), nil)
			},
			allowed: false, unsynced: 5,
		},
		{
			name: "is-granted pod UPDATE",
			path: "/is-granted",
			review: func(t *testing.T) admissionv1.AdmissionReview {
				return review(admissionv1.Update, podGVR, podGVK, "proj", "p", pod(t, "kata", map[string]any{"a": "b"}), pod(t, "kata", nil))
			},
			allowed: true, unsynced: 2,
		},
		{
			name: "is-granted Deployment UPDATE keeping its runtime class",
			path: "/is-granted",
			review: func(t *testing.T) admissionv1.AdmissionReview {
				return review(admissionv1.Update, deployGVR, deployGVK, "proj", "d", deployment(t, "kata"), deployment(t, "kata"))
			},
			allowed: true, unsynced: 2,
		},
		{
			name: "defaults pod CREATE, no reference defaults",
			path: "/defaults",
			review: func(t *testing.T) admissionv1.AdmissionReview {
				return review(admissionv1.Create, podGVR, podGVK, "proj", "p", pod(t, "", nil), nil)
			},
			allowed: true, unsynced: 2,
		},
		{
			name: "defaults Deployment CREATE naming its runtime class (FillEmpty)",
			path: "/defaults",
			review: func(t *testing.T) admissionv1.AdmissionReview {
				return review(admissionv1.Create, deployGVR, deployGVK, "proj", "d", deployment(t, "gvisor"), nil)
			},
			allowed: true, unsynced: 2,
		},
		{
			name: "defaults Deployment CREATE without a runtime class",
			path: "/defaults",
			review: func(t *testing.T) admissionv1.AdmissionReview {
				return review(admissionv1.Create, deployGVR, deployGVK, "proj", "d", deployment(t, ""), nil)
			},
			allowed: true, patched: true, unsynced: 5,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, synced := range []bool{false, true} {
				f := newModelFixture(t, synced, rcModel()...)
				var resp *admissionv1.AdmissionResponse
				if tc.path == "/is-granted" {
					resp = serve(t, NewIsGrantedValidator(logr.Discard(), f.reader, testMapper(), jsonpath.NewWithCache()), tc.path, tc.review(t))
				} else {
					resp = serve(t, NewDefaultsMutator(logr.Discard(), f.reader, testMapper(), jsonpath.NewWithCache()), tc.path, tc.review(t))
				}
				if resp.Allowed != tc.allowed {
					t.Fatalf("synced=%v: allowed=%v, want %v: %v", synced, resp.Allowed, tc.allowed, resp.Result)
				}
				if (len(resp.Patch) > 0) != tc.patched {
					t.Fatalf("synced=%v: patch %s, want patched=%v", synced, resp.Patch, tc.patched)
				}
				want := tc.unsynced
				if synced {
					want = 0
				}
				if got := f.direct.total(); got != want {
					t.Fatalf("synced=%v: %d reads of the API server, want %d: %v", synced, got, want, f.direct)
				}
				t.Logf("synced=%v: API server %d %v, cache %d %v", synced, f.direct.total(), f.direct, f.cache.total(), f.cache)
			}
		})
	}
}

// A reference of pods on a mutable field (an annotation) is checked on UPDATE: a value the UPDATE adds is
// denied, an unchanged one is grandfathered.
func TestIsGranted_PodUpdateChecksMutableFields(t *testing.T) {
	ref := &v1alpha1.GrantableClusterResourceReference{
		ObjectMeta: metav1.ObjectMeta{Name: "runtimeclasses-annotation"},
		Spec: v1alpha1.GrantableClusterResourceReferenceSpec{
			GrantableClusterResourceName: "runtimeclasses",
			Rule:                         v1alpha1.UsageRule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"pods"}},
			FieldPaths:                   []v1alpha1.FieldPath{{Path: "$.metadata.annotations['example.com/runtime']"}},
		},
	}
	v := isGranted(t, rcDef(), ref, rcGrant(), projectNS("proj", nil))
	newPod := pod(t, "", map[string]any{"example.com/runtime": "kata"})
	if resp := serve(t, v, "/is-granted", review(admissionv1.Update, podGVR, podGVK, "proj", "p", newPod, pod(t, "", nil))); resp.Allowed {
		t.Fatal("a value added to a mutable pod field on UPDATE must be checked")
	}
	if resp := serve(t, v, "/is-granted", review(admissionv1.Update, podGVR, podGVK, "proj", "p", newPod, newPod)); !resp.Allowed {
		t.Fatalf("an unchanged value is grandfathered: %v", resp.Result)
	}
}

// NewModelReader registers the informers on a cache that has not started without waiting for them, and
// until they sync every read goes to the API server: a webhook request never waits for an informer.
func TestNewModelReader_DoesNotWaitForTheCache(t *testing.T) {
	directCl, directCalls := countingClient(t, projectNS("proj", nil))
	mapper := meta.NewDefaultRESTMapper(nil)
	for _, obj := range modelObjects() {
		gvk, err := apiutil.GVKForObject(obj, directCl.Scheme())
		if err != nil {
			t.Fatal(err)
		}
		mapper.Add(gvk, meta.RESTScopeRoot)
	}
	// Nothing listens there: the cache is never started, so nothing may connect.
	c, err := cache.New(&rest.Config{Host: "http://127.0.0.1:1"}, cache.Options{Scheme: directCl.Scheme(), Mapper: mapper})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r, err := NewModelReader(ctx, c, directCl, NewModelWatch())
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Get(ctx, client.ObjectKey{Name: "proj"}, &corev1.Namespace{}); err != nil {
		t.Fatal(err)
	}
	if err := r.List(ctx, &grantsv1alpha2.ClusterResourceGrantPolicyList{}); err != nil {
		t.Fatal(err)
	}
	if directCalls.total() != 2 {
		t.Fatalf("an unsynced model must be read from the API server: %v", directCalls)
	}
}

// One kind synced and another not: each is read from its own source, the lists by the kind of their
// items.
func TestModelReader_PartialSync(t *testing.T) {
	f := newModelFixture(t, true, rcModel()...)
	f.reader.synced[modelPolicy] = func() bool { return false }
	ctx := context.Background()
	for _, l := range []client.ObjectList{
		&v1alpha1.GrantableClusterResourceReferenceList{},
		&v1alpha1.GrantableClusterResourceDefinitionList{},
		&grantsv1alpha2.ClusterResourceGrantPolicyList{},
		&corev1.NamespaceList{},
		&v1alpha3.ProjectList{},
	} {
		if err := f.reader.List(ctx, l); err != nil {
			t.Fatal(err)
		}
	}
	if f.direct.total() != 1 || f.direct.n["list *v1alpha2.ClusterResourceGrantPolicyList"] != 1 {
		t.Fatalf("only the unsynced policies may be listed from the API server: %v", f.direct)
	}
	if f.cache.total() != 4 {
		t.Fatalf("the synced kinds must be listed from the cache: %v", f.cache)
	}
}

// reflectorOf returns a reflector of the informer type of obj, for the watch error handler.
func reflectorOf(obj runtime.Object) *toolscache.Reflector {
	return toolscache.NewReflector(&toolscache.ListWatch{}, obj, toolscache.NewStore(toolscache.MetaNamespaceKeyFunc), 0)
}

// A synced informer whose list or watch fails is not read: HasSynced stays true, the cache would answer
// with what it last saw. The kind is read from the API server until the hold after its last error has
// passed. An expired or closed watch, which the reflector resumes at once, holds nothing.
func TestModelReader_WatchErrorReadsTheAPIServer(t *testing.T) {
	f := newModelFixture(t, true, rcModel()...)
	now := time.Now()
	watch := NewModelWatch()
	watch.now = func() time.Time { return now }
	f.reader.watch = watch
	ctx := context.Background()
	list := func() {
		t.Helper()
		if err := f.reader.List(ctx, &grantsv1alpha2.ClusterResourceGrantPolicyList{}); err != nil {
			t.Fatal(err)
		}
	}

	watch.WatchError(ctx, reflectorOf(&grantsv1alpha2.ClusterResourceGrantPolicy{}), k8serrors.NewResourceExpired("too old"))
	watch.WatchError(ctx, reflectorOf(&grantsv1alpha2.ClusterResourceGrantPolicy{}), io.EOF)
	list()
	if f.direct.total() != 0 {
		t.Fatalf("an expired or closed watch must not send reads to the API server: %v", f.direct)
	}

	watch.WatchError(ctx, reflectorOf(&grantsv1alpha2.ClusterResourceGrantPolicy{}), k8serrors.NewForbidden(
		schema.GroupResource{Group: "multitenancy.deckhouse.io", Resource: "clusterresourcegrantpolicies"}, "", errors.New("watch revoked")))
	list()
	if f.direct.total() != 1 {
		t.Fatalf("a failing watch must send the reads of its kind to the API server: %v", f.direct)
	}
	// The other kinds keep reading the cache.
	if err := f.reader.List(ctx, &v1alpha1.GrantableClusterResourceReferenceList{}); err != nil {
		t.Fatal(err)
	}
	if f.direct.total() != 1 {
		t.Fatalf("a failing watch of one kind must not affect another: %v", f.direct)
	}

	now = now.Add(modelWatchErrorHold + time.Second)
	f.direct.reset()
	list()
	if f.direct.total() != 0 {
		t.Fatalf("after the hold the kind must be read from the cache again: %v", f.direct)
	}
}

// The webhook answers from the API server while the watch of a kind fails, and fails the request when
// the API server does not answer either, instead of deciding on the stale cache.
func TestIsGranted_WatchErrorFailsClosed(t *testing.T) {
	cacheCl, _ := countingClient(t, rcModel()...)
	directCl, _ := countingClient(t)
	broken := interceptor.NewClient(directCl.(client.WithWatch), interceptor.Funcs{
		List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
			return errors.New("API server unavailable")
		},
		Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
			return errors.New("API server unavailable")
		},
	})
	watch := NewModelWatch()
	s := map[modelKind]func() bool{}
	for kind := range modelObjects() {
		s[kind] = func() bool { return true }
	}
	r := &ModelReader{cache: cacheCl, direct: broken, synced: s, watch: watch}
	v := NewIsGrantedValidator(logr.Discard(), r, testMapper(), jsonpath.NewWithCache())
	req := review(admissionv1.Create, podGVR, podGVK, "proj", "p", pod(t, "kata", nil), nil).Request

	if resp, err := v.decide(context.Background(), req, logr.Discard()); err != nil || resp.Allowed {
		t.Fatalf("with a healthy cache the class not granted must be denied: %v %v", resp, err)
	}
	watch.WatchError(context.Background(), reflectorOf(&v1alpha1.GrantableClusterResourceReference{}), errors.New("connection refused"))
	if _, err := v.decide(context.Background(), req, logr.Discard()); err == nil {
		t.Fatal("with the watch of the references failing and no API server the request must fail")
	}
}
