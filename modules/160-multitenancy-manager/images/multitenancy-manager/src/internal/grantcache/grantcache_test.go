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

package grantcache

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"controller/api/v1alpha1"
)

var roleGVK = schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole"}

// stubCache serves reads from a fake client and hands out one stub informer per kind.
type stubCache struct {
	cache.Cache
	mu        sync.Mutex
	objects   client.Reader
	informers map[schema.GroupVersionKind]*stubInformer
	reads     int
	created   int
	removed   int
}

func (s *stubCache) GetInformer(_ context.Context, obj client.Object, _ ...cache.InformerGetOption) (cache.Informer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	gvk := obj.GetObjectKind().GroupVersionKind()
	if s.informers[gvk] == nil {
		s.informers[gvk] = &stubInformer{}
		s.created++
	}
	return s.informers[gvk], nil
}

func (s *stubCache) RemoveInformer(_ context.Context, obj client.Object) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	gvk := obj.GetObjectKind().GroupVersionKind()
	if inf := s.informers[gvk]; inf != nil {
		inf.setStopped()
		delete(s.informers, gvk)
		s.removed++
	}
	return nil
}

func (s *stubCache) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	s.reads++
	return s.objects.Get(ctx, key, obj, opts...)
}

func (s *stubCache) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	s.reads++
	return s.objects.List(ctx, list, opts...)
}

type stubInformer struct {
	cache.Informer
	mu       sync.Mutex
	synced   bool
	stopped  bool
	handlers []toolscache.ResourceEventHandler
}

func (i *stubInformer) HasSynced() bool { i.mu.Lock(); defer i.mu.Unlock(); return i.synced }
func (i *stubInformer) IsStopped() bool { i.mu.Lock(); defer i.mu.Unlock(); return i.stopped }
func (i *stubInformer) setStopped()     { i.mu.Lock(); defer i.mu.Unlock(); i.stopped = true }

func (i *stubInformer) AddEventHandler(h toolscache.ResourceEventHandler) (toolscache.ResourceEventHandlerRegistration, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.handlers = append(i.handlers, h)
	return nil, nil
}

// countingReader counts the reads that reach the API server.
type countingReader struct {
	client.Reader
	reads int
}

func (c *countingReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	c.reads++
	return c.Reader.Get(ctx, key, obj, opts...)
}

func (c *countingReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	c.reads++
	return c.Reader.List(ctx, list, opts...)
}

