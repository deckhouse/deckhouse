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

// Package grantcache serves the granted objects of the grant model (the objects of the kinds the
// GrantableClusterResourceDefinitions name) from informers of a cache of their own, and tells the
// catalog reconciler when any of them changes.
//
// Without it, every catalog reconcile of every project namespace and every admission request listed
// the whole granted kind from the API server: with thousands of granted objects and hundreds of
// projects a pass over the catalogs outgrew the resync period and inflated the memory of the API
// server.
//
// The cache is kept apart from the manager's: only the granted kinds get an informer here, while the
// manager's client keeps reading other unstructured objects straight from the API server. An informer
// is started the first time a granted kind is read and removed when no definition names the kind any
// more or the kind is no longer served. Until an informer has synced, and while another read of the
// same kind is setting its informer up, reads go to the API server, so a read never waits for an
// informer.
package grantcache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"controller/api/v1alpha1"
)

const (
	// retryDelay is how long a kind whose informer could not be set up is read from the API server
	// before the next attempt.
	retryDelay = 10 * time.Second
	// NotServedDelay is how long a kind the API server answered 404 for is read from the API server
	// (which then reports it as not served) before an informer is tried again.
	NotServedDelay = 2 * time.Minute
	// kindsTTL is how long the set of granted kinds is reused before the definitions are listed again.
	kindsTTL = 10 * time.Second
	// kindsTimeout bounds one list of the definitions.
	kindsTimeout = 10 * time.Second
)

// Cache holds the informers of the granted kinds.
type Cache struct {
	cache       cache.Cache
	direct      client.Reader
	definitions client.Reader

	mu          sync.Mutex
	entries     map[schema.GroupVersionKind]*entry
	onChange    func()
	onNotServed func()

	// kinds is the set of granted kinds, refreshed in the background (refreshing) so a read never
	// waits for the list of the definitions.
	kinds      atomic.Pointer[kindSet]
	refreshing atomic.Bool
}

// kindSet is the set of the kinds the definitions grant, as listed at a time.
type kindSet struct {
	kinds map[schema.GroupKind]struct{}
	at    time.Time
}

// entry is the informer of one kind. Its own lock keeps the setup of one kind from holding up the
// reads of the others; a read that finds it locked goes to the API server instead of waiting.
type entry struct {
	mu      sync.Mutex
	inf     cache.Informer
	removed bool
	retryAt time.Time
}

// New wraps a controller-runtime cache made for the granted kinds; Build makes one. direct reads a
// kind whose informer has not synced yet. definitions lists the GrantableClusterResourceDefinitions,
// which say which kinds are granted; when nil, every unstructured kind counts as granted.
func New(c cache.Cache, direct, definitions client.Reader) *Cache {
	return &Cache{cache: c, direct: direct, definitions: definitions, entries: map[schema.GroupVersionKind]*entry{}}
}

// Build makes the controller-runtime cache from opts and wraps it. The cache is made with
// ReaderFailOnMissingInformer, so that a read never creates an informer and waits for its sync, and
// with a watch error handler that notices a kind the API server stopped serving.
func Build(cfg *rest.Config, opts cache.Options, direct, definitions client.Reader) (*Cache, error) {
	c := New(nil, direct, definitions)
	opts.ReaderFailOnMissingInformer = true
	opts.DefaultWatchErrorHandler = c.watchError
	rc, err := cache.New(cfg, opts)
	if err != nil {
		return nil, err
	}
	c.cache = rc
	return c, nil
}

// OnChange sets the function called on every change of a granted object after the initial list of
// its informer. It must not block.
func (c *Cache) OnChange(fn func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onChange = fn
}

// OnNotServed sets the function called when the API server answers 404 for a granted kind, its CRD
// being deleted. It must not block.
func (c *Cache) OnNotServed(fn func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onNotServed = fn
}

