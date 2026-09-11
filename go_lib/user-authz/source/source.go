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

// Package source keeps a rules.Directory in step with the ClusterAuthorizationRules of the cluster.
// It is an informer with a debounced rebuild: every consumer of the rules learns about a change
// within the watch latency plus the debounce window, instead of after a module release and a
// ConfigMap propagation.
package source

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"

	"github.com/deckhouse/deckhouse/go_lib/user-authz/rules"
)

// State is what the source knows about its own freshness.
type State int

const (
	// StateUnsynced: the initial list has not completed; the directory is nil.
	StateUnsynced State = iota
	// StateSynced: the directory reflects the rules as of the last event.
	StateSynced
	// StateCRDMissing: the ClusterAuthorizationRule CRD is not served. There can be no rules, so an
	// empty directory is the truth, not a failure; the informer keeps retrying and will sync once
	// the CRD appears.
	StateCRDMissing
)

// String implements fmt.Stringer.
func (s State) String() string {
	switch s {
	case StateUnsynced:
		return "unsynced"
	case StateSynced:
		return "synced"
	case StateCRDMissing:
		return "crd-missing"
	}
	return fmt.Sprintf("state(%d)", int(s))
}

// Observer receives the facts a consumer exports as metrics. Every method may be called from the
// rebuild goroutine; implementations must be safe for that.
type Observer interface {
	DirectoryRebuilt(stats rules.Stats, took time.Duration)
	SyncedChanged(synced bool)
	WatchError(err error)
}

// Options configure a Source.
type Options struct {
	// Debounce is how long the source waits after an event for more of them before rebuilding.
	// Zero means DefaultDebounce.
	Debounce time.Duration
	// Resync is the informer resync period; zero disables periodic resync.
	Resync time.Duration
	// OnUpdate is called with every new directory, after it is published.
	OnUpdate func(dir *rules.Directory, stats rules.Stats)
	// Observer receives the metrics facts; nil is fine.
	Observer Observer
	// Logf receives diagnostics; nil discards them.
	Logf func(format string, args ...interface{})
}

// DefaultDebounce is the rebuild coalescing window: long enough to fold the initial list of thousands
// of rules and the bursts a batch of changes produces into one build, short enough to stay far below
// the API server's 30 s cache of authorizer answers.
const DefaultDebounce = 200 * time.Millisecond

// Source is the informer-backed provider of a rules.Directory.
type Source struct {
	informer cache.SharedIndexInformer
	builder  *rules.Builder
	opts     Options

	dir   atomic.Pointer[rules.Directory]
	dirty chan struct{}
	// listed is signalled once, when the initial list completes, so the first directory is
	// published without waiting out the debounce window.
	listed chan struct{}
	synced atomic.Bool

	mu        sync.Mutex
	lastError error
	crdGone   bool
}

// New builds a Source over a dynamic client. Run must be called for it to do anything.
func New(client dynamic.Interface, opts Options) *Source {
	if opts.Debounce <= 0 {
		opts.Debounce = DefaultDebounce
	}
	if opts.Logf == nil {
		opts.Logf = func(string, ...interface{}) {}
	}
	informer := dynamicinformer.NewFilteredDynamicInformer(client, rules.GroupVersionResource, "", opts.Resync, cache.Indexers{}, nil).Informer()
	s := &Source{
		informer: informer,
		builder:  rules.NewBuilder(),
		opts:     opts,
		dirty:    make(chan struct{}, 1),
		listed:   make(chan struct{}, 1),
	}
	// Errors during the initial list normally surface only as a log line of the reflector; a missing
	// CRD must be told apart from an unreachable API server, so the handler records them.
	_ = informer.SetWatchErrorHandler(s.watchError)
	_ = informer.SetTransform(rules.Project)
	_, _ = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(interface{}) { s.markDirty() },
		UpdateFunc: func(oldObj, newObj interface{}) {
			// Both objects have already been through Project, which keeps only the fields the
			// directory reads. An update that leaves them equal (a label, an annotation, a
			// re-list) would rebuild every map to the same result, so skip it.
			if specUnchanged(oldObj, newObj) {
				return
			}
			s.markDirty()
		},
		DeleteFunc: func(interface{}) { s.markDirty() },
	})
	return s
}

// Directory returns the current directory, or nil before the first successful build. Consumers must
// treat nil as "the rules are not known yet", not as "there are no rules".
func (s *Source) Directory() *rules.Directory {
	return s.dir.Load()
}

// HasSynced reports whether the directory reflects the cluster at least once.
func (s *Source) HasSynced() bool {
	return s.synced.Load()
}

