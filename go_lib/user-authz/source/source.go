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
	"strconv"
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
	// StateStale: the rules were listed once, and the watch has been failing ever since. The
	// directory is real but is no longer tracking the cluster, which is worth reporting as not
	// ready - the rules it serves are as old as the moment the watch broke.
	StateStale
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
	case StateStale:
		return "stale"
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
	// DegradeAfterWatchErrors is how many consecutive list/watch failures, with no success in
	// between, make a source that has already synced report itself stale. Zero means
	// DefaultDegradeAfterWatchErrors.
	DegradeAfterWatchErrors int
}

// DefaultDebounce is the rebuild coalescing window: long enough to fold the initial list of thousands
// of rules and the bursts a batch of changes produces into one build, short enough to stay far below
// the API server's 30 s cache of authorizer answers.
const DefaultDebounce = 200 * time.Millisecond

// DefaultDegradeAfterWatchErrors is how many consecutive list/watch failures it takes before a
// source that listed the cluster once admits it is no longer tracking it.
//
// Twenty is deliberately lenient. This gates the readiness of a DaemonSet on every master of a
// fail-closed authorizer, so the cost of being too eager is a rollout that stalls, or an operator
// chasing an instance that was fine - while the cost of being too slow is only that a stale
// directory is reported by its metrics rather than by readiness, which is where it was reported
// before this existed at all. client-go backs the reflector off to about thirty seconds between
// retries, so twenty failures is on the order of ten minutes of continuous failure.
//
// The number has not been calibrated against a real degradation; see the card in specs for the
// experiment that would justify tightening it.
const DefaultDegradeAfterWatchErrors = 20

// crdMissingConfirmations is how many consecutive NotFound errors it takes to conclude that the
// ClusterAuthorizationRule CRD is not served. See the reasoning in watchError.
const crdMissingConfirmations = 3

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
	// consecutiveWatchErrors counts list/watch failures with no success in between. It is what
	// turns a permanently broken watch into a state change, and it is reset by every successful
	// sync and by every rebuild that the informer feeds.
	consecutiveWatchErrors int
	degradeAfter           int
	// crdMissingStreak counts consecutive NotFound errors. One is not enough to conclude that the
	// CRD is not served: see the comment in watchError.
	crdMissingStreak int
	// maxSeenResourceVersion is the highest resourceVersion this instance has OBSERVED, which is
	// not the same as the highest the directory was built from: an update that changes nothing the
	// directory reads is skipped, and skipping it must not make two instances look like they
	// disagree. See noteResourceVersion.
	maxSeenResourceVersion uint64

	// muNotify serialises the state report to the observer, so two goroutines that change the
	// state at the same time cannot deliver their answers in the wrong order.
	muNotify sync.Mutex
	// syncedReported is the last value handed to Observer.SyncedChanged; nil until the first one.
	syncedReported *bool
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
	if opts.DegradeAfterWatchErrors <= 0 {
		opts.DegradeAfterWatchErrors = DefaultDegradeAfterWatchErrors
	}
	s := &Source{
		degradeAfter: opts.DegradeAfterWatchErrors,
		informer:     informer,
		builder:      rules.NewBuilder(),
		opts:         opts,
		dirty:        make(chan struct{}, 1),
		listed:       make(chan struct{}, 1),
	}
	// Errors during the initial list normally surface only as a log line of the reflector; a missing
	// CRD must be told apart from an unreachable API server, so the handler records them.
	_ = informer.SetWatchErrorHandler(s.watchError)
	_ = informer.SetTransform(rules.Project)
	_, _ = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) { s.noteWatchAliveAndReport(); s.noteResourceVersion(obj); s.markDirty() },
		UpdateFunc: func(oldObj, newObj interface{}) {
			// Reaching this at all means the watch is delivering, so the error streak ends here
			// even when the update itself changes nothing the directory reads. A reflector that
			// re-lists after an error generates an update for every object it holds, which is what
			// makes recovery observable on a cluster whose rules never change.
			s.noteWatchAliveAndReport()
			s.noteResourceVersion(newObj)
			// Both objects have already been through Project, which keeps only the fields the
			// directory reads. An update that leaves them equal (a label, an annotation, a
			// re-list) would rebuild every map to the same result, so skip it.
			if specUnchanged(oldObj, newObj) {
				return
			}
			s.markDirty()
		},
		DeleteFunc: func(obj interface{}) { s.noteWatchAliveAndReport(); s.noteResourceVersion(obj); s.markDirty() },
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
//
// A source that listed the cluster once and then lost its watch for good - RBAC revoked, a wedged
// reflector, an API server that keeps refusing - used to stay Synced forever, because the first
// list had succeeded and nothing ever took that back. The directory it serves is then a snapshot of
// whenever the watch broke, and readiness had no way to notice.
//
// After DegradeAfterWatchErrors consecutive failures with no success in between, it reports
// Unsynced again. The threshold is deliberately lenient: this predicate gates the readiness of a
// DaemonSet on every master of a fail-closed authorizer, so a handful of transient errors must not
// move it. client-go backs off to about thirty seconds between reflector retries, so the default
// is on the order of ten minutes of continuous failure.
func (s *Source) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.crdGone {
		return StateCRDMissing
	}
	if s.synced.Load() && s.consecutiveWatchErrors < s.degradeAfter {
		return StateSynced
	}
	if s.synced.Load() {
		return StateStale
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
	s.crdMissingStreak = 0
	s.lastError = nil
	s.consecutiveWatchErrors = 0
	s.mu.Unlock()
	s.notifySynced()
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

// noteWatchAlive ends the error streak that State() reads.
//
// The streak has to be *consecutive* for the threshold to mean anything, and the only reset used to
// be in awaitSync - which runs once, at the first sync, and never again. Watch errors are a normal
// part of a long-lived cluster: every API server restart produces some. So the count crept up over
// days until it crossed the threshold, and then State() said Stale for the rest of the process's
// life, holding both consumers unready forever - a DaemonSet rollout that never finishes and an
// aggregated apiserver dropped from its Service, on a cluster where nothing was actually wrong.
func (s *Source) noteWatchAlive() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.consecutiveWatchErrors == 0 && s.crdMissingStreak == 0 && s.lastError == nil && !s.crdGone {
		return
	}
	recovered := s.consecutiveWatchErrors >= s.degradeAfter
	s.consecutiveWatchErrors = 0
	s.crdMissingStreak = 0
	s.lastError = nil
	s.crdGone = false
	if recovered {
		s.opts.Logf("rules source: the watch is delivering again; this instance is no longer stale")
	}
}