func (c *Cache) changed() {
	c.mu.Lock()
	fn := c.onChange
	c.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// Start runs the informers. It implements manager.Runnable.
func (c *Cache) Start(ctx context.Context) error { return c.cache.Start(ctx) }

// GetCache puts the cache into the manager's group of caches, which runs on every replica, not only
// on the leader: the webhooks of every replica read through it.
func (c *Cache) GetCache() cache.Cache { return c.cache }

// errBusy and errBackoff tell a read to go to the API server: another read is setting the informer
// up, or the kind is in its retry delay.
var (
	errBusy    = errors.New("informer is being set up")
	errBackoff = errors.New("informer setup is delayed")
)

// informer returns the informer of the kind, creating and starting it without waiting for its sync.
// It never waits for another read of the same kind (errBusy) and does not retry a failed setup
// before retryDelay (errBackoff).
func (c *Cache) informer(ctx context.Context, gvk schema.GroupVersionKind) (cache.Informer, error) {
	c.mu.Lock()
	e := c.entries[gvk]
	if e == nil {
		e = &entry{}
		c.entries[gvk] = e
	}
	c.mu.Unlock()

	if !e.mu.TryLock() {
		return nil, errBusy
	}
	defer e.mu.Unlock()
	if e.removed {
		return nil, errBackoff
	}
	// An informer stopped behind our back (removed from the cache while a read was setting it up) is
	// replaced; GetInformer below then makes a new one.
	if e.inf != nil && e.inf.IsStopped() {
		e.inf = nil
	}
	if e.inf != nil {
		return e.inf, nil
	}
	if time.Now().Before(e.retryAt) {
		return nil, errBackoff
	}
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	inf, err := c.cache.GetInformer(ctx, obj, cache.BlockUntilSynced(false))
	if err == nil {
		_, err = inf.AddEventHandler(changeHandler{notify: c.changed})
	}
	if err != nil {
		e.retryAt = time.Now().Add(retryDelay)
		return nil, err
	}
	e.inf = inf
	return inf, nil
}

// drop removes the informer of the kind. Reads of the kind go to the API server until retryAt.
func (c *Cache) drop(ctx context.Context, gvk schema.GroupVersionKind, retryAt time.Time) error {
	c.mu.Lock()
	e := c.entries[gvk]
	if retryAt.IsZero() {
		delete(c.entries, gvk)
	} else {
		c.entries[gvk] = &entry{retryAt: retryAt}
	}
	c.mu.Unlock()
	if e == nil {
		return nil
	}
	// Waits for a read that is setting the informer up, so the informer it makes is removed too.
	e.mu.Lock()
	e.removed = true
	inf := e.inf
	e.mu.Unlock()
	if inf == nil {
		return nil
	}
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	return c.cache.RemoveInformer(ctx, obj)
}

// watchError is the watch error handler of the informers. A 404 from the list or the watch means the
// kind is no longer served (its CRD was deleted): the informer is dropped, so the reads of the kind go
// to the API server and report the kind as not served, as they did before the cache, instead of
// answering from what the store still holds.
func (c *Cache) watchError(ctx context.Context, r *toolscache.Reflector, err error) {
	toolscache.DefaultWatchErrorHandler(ctx, r, err)
	if !k8serrors.IsNotFound(err) {
		return
	}
	c.mu.Lock()
	var gvk schema.GroupVersionKind
	found := false
	for k := range c.entries {
		if k.String() == r.TypeDescription() {
			gvk, found = k, true
			break
		}
	}
	notServed := c.onNotServed
	c.mu.Unlock()
	if !found {
		return
	}
	// Removing the informer stops this very reflector, so it is done apart from it.
	go func() {
		if err := c.drop(context.WithoutCancel(ctx), gvk, time.Now().Add(NotServedDelay)); err != nil {
			ctrllog.FromContext(ctx).Error(err, "drop informer of a kind no longer served", "kind", gvk.String())
		}
		ctrllog.FromContext(ctx).Info("granted kind is no longer served, its informer is dropped", "kind", gvk.String())
		if notServed != nil {
			notServed()
		}
	}()
}

// grantedKind reports whether a definition grants the kind. It answers from the last listed set and
// never waits for the list: a set older than kindsTTL is refreshed in the background, one refresh at a
// time, with a context of its own, so a slow or failing API server never holds up a read. Until the
// first list has come back no kind counts as granted, which sends the reads to the API server.
func (c *Cache) grantedKind(_ context.Context, gk schema.GroupKind) bool {
	if c.definitions == nil {
		return true
	}
	set := c.kinds.Load()
	if set == nil || time.Since(set.at) > kindsTTL {
		c.refreshKinds()
	}
	if set == nil {
		return false
	}
	_, ok := set.kinds[gk]
	return ok
}

// refreshKinds lists the definitions in the background unless a list is already running. A failed
// list keeps the last set, and is not retried before kindsTTL.
func (c *Cache) refreshKinds() {
	if !c.refreshing.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer c.refreshing.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), kindsTimeout)
		defer cancel()
		kinds, err := c.listKinds(ctx)
		if err != nil {
			ctrllog.FromContext(ctx).V(1).Info("list granted kinds failed, keeping the last set", "reason", err.Error())
			if last := c.kinds.Load(); last != nil {
				kinds = last.kinds
			}
		}
		c.kinds.Store(&kindSet{kinds: kinds, at: time.Now()})
	}()
}