func newFake(t *testing.T, objs ...client.Object) client.Reader {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := rbacv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func role(name string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func TestReaderFallsBackUntilSynced(t *testing.T) {
	ctx := context.Background()
	// The cache knows "old"; the API server also has "fresh", created after the cache saw the kind.
	stub := &stubCache{objects: newFake(t, role("old")), informers: map[schema.GroupVersionKind]*stubInformer{}}
	api := &countingReader{Reader: newFake(t, role("old"), role("fresh"))}
	r := New(stub, api, nil).Reader(nil)

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(roleGVK.GroupVersion().WithKind("ClusterRoleList"))
	if err := r.List(ctx, list); err != nil {
		t.Fatal(err)
	}
	if api.reads != 1 || stub.reads != 0 || len(list.Items) != 2 {
		t.Fatalf("before the sync: api reads %d, cache reads %d, items %d; want the API server only", api.reads, stub.reads, len(list.Items))
	}
	if len(stub.informers[roleGVK].handlers) != 1 {
		t.Fatalf("the informer of the kind was not set up with a change handler")
	}

	stub.informers[roleGVK].mu.Lock()
	stub.informers[roleGVK].synced = true
	stub.informers[roleGVK].mu.Unlock()
	api.reads = 0
	if err := r.List(ctx, list); err != nil {
		t.Fatal(err)
	}
	if api.reads != 0 || stub.reads != 1 || len(list.Items) != 1 {
		t.Fatalf("after the sync: api reads %d, cache reads %d, items %d; want the cache only", api.reads, stub.reads, len(list.Items))
	}

	// A name the cache does not have yet is looked up in the API server.
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(roleGVK)
	if err := r.Get(ctx, client.ObjectKey{Name: "fresh"}, obj); err != nil {
		t.Fatalf("get fresh: %v", err)
	}
	if api.reads != 1 {
		t.Fatalf("api reads %d, want 1 for the object the cache misses", api.reads)
	}
	if err := r.Get(ctx, client.ObjectKey{Name: "old"}, obj); err != nil || api.reads != 1 {
		t.Fatalf("get old: err %v, api reads %d; want it from the cache", err, api.reads)
	}
	if err := r.Get(ctx, client.ObjectKey{Name: "missing"}, obj); !k8serrors.IsNotFound(err) {
		t.Fatalf("get missing: %v, want NotFound", err)
	}
}

func TestReaderPassesTypedReadsToBase(t *testing.T) {
	stub := &stubCache{objects: newFake(t), informers: map[schema.GroupVersionKind]*stubInformer{}}
	base := &countingReader{Reader: newFake(t, role("typed"))}
	r := New(stub, newFake(t), nil).Reader(base)
	if err := r.Get(context.Background(), client.ObjectKey{Name: "typed"}, &rbacv1.ClusterRole{}); err != nil {
		t.Fatal(err)
	}
	if base.reads != 1 || len(stub.informers) != 0 {
		t.Fatalf("a typed read must go to the base reader without an informer")
	}
}

func TestChangeHandlerSkipsInitialListAndResyncs(t *testing.T) {
	n := 0
	h := changeHandler{notify: func() { n++ }}
	a := &unstructured.Unstructured{}
	a.SetResourceVersion("1")
	b := &unstructured.Unstructured{}
	b.SetResourceVersion("2")

	h.OnAdd(a, true)
	h.OnUpdate(a, a)
	if n != 0 {
		t.Fatalf("initial list and resync notified %d times", n)
	}
	h.OnAdd(a, false)
	h.OnUpdate(a, b)
	h.OnDelete(b)
	if n != 3 {
		t.Fatalf("notified %d times, want 3", n)
	}
}

func TestTriggerFoldsABurst(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tr := &Trigger{Delay: 50 * time.Millisecond}
	go func() { _ = tr.Start(ctx) }()

	for range 100 {
		tr.Kick()
	}
	select {
	case <-tr.Events():
	case <-time.After(time.Second):
		t.Fatal("no event after a burst")
	}
	select {
	case <-tr.Events():
		t.Fatal("a burst produced more than one event")
	case <-time.After(150 * time.Millisecond):
	}

	tr.Kick()
	select {
	case <-tr.Events():
	case <-time.After(time.Second):
		t.Fatal("no event for a kick after the burst")
	}
}

func TestTriggerNeverBlocksWithoutAReader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tr := &Trigger{Delay: time.Millisecond}
	go func() { _ = tr.Start(ctx) }()
	done := make(chan struct{})
	go func() {
		for range 1000 {
			tr.Kick()
			time.Sleep(10 * time.Microsecond)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Kick blocked while nobody reads the events")
	}
}

func TestTriggerKeepsMinInterval(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tr := &Trigger{Delay: 10 * time.Millisecond, MinInterval: 300 * time.Millisecond}
	go func() { _ = tr.Start(ctx) }()

	tr.Kick()
	<-tr.Events()
	first := time.Now()
	tr.Kick()
	select {
	case <-tr.Events():
		if gap := time.Since(first); gap < 250*time.Millisecond {
			t.Fatalf("second event %v after the first, want at least MinInterval", gap)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no second event")
	}
}

func TestNotCached(t *testing.T) {
	if !notCached(&cache.ErrResourceNotCached{}) || !notCached(&cache.ErrCacheNotStarted{}) {
		t.Fatal("a missing informer or a cache not started must be read from the API server")
	}
	if notCached(k8serrors.NewNotFound(schema.GroupResource{}, "x")) || notCached(nil) {
		t.Fatal("other errors are not a missing informer")
	}
}

// definitions is a fake list of GrantableClusterResourceDefinitions granting the given kinds.
func definitions(t *testing.T, kinds ...schema.GroupKind) client.Reader {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	b := fake.NewClientBuilder().WithScheme(scheme)
	for _, gk := range kinds {
		b = b.WithObjects(&v1alpha1.GrantableClusterResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: gk.Kind},
			Spec: v1alpha1.GrantableClusterResourceDefinitionSpec{
				GrantedResource: &v1alpha1.GrantedResource{APIGroup: gk.Group, Kind: gk.Kind},
			},
		})
	}
	return b.Build()
}

func roleList() *unstructured.UnstructuredList {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(roleGVK.GroupVersion().WithKind("ClusterRoleList"))
	return list
}

func TestReaderLeavesKindsNotGrantedToTheAPIServer(t *testing.T) {
	stub := &stubCache{objects: newFake(t), informers: map[schema.GroupVersionKind]*stubInformer{}}
	api := &countingReader{Reader: newFake(t, role("a"))}
	c := New(stub, api, definitions(t, schema.GroupKind{Group: "storage.k8s.io", Kind: "StorageClass"}))
	primeKinds(t, c)
	r := c.Reader(nil)
	if err := r.List(context.Background(), roleList()); err != nil {
		t.Fatal(err)
	}
	if api.reads != 1 || stub.created != 0 {
		t.Fatalf("a kind no definition grants: api reads %d, informers created %d; want the API server, no informer", api.reads, stub.created)
	}
}

func TestReaderReplacesAStoppedInformer(t *testing.T) {
	ctx := context.Background()
	stub := &stubCache{objects: newFake(t), informers: map[schema.GroupVersionKind]*stubInformer{}}
	c := New(stub, newFake(t), nil)
	first, err := c.informer(ctx, roleGVK)
	if err != nil {
		t.Fatal(err)
	}
	// Removed from the cache behind the back of the entry, as a sweep racing a read could do.
	_ = stub.RemoveInformer(ctx, func() client.Object {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(roleGVK)
		return u
	}())
	second, err := c.informer(ctx, roleGVK)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || second.IsStopped() {
		t.Fatal("a stopped informer must be replaced by a new one")
	}
}

// failingMapper answers every RESTMapping with the given error.
type failingMapper struct {
	meta.RESTMapper
	err error
}

func (m failingMapper) RESTMapping(schema.GroupKind, ...string) (*meta.RESTMapping, error) {
	return nil, m.err
}

func TestSweepKeepsInformersOnATransientMapperError(t *testing.T) {
	ctx := context.Background()
	stub := &stubCache{objects: newFake(t), informers: map[schema.GroupVersionKind]*stubInformer{}}
	c := New(stub, newFake(t), definitions(t, roleGVK.GroupKind()))
	if _, err := c.informer(ctx, roleGVK); err != nil {
		t.Fatal(err)
	}
	s := &Sweeper{Cache: c, Mapper: failingMapper{err: errors.New("discovery failed")}}
	if err := s.Sweep(ctx); err == nil {
		t.Fatal("a transient mapper error must fail the sweep")
	}
	if stub.removed != 0 {
		t.Fatal("a transient mapper error must not remove informers")
	}

	s.Mapper = failingMapper{err: &meta.NoKindMatchError{GroupKind: roleGVK.GroupKind()}}
	if err := s.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	if stub.removed != 1 {
		t.Fatalf("an unmapped kind must be removed, removed %d", stub.removed)
	}
}

func TestSweepRacesReads(t *testing.T) {
	ctx := context.Background()
	stub := &stubCache{objects: newFake(t), informers: map[schema.GroupVersionKind]*stubInformer{}}
	c := New(stub, newFake(t), definitions(t, roleGVK.GroupKind()))
	primeKinds(t, c)
	r := c.Reader(nil)
	s := &Sweeper{Cache: c, Mapper: failingMapper{err: &meta.NoKindMatchError{GroupKind: roleGVK.GroupKind()}}}
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 200 {
				_ = r.List(ctx, roleList())
			}
		}()
	}
	for range 50 {
		_ = s.Sweep(ctx)
	}
	wg.Wait()
	// Whatever the interleaving, the entry the reads hold is never a stopped informer.
	inf, err := c.informer(ctx, roleGVK)
	if err == nil && inf.IsStopped() {
		t.Fatal("a stopped informer is left in use")
	}
}