// noteWatchAliveAndReport is noteWatchAlive plus the state report. The report cannot happen inside
// noteWatchAlive: that method holds the lock State() needs.
func (s *Source) noteWatchAliveAndReport() {
	s.noteWatchAlive()
	s.notifySynced()
}

// noteResourceVersion records the highest resourceVersion this instance has seen.
//
// The watermark is what an operator compares between masters to find the one that is behind, and it
// used to be read off the directory - the highest version among the rules it was BUILT from. That
// makes it a function of when each instance last rebuilt rather than of what each instance knows: a
// metadata-only change bumps the version in the cluster and is deliberately skipped by the rebuild,
// so an instance that restarted afterwards picks the new version up in its initial list while one
// that did not keeps the old one. Two instances holding identical rules then report different
// numbers, permanently, and the divergence alert fires on a cluster where nothing is wrong.
//
// Observed, not built-from, is the honest meaning and the one the alert needs.
func (s *Source) noteResourceVersion(obj interface{}) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		if tombstone, isTombstone := obj.(cache.DeletedFinalStateUnknown); isTombstone {
			u, ok = tombstone.Obj.(*unstructured.Unstructured)
		}
		if !ok {
			return
		}
	}
	rv, err := strconv.ParseUint(u.GetResourceVersion(), 10, 64)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if rv > s.maxSeenResourceVersion {
		s.maxSeenResourceVersion = rv
	}
}