// listKinds lists the kinds the definitions grant, as Sweep counts them: a definition being deleted or
// value-backed grants no objects.
func (c *Cache) listKinds(ctx context.Context) (map[schema.GroupKind]struct{}, error) {
	defs := &v1alpha1.GrantableClusterResourceDefinitionList{}
	if err := c.definitions.List(ctx, defs); err != nil {
		return map[schema.GroupKind]struct{}{}, err
	}
	kinds := map[schema.GroupKind]struct{}{}
	for i := range defs.Items {
		def := &defs.Items[i]
		if def.IsValueBacked() || def.DeletionTimestamp != nil {
			continue
		}
		gr := def.Spec.GrantedResource
		kinds[schema.GroupKind{Group: gr.APIGroup, Kind: gr.Kind}] = struct{}{}
	}
	return kinds, nil
}

// cached reports whether the kind is read from the cache: it is a granted kind and its informer has
// synced. Anything else is read from the API server, as it was before the cache.
func (c *Cache) cached(ctx context.Context, gvk schema.GroupVersionKind) bool {
	if !c.grantedKind(ctx, gvk.GroupKind()) {
		return false
	}
	inf, err := c.informer(ctx, gvk)
	if err != nil {
		if !errors.Is(err, errBusy) && !errors.Is(err, errBackoff) {
			ctrllog.FromContext(ctx).V(1).Info("granted objects are read from the API server: informer unavailable", "kind", gvk.String(), "reason", err.Error())
		}
		return false
	}
	return inf.HasSynced()
}

// Reader returns a client.Reader that serves the objects of the granted kinds, read as unstructured,
// from the cache. Typed reads go to base; unstructured reads of a kind no definition grants go to the
// API server, so no informer is ever started for a kind that is not granted.
//
// The cache follows the API server with the lag of the watch. A Get the cache answers with NotFound is
// asked again from the API server, so an object created a moment ago is found; an object changed a
// moment ago may still be seen as it was.
func (c *Cache) Reader(base client.Reader) client.Reader { return &reader{c: c, base: base} }

type reader struct {
	c    *Cache
	base client.Reader
}

func (r *reader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return r.base.Get(ctx, key, obj, opts...)
	}
	if !r.c.cached(ctx, u.GroupVersionKind()) {
		return r.c.direct.Get(ctx, key, obj, opts...)
	}
	err := r.c.cache.Get(ctx, key, obj, opts...)
	if k8serrors.IsNotFound(err) || notCached(err) {
		return r.c.direct.Get(ctx, key, obj, opts...)
	}
	return err
}

func (r *reader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	u, ok := list.(*unstructured.UnstructuredList)
	if !ok {
		return r.base.List(ctx, list, opts...)
	}
	gvk := u.GroupVersionKind()
	gvk.Kind = strings.TrimSuffix(gvk.Kind, "List")
	if !r.c.cached(ctx, gvk) {
		return r.c.direct.List(ctx, list, opts...)
	}
	err := r.c.cache.List(ctx, list, opts...)
	if notCached(err) {
		return r.c.direct.List(ctx, list, opts...)
	}
	return err
}

// notCached reports the error of a cache read that found no informer (it was removed after the sync
// check; the cache is made with ReaderFailOnMissingInformer, so the read neither recreates the
// informer nor waits for its sync) or a cache that has not started yet.
func notCached(err error) bool {
	var missing *cache.ErrResourceNotCached
	var notStarted *cache.ErrCacheNotStarted
	return errors.As(err, &missing) || errors.As(err, &notStarted)
}

var _ toolscache.ResourceEventHandler = changeHandler{}

// changeHandler calls notify on every add, update and delete after the initial list.
type changeHandler struct{ notify func() }

func (h changeHandler) OnAdd(_ any, isInInitialList bool) {
	if !isInInitialList {
		h.notify()
	}
}

func (h changeHandler) OnUpdate(oldObj, newObj any) {
	// A resync of the informer delivers the object unchanged.
	if o, ok := oldObj.(*unstructured.Unstructured); ok {
		if n, ok := newObj.(*unstructured.Unstructured); ok && o.GetResourceVersion() == n.GetResourceVersion() {
			return
		}
	}
	h.notify()
}

func (h changeHandler) OnDelete(any) { h.notify() }

// Sweeper removes the informers of the kinds no definition grants any more, or that are no longer
// mapped (their CRD is gone or re-created with another scope or version), so a stale informer neither
// holds memory nor answers with objects that are gone. It runs on every replica.
type Sweeper struct {
	Cache *Cache
	// Mapper is the grants REST mapper, which forgets its mappings periodically.
	Mapper   meta.RESTMapper
	Interval time.Duration
}