func TestWatchNotFoundDropsTheKind(t *testing.T) {
	ctx := context.Background()
	stub := &stubCache{objects: newFake(t), informers: map[schema.GroupVersionKind]*stubInformer{}}
	api := &countingReader{Reader: newFake(t)}
	c := New(stub, api, nil)
	notified := make(chan struct{}, 1)
	c.OnNotServed(func() { notified <- struct{}{} })
	inf, err := c.informer(ctx, roleGVK)
	if err != nil {
		t.Fatal(err)
	}
	expected := &unstructured.Unstructured{}
	expected.SetGroupVersionKind(roleGVK)
	reflector := toolscache.NewReflector(&toolscache.ListWatch{}, expected, toolscache.NewStore(toolscache.MetaNamespaceKeyFunc), 0)

	c.watchError(ctx, reflector, k8serrors.NewNotFound(schema.GroupResource{}, ""))
	select {
	case <-notified:
	case <-time.After(time.Second):
		t.Fatal("a 404 of the watch did not report the kind as not served")
	}
	if !inf.IsStopped() {
		t.Fatal("the informer of a kind no longer served must be removed")
	}
	// Until NotServedDelay, the kind is read from the API server and no informer is made.
	created := stub.created
	if err := c.Reader(nil).List(ctx, roleList()); err != nil {
		t.Fatal(err)
	}
	if api.reads != 1 || stub.created != created {
		t.Fatalf("api reads %d, informers created %d; want the API server and no new informer", api.reads, stub.created-created)
	}
	if _, err := c.informer(ctx, roleGVK); !errors.Is(err, errBackoff) {
		t.Fatalf("informer: %v, want the not-served delay", err)
	}
}