// State reports the freshness state.
func (s *Source) State() State {
	if s.synced.Load() {
		return StateSynced
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.crdGone {
		return StateCRDMissing
	}
	return StateUnsynced
}

// LastError returns the last list/watch error, nil when the last attempt succeeded.
func (s *Source) LastError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastError
}

// Run starts the informer and the rebuild loop and blocks until ctx is done. The first directory is
// published as soon as the initial list completes; a missing CRD does not block Run, the informer
// retries until the CRD appears.
func (s *Source) Run(ctx context.Context) {
	go s.informer.Run(ctx.Done())
	go s.awaitSync(ctx)
	s.rebuildLoop(ctx)
}

// awaitSync marks the first sync. It never gives up: a CRD that does not exist at bootstrap will
// be created by the module release that also starts the consumer.
func (s *Source) awaitSync(ctx context.Context) {
	if !cache.WaitForCacheSync(ctx.Done(), s.informer.HasSynced) {
		return
	}
	s.mu.Lock()
	s.crdGone = false
	s.lastError = nil
	s.mu.Unlock()
	// Publish the first directory at once. There is nothing to coalesce yet, and until it exists
	// every subject of a rule is treated as restricted, so waiting out the debounce window would be
	// pure added exposure.
	select {
	case s.listed <- struct{}{}:
	default:
	}
}

// specUnchanged reports whether two projected rules carry the same spec. It is deliberately
// conservative: anything it cannot compare counts as changed.
func specUnchanged(oldObj, newObj interface{}) bool {
	oldRule, ok := oldObj.(*unstructured.Unstructured)
	if !ok {
		return false
	}
	newRule, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		return false
	}
	oldSpec, _, err := unstructured.NestedFieldNoCopy(oldRule.Object, "spec")
	if err != nil {
		return false
	}
	newSpec, _, err := unstructured.NestedFieldNoCopy(newRule.Object, "spec")
	if err != nil {
		return false
	}
	return equality.Semantic.DeepEqual(oldSpec, newSpec)
}

func (s *Source) markDirty() {
	select {
	case s.dirty <- struct{}{}:
	default:
	}
}

func (s *Source) watchError(_ *cache.Reflector, err error) {
	s.mu.Lock()
	s.lastError = err
	s.crdGone = apierrors.IsNotFound(err)
	s.mu.Unlock()
	if s.opts.Observer != nil {
		s.opts.Observer.WatchError(err)
	}
	s.opts.Logf("rules source: list/watch error: %v", err)
}

// rebuildLoop folds bursts of events into one build per debounce window.
func (s *Source) rebuildLoop(ctx context.Context) {
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	armed := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.listed:
			// Publish the first directory without waiting out the debounce window. The signal can
			// also arrive after the debounced path already published one - WaitForCacheSync polls,
			// so the add handlers of the initial list may well have fired a build first - and then
			// there is nothing to do: rebuilding the same store again is pure waste.
			if s.Directory() != nil {
				continue
			}
			if armed {
				timer.Stop()
				armed = false
			}
			// The initial list fired the add handler of every object, and the build below is exactly
			// what that signal asks for.
			select {
			case <-s.dirty:
			default:
			}
			if s.informer.HasSynced() {
				s.rebuild()
			}
		case <-s.dirty:
			if !armed {
				timer.Reset(s.opts.Debounce)
				armed = true
			}
		case <-timer.C:
			armed = false
			if !s.informer.HasSynced() {
				continue
			}
			s.rebuild()
		}
	}
}

// rebuild publishes a directory built from the informer store.
func (s *Source) rebuild() {
	started := time.Now()
	objects := s.informer.GetStore().List()
	list := make([]rules.Rule, 0, len(objects))
	for _, obj := range objects {
		u, ok := obj.(*unstructured.Unstructured)
		if !ok {
			continue
		}
		rule, err := rules.FromUnstructured(u)
		if err != nil {
			// The CRD schema validates the spec, so this is a programming error rather than a user
			// one; still, one rule must not take the directory down with it.
			s.opts.Logf("rules source: skip %q: %v", u.GetName(), err)
			continue
		}
		list = append(list, rule)
	}
	dir, stats := s.builder.Build(list)
	s.dir.Store(dir)
	first := !s.synced.Swap(true)
	took := time.Since(started)

	for name, err := range stats.Quarantined {
		s.opts.Logf("rules source: rule %q quarantined: %v", name, err)
	}
	if s.opts.Observer != nil {
		s.opts.Observer.DirectoryRebuilt(stats, took)
		if first {
			s.opts.Observer.SyncedChanged(true)
		}
	}
	if s.opts.OnUpdate != nil {
		s.opts.OnUpdate(dir, stats)
	}
	s.opts.Logf("rules source: directory rebuilt from %d rules (%d subjects, %d quarantined) in %s", stats.Rules, stats.Subjects, len(stats.Quarantined), took)
}