// seenResourceVersion returns the watermark for the metrics.
func (s *Source) seenResourceVersion() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maxSeenResourceVersion
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
	s.consecutiveWatchErrors++
	// crdMissing needs corroboration. A single NotFound is not proof that the CRD is not served -
	// the API server returns it during its own startup, and an aggregated layer returns it while a
	// backend restarts - and concluding CRDMissing from one is expensive: it is the one state that
	// OPENS the webhook's serving gate, on the reasoning that a cluster without the CRD has no
	// rules to wait for. Reached wrongly on a cluster that does have rules, it opens the gate with
	// an empty directory, and the ordering guard then denies every subject a rule covers, for
	// unauthorizedTTL each.
	//
	// A CRD that genuinely is not served produces these continuously, so requiring a short streak
	// costs a few seconds of bootstrap and rules out the transient case.
	if apierrors.IsNotFound(err) {
		s.crdMissingStreak++
	} else {
		s.crdMissingStreak = 0
	}
	s.crdGone = s.crdMissingStreak >= crdMissingConfirmations
	degraded := s.synced.Load() && s.consecutiveWatchErrors == s.degradeAfter
	s.mu.Unlock()

	if degraded {
		s.opts.Logf("rules source: %d list/watch errors in a row; the directory is no longer tracking "+
			"the cluster and this instance reports itself stale", s.degradeAfter)
	}
	if s.opts.Observer != nil {
		s.opts.Observer.WatchError(err)
	}
	s.notifySynced()
	s.opts.Logf("rules source: list/watch error: %v", err)
}

// notifySynced tells the observer whether this source is currently tracking the cluster, whenever
// that answer changes.
//
// It used to be reported once, on the first sync, and never again - so the gauge said "synced"
// for the rest of the process's life, including while State() said Stale or CRDMissing and the
// consumer was answering 503 to every request. The one metric an operator would reach for to see
// that was the one metric that could not show it.
//
// The state is read and reported under the same lock, so two goroutines changing the state
// concurrently cannot report their answers out of order.
func (s *Source) notifySynced() {
	if s.opts.Observer == nil {
		return
	}

	s.muNotify.Lock()
	defer s.muNotify.Unlock()

	synced := s.State() == StateSynced
	if s.syncedReported != nil && *s.syncedReported == synced {
		return
	}
	s.syncedReported = &synced
	s.opts.Observer.SyncedChanged(synced)
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
	// unreadable are the rules that could not be projected at all, counted with the quarantined
	// ones below.
	var unreadable map[string]error
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
			if unreadable == nil {
				unreadable = make(map[string]error)
			}
			unreadable[u.GetName()] = err
			continue
		}
		list = append(list, rule)
	}
	dir, stats := s.builder.Build(list)

	// A rule that could not be read at all stays out of the directory, which is the safe place for
	// it: the ordering guard restricts every subject its bindings name until the rule appears. It
	// used to stay out of the counting too, and that is not safe - the quarantine gauge read zero
	// and its alert never fired while a rule sat there unapplied, so the one signal that says "a
	// rule in this cluster is not doing what it says" was blind to the case where the rule cannot
	// even be parsed.
	for name, err := range unreadable {
		if stats.Quarantined == nil {
			stats.Quarantined = make(map[string]error, len(unreadable))
		}
		stats.Quarantined[name] = err
	}
	s.dir.Store(dir)
	first := !s.synced.Swap(true)
	took := time.Since(started)

	for name, err := range stats.Quarantined {
		s.opts.Logf("rules source: rule %q quarantined: %v", name, err)
	}
	// The watermark reported to the metrics is what this instance has OBSERVED, not what the
	// directory happened to be built from - see noteResourceVersion for why the difference matters
	// to the divergence alert.
	if seen := s.seenResourceVersion(); seen > stats.MaxResourceVersion {
		stats.MaxResourceVersion = seen
	}

	if s.opts.Observer != nil {
		s.opts.Observer.DirectoryRebuilt(stats, took)
	}
	if first {
		s.opts.Logf("rules source: the first directory is published")
	}
	s.notifySynced()
	if s.opts.OnUpdate != nil {
		s.opts.OnUpdate(dir, stats)
	}
	s.opts.Logf("rules source: directory rebuilt from %d rules (%d subjects, %d quarantined) in %s", stats.Rules, stats.Subjects, len(stats.Quarantined), took)
}