// primeKinds loads the set of granted kinds, as the background refresh would.
func primeKinds(t *testing.T, c *Cache) {
	t.Helper()
	kinds, err := c.listKinds(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	c.kinds.Store(&kindSet{kinds: kinds, at: time.Now()})
}

// blockingReader blocks every List until release is closed, or fails it when fail is set.
type blockingReader struct {
	client.Reader
	release chan struct{}
	fail    bool
}

func (b *blockingReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if b.fail {
		return errors.New("api server unavailable")
	}
	<-b.release
	return b.Reader.List(ctx, list, opts...)
}

func TestGrantedKindNeverWaitsForTheDefinitions(t *testing.T) {
	defs := &blockingReader{Reader: definitions(t, roleGVK.GroupKind()), release: make(chan struct{})}
	c := New(&stubCache{objects: newFake(t), informers: map[schema.GroupVersionKind]*stubInformer{}}, newFake(t), defs)

	done := make(chan bool, 1)
	go func() { done <- c.grantedKind(context.Background(), roleGVK.GroupKind()) }()
	select {
	case granted := <-done:
		if granted {
			t.Fatal("no kind counts as granted before the first list")
		}
	case <-time.After(time.Second):
		t.Fatal("a read waited for the list of the definitions")
	}

	close(defs.release)
	deadline := time.Now().Add(2 * time.Second)
	for !c.grantedKind(context.Background(), roleGVK.GroupKind()) {
		if time.Now().After(deadline) {
			t.Fatal("the granted kinds were not loaded in the background")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestGrantedKindKeepsTheLastSetWhenTheListFails(t *testing.T) {
	defs := &blockingReader{Reader: definitions(t, roleGVK.GroupKind()), release: make(chan struct{})}
	close(defs.release)
	c := New(&stubCache{objects: newFake(t), informers: map[schema.GroupVersionKind]*stubInformer{}}, newFake(t), defs)
	primeKinds(t, c)
	// The set is stale and the API server fails: the read still answers from the last set.
	c.kinds.Store(&kindSet{kinds: c.kinds.Load().kinds, at: time.Now().Add(-time.Hour)})
	defs.fail = true
	if !c.grantedKind(context.Background(), roleGVK.GroupKind()) {
		t.Fatal("a stale set must still answer")
	}
	deadline := time.Now().Add(2 * time.Second)
	for c.refreshing.Load() || time.Since(c.kinds.Load().at) > kindsTTL {
		if time.Now().After(deadline) {
			t.Fatal("the failed refresh did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !c.grantedKind(context.Background(), roleGVK.GroupKind()) {
		t.Fatal("a failed list must keep the last set")
	}
}

func TestGrantedKindSkipsDefinitionsBeingDeletedOrValueBacked(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	now := metav1.Now()
	defs := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&v1alpha1.GrantableClusterResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: "deleting", DeletionTimestamp: &now, Finalizers: []string{"test"}},
			Spec:       v1alpha1.GrantableClusterResourceDefinitionSpec{GrantedResource: &v1alpha1.GrantedResource{APIGroup: roleGVK.Group, Kind: roleGVK.Kind}},
		},
		&v1alpha1.GrantableClusterResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "values"}},
	).Build()
	c := New(&stubCache{objects: newFake(t), informers: map[schema.GroupVersionKind]*stubInformer{}}, newFake(t), defs)
	primeKinds(t, c)
	if c.grantedKind(context.Background(), roleGVK.GroupKind()) {
		t.Fatal("a definition being deleted grants no kind")
	}
}