// Start sweeps every Interval until ctx is done.
func (s *Sweeper) Start(ctx context.Context) error {
	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.Sweep(ctx); err != nil {
				ctrllog.FromContext(ctx).Error(err, "sweep granted informers")
			}
		}
	}
}

// NeedLeaderElection is false: the webhooks of every replica read through the cache.
func (s *Sweeper) NeedLeaderElection() bool { return false }

// Sweep removes the informers no definition needs. A kind is unwanted only when no definition names
// it, or the mapper says for sure that the kind is not served (no match) or is namespaced. Any other
// error of the mapper (a failed discovery) stops the sweep with nothing removed: dropping informers on
// a transient error would make every replica list every granted kind again.
func (s *Sweeper) Sweep(ctx context.Context) error {
	c := s.Cache
	defs := &v1alpha1.GrantableClusterResourceDefinitionList{}
	if err := c.definitions.List(ctx, defs); err != nil {
		return fmt.Errorf("list GrantableClusterResourceDefinitions: %w", err)
	}
	wanted := map[schema.GroupVersionKind]struct{}{}
	for i := range defs.Items {
		def := &defs.Items[i]
		if def.IsValueBacked() || def.DeletionTimestamp != nil {
			continue
		}
		gr := def.Spec.GrantedResource
		mapping, err := s.Mapper.RESTMapping(schema.GroupKind{Group: gr.APIGroup, Kind: gr.Kind})
		if meta.IsNoMatchError(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("map %s/%s: %w", gr.APIGroup, gr.Kind, err)
		}
		if mapping.Scope.Name() != meta.RESTScopeNameRoot {
			continue
		}
		wanted[mapping.GroupVersionKind] = struct{}{}
	}

	c.mu.Lock()
	var stale []schema.GroupVersionKind
	for gvk, e := range c.entries {
		if _, ok := wanted[gvk]; ok {
			continue
		}
		// An entry a read is setting up is left to the next sweep, and one in its retry delay (a kind
		// just found not served) is left to expire.
		if !e.mu.TryLock() {
			continue
		}
		delayed := e.retryAt.After(time.Now())
		e.mu.Unlock()
		if !delayed {
			stale = append(stale, gvk)
		}
	}
	c.mu.Unlock()
	for _, gvk := range stale {
		if err := c.drop(ctx, gvk, time.Time{}); err != nil {
			return fmt.Errorf("remove informer of %s: %w", gvk, err)
		}
		ctrllog.FromContext(ctx).V(1).Info("removed granted informer", "kind", gvk.String())
	}
	return nil
}

// Trigger turns the changes of granted objects into at most one event per Delay, which the catalog
// reconciler maps to every project namespace. A burst of changes (a module syncing hundreds of
// objects) thus sends the namespaces through the reconciler once, not once per object.
type Trigger struct {
	// Delay is how long a burst is collected after its first change.
	Delay time.Duration
	// MinInterval is the least time between two events, so that a steady trickle of changes does not
	// rewrite the catalogs of all project namespaces every Delay.
	MinInterval time.Duration

	once   sync.Once
	kick   chan struct{}
	events chan event.GenericEvent
}

func (t *Trigger) init() {
	t.once.Do(func() {
		t.kick = make(chan struct{}, 1)
		t.events = make(chan event.GenericEvent, 1)
	})
}

// Kick records a change. It never blocks.
func (t *Trigger) Kick() {
	t.init()
	select {
	case t.kick <- struct{}{}:
	default:
	}
}

// Events delivers one event per debounced burst of changes; its object carries nothing, the receiver
// reconciles every project namespace. Delivery is dropped while the previous event is still unread
// (on a replica that is not the leader nobody reads).
func (t *Trigger) Events() <-chan event.GenericEvent {
	t.init()
	return t.events
}

// Start debounces the kicks until ctx is done. It runs on every replica, as the informers do.
func (t *Trigger) Start(ctx context.Context) error {
	t.init()
	var last time.Time
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.kick:
		}
		wait := t.Delay
		if next := time.Until(last.Add(t.MinInterval)); next > wait {
			wait = next
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(wait):
		}
		last = time.Now()
		// The kicks of the burst are folded into this event; a kick from now on starts the next one.
		select {
		case <-t.kick:
		default:
		}
		select {
		case t.events <- event.GenericEvent{Object: &v1alpha1.GrantableClusterResourceDefinition{}}:
		default:
		}
	}
}

// NeedLeaderElection is false: it only folds the kicks of the informers, which run on every replica.
func (t *Trigger) NeedLeaderElection() bool { return false }
