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

package ledger

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/quota"
)

const (
	// confirmAfter is the age of a pending approval at which a quorum read settles it.
	confirmAfter = 15 * time.Second
	// drainAge is the age of a pending approval a leader that steps down settles at once.
	drainAge = 100 * time.Millisecond
	// livenessWindow is how old the last tick of the main loop may be before the liveness probe fails.
	livenessWindow = 60 * time.Second
	// relistAfter is how long the informers may take to catch up with a quorum list before it is taken
	// again.
	relistAfter = 30 * time.Second
)

// Options configure a run of the ledger.
type Options struct {
	Config  *rest.Config
	Self    Self
	CertDir string
	Logger  logr.Logger
	// WebhookAddress, HealthAddress and MetricsAddress are the listen addresses of the webhook, of the
	// probes and of the metrics.
	WebhookAddress string
	HealthAddress  string
	MetricsAddress string
}

// NewRunID returns a random identifier of this run of the process.
func NewRunID() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}

type runner struct {
	opts      Options
	timing    Timing
	log       logr.Logger
	state     *State
	metrics   *Metrics
	reader    client.Client
	cache     cache.Cache
	lock      *FencedLock
	endpoints *Endpoints
	watchdog  *Watchdog
	webhook   *http.Server

	leader   atomic.Bool
	synced   atomic.Bool
	stopping atomic.Bool
	lastTick atomic.Int64

	// flagsMu orders the changes of leader and synced by a start of the leadership and by its end.
	flagsMu sync.Mutex

	// deleted are the recent deletes the state has applied, which catchUp tells from a lagging informer.
	deleted deletions
}

// Run serves the ledger until the context is cancelled, then hands the lease over.
func Run(ctx context.Context, opts Options) error {
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, discoveryv1.AddToScheme, coordinationv1.AddToScheme, v1alpha1.AddToScheme} {
		if err := add(scheme); err != nil {
			return fmt.Errorf("build the scheme: %w", err)
		}
	}
	reader, err := client.New(opts.Config, client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("create the API client: %w", err)
	}

	cm := new(corev1.ConfigMap)
	if err := reader.Get(ctx, client.ObjectKey{Namespace: Namespace, Name: quota.ConfigMapName}, cm); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get the %s ConfigMap: %w", quota.ConfigMapName, err)
		}
		cm = nil
	}
	timing, err := TimingFromConfigMap(cm)
	if err != nil {
		return fmt.Errorf("the intervals of the ledger: %w", err)
	}

	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	r := &runner{opts: opts, timing: timing, log: opts.Logger, state: NewState(), metrics: NewMetrics(registry), reader: reader}
	r.endpoints = &Endpoints{Client: reader, Self: opts.Self, Metrics: r.metrics}
	r.watchdog = &Watchdog{Reader: reader, Endpoints: r.endpoints}
	r.lastTick.Store(time.Now().UnixNano())

	if r.cache, err = newCache(opts.Config, scheme); err != nil {
		return err
	}
	if err := r.watch(ctx); err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(opts.Config)
	if err != nil {
		return fmt.Errorf("create the clientset: %w", err)
	}
	inner, err := resourcelock.New(resourcelock.LeasesResourceLock, Namespace, LeaseName, clientset.CoreV1(), clientset.CoordinationV1(),
		resourcelock.ResourceLockConfig{Identity: opts.Self.Identity()})
	if err != nil {
		return fmt.Errorf("create the lease lock: %w", err)
	}
	r.lock = &FencedLock{Interface: inner}

	watcher, err := certwatcher.New(filepath.Join(opts.CertDir, "tls.crt"), filepath.Join(opts.CertDir, "tls.key"))
	if err != nil {
		return fmt.Errorf("read the certificate of the ledger: %w", err)
	}

	serveCtx, stopServing := context.WithCancel(context.Background())
	defer stopServing()
	go func() {
		if err := watcher.Start(serveCtx); err != nil {
			r.log.Error(err, "watch the certificate of the ledger")
		}
	}()
	go func() {
		if err := r.cache.Start(serveCtx); err != nil {
			r.log.Error(err, "run the informers of the ledger")
		}
	}()
	if !r.cache.WaitForCacheSync(ctx) {
		return errors.New("the informers of the ledger did not sync")
	}

	handler := &Handler{State: r.state, Gate: r.gate, Metrics: r.metrics}
	mux := http.NewServeMux()
	mux.Handle("/validate/quota-ledger", handler)
	r.webhook = &http.Server{
		Addr:              opts.WebhookAddress,
		Handler:           mux,
		TLSConfig:         &tls.Config{MinVersion: tls.VersionTLS12, GetCertificate: watcher.GetCertificate},
		ReadHeaderTimeout: 5 * time.Second,
		// a body that comes slowly does not hold a place of its source longer than this
		ReadTimeout: 5 * time.Second,
		IdleTimeout: 90 * time.Second,
	}
	r.webhook.SetKeepAlivesEnabled(false)
	servers := []*http.Server{r.webhook, r.healthServer(), {Addr: opts.MetricsAddress, Handler: promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
		ReadHeaderTimeout: 5 * time.Second}}
	for i, srv := range servers {
		go func() {
			var err error
			if i == 0 {
				err = srv.ListenAndServeTLS("", "")
			} else {
				err = srv.ListenAndServe()
			}
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				r.log.Error(err, "serve", "address", srv.Addr)
			}
		}()
	}

	electCtx, stopElecting := context.WithCancel(context.Background())
	elected := make(chan struct{})
	go func() {
		defer close(elected)
		r.elect(electCtx)
	}()
	go r.loop(serveCtx)

	<-ctx.Done()
	r.stopping.Store(true)
	r.handOver()
	stopElecting()
	<-elected
	for _, srv := range servers {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := srv.Shutdown(shutdownCtx); err != nil {
			r.log.Error(err, "shut the server down", "address", srv.Addr)
		}
		cancel()
	}
	return nil
}

// newCache makes the informers of the ledger: only d8-project-quota among the ResourceQuota objects,
// only the namespaces of projects, and the objects without their managed fields.
func newCache(config *rest.Config, scheme *runtime.Scheme) (cache.Cache, error) {
	inProject, err := labels.NewRequirement(v1alpha3.ResourceLabelProject, selection.Exists, nil)
	if err != nil {
		return nil, fmt.Errorf("select the namespaces of projects: %w", err)
	}
	c, err := cache.New(config, cache.Options{
		Scheme: scheme,
		ByObject: map[client.Object]cache.ByObject{
			&corev1.ResourceQuota{}: {Field: fields.OneTermEqualSelector("metadata.name", v1alpha3.ProjectQuotaName)},
			&corev1.Namespace{}:     {Label: labels.NewSelector().Add(*inProject)},
		},
		DefaultTransform: cache.TransformStripManagedFields(),
	})
	if err != nil {
		return nil, fmt.Errorf("create the informers of the ledger: %w", err)
	}
	return c, nil
}

// watch feeds the events of the informers into the state.
func (r *runner) watch(ctx context.Context) error {
	handlers := map[client.Object]toolscache.ResourceEventHandlerFuncs{
		&corev1.ResourceQuota{}: {
			AddFunc:    func(obj any) { r.onQuota(obj) },
			UpdateFunc: func(_, obj any) { r.onQuota(obj) },
			DeleteFunc: func(obj any) {
				if rq, ok := unwrap(obj).(*corev1.ResourceQuota); ok {
					r.state.DeleteQuota(rq.Namespace)
					r.deleted.record(rq)
				}
			},
		},
		&corev1.Namespace{}: {
			AddFunc:    func(obj any) { r.onNamespace(obj) },
			UpdateFunc: func(_, obj any) { r.onNamespace(obj) },
			DeleteFunc: func(obj any) {
				if ns, ok := unwrap(obj).(*corev1.Namespace); ok {
					r.state.DeleteNamespace(ns.Name)
					r.deleted.record(ns)
				}
			},
		},
		&v1alpha1.ProjectQuota{}: {
			AddFunc:    func(obj any) { r.onProjectQuota(obj) },
			UpdateFunc: func(_, obj any) { r.onProjectQuota(obj) },
			DeleteFunc: func(obj any) {
				if pq, ok := unwrap(obj).(*v1alpha1.ProjectQuota); ok {
					r.state.DeleteProjectQuota(pq.Name)
					r.deleted.record(pq)
				}
			},
		},
	}
	for object, funcs := range handlers {
		informer, err := r.cache.GetInformer(ctx, object)
		if err != nil {
			return fmt.Errorf("get the informer of %T: %w", object, err)
		}
		if _, err := informer.AddEventHandler(funcs); err != nil {
			return fmt.Errorf("watch %T: %w", object, err)
		}
	}
	return nil
}

func unwrap(obj any) any {
	if tombstone, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
		return tombstone.Obj
	}
	return obj
}

func (r *runner) onQuota(obj any) {
	if rq, ok := obj.(*corev1.ResourceQuota); ok {
		if confirmed := r.state.SetQuota(rq); confirmed > 0 {
			r.metrics.PendingResolved.WithLabelValues("confirmed").Add(float64(confirmed))
		}
	}
}

func (r *runner) onNamespace(obj any) {
	if ns, ok := obj.(*corev1.Namespace); ok {
		r.state.SetNamespace(ns)
	}
}

func (r *runner) onProjectQuota(obj any) {
	if pq, ok := obj.(*v1alpha1.ProjectQuota); ok {
		r.state.SetProjectQuota(pq)
	}
}

// gate says whether the ledger answers: only the holder of the lease, inside the fence, once its
// informers caught up with its quorum list.
func (r *runner) gate() (string, string) {
	switch {
	case !r.leader.Load():
		return ResultNotLeader, "not the leader"
	case !r.lock.Fresh(r.timing.Fence):
		return ResultFenced, "the lease is not renewed"
	case !r.synced.Load():
		return ResultSyncing, "syncing"
	}
	return "", ""
}

// elect runs the leader election until the context is cancelled. A replica that loses the lease
// becomes a candidate again; client-go returns from Run when the lease is lost.
func (r *runner) elect(ctx context.Context) {
	for ctx.Err() == nil {
		elector, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
			Lock:            r.lock,
			LeaseDuration:   r.timing.Lease,
			RenewDeadline:   r.timing.RenewDeadline,
			RetryPeriod:     r.timing.Retry,
			ReleaseOnCancel: true,
			Name:            LeaseName,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: r.lead,
				OnStoppedLeading: r.stepDown,
			},
		})
		if err != nil {
			r.log.Error(err, "configure the leader election")
			return
		}
		elector.Run(ctx)
	}
}

// lead takes the ledger over: a quorum list, the informers catching up with it, the address of this
// replica in the slice, and then the answers.
func (r *runner) lead(ctx context.Context) {
	// client-go starts this in a goroutine and cancels its context before it calls stepDown, so a lead
	// that runs late sees the cancelled context under the same lock and leaves the flags alone.
	if !r.whileLeading(ctx, func() { r.leader.Store(true) }) {
		return
	}
	r.metrics.Leader.WithLabelValues(r.opts.Self.PodName).Set(1)
	r.log.Info("took the ledger lease", "identity", r.opts.Self.Identity())

	// a failed list is retried for as long as the lease is held: a leader that gave up here would hold
	// the lease and refuse every write until its pod restarts
	if err := r.catchUp(ctx); err != nil {
		return
	}
	if !r.whileLeading(ctx, func() { r.synced.Store(true) }) {
		return
	}
	r.metrics.Synced.Set(1)
	r.webhook.SetKeepAlivesEnabled(true)

	ticker := time.NewTicker(r.timing.Retry)
	defer ticker.Stop()
	for {
		if r.mayAct() {
			fenced, cancel := r.withinFence(ctx)
			if err := r.endpoints.Publish(fenced, r.mayAct); err != nil {
				r.log.Error(err, "publish the address of the leader")
			}
			cancel()
		}
		r.resolve(ctx, time.Now().Add(-confirmAfter), true)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// mayAct reports whether the leader may write the route of the ledger: it answers, and its last
// renewal was sent within the fence. A leader that stops answering withdraws its address and must not
// put it back; and a leader resumed after a pause keeps its context until the renew deadline, while
// another replica may hold the lease and the route already.
func (r *runner) mayAct() bool {
	return !r.stopping.Load() && r.synced.Load() && r.lock.Fresh(r.timing.Fence)
}

// withinFence bounds a write of the leader by the end of its fence: a call that a slow API server holds
// past it is cancelled rather than written by a replica that may have lost the lease.
func (r *runner) withinFence(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithDeadline(ctx, r.lock.Deadline(r.timing.Fence))
}

// whileLeading runs the change of the flags unless the leadership the context stands for has ended.
func (r *runner) whileLeading(ctx context.Context, change func()) bool {
	r.flagsMu.Lock()
	defer r.flagsMu.Unlock()
	if ctx.Err() != nil {
		return false
	}
	change()
	return true
}

// stepDown stops the answers of a replica that lost the lease and closes its idle connections.
func (r *runner) stepDown() {
	r.flagsMu.Lock()
	defer r.flagsMu.Unlock()
	if r.leader.Swap(false) {
		reason := "expired"
		if r.stopping.Load() {
			reason = "graceful"
		}
		r.metrics.Handovers.WithLabelValues(reason).Inc()
	}
	r.synced.Store(false)
	r.lock.Forget()
	r.metrics.Leader.WithLabelValues(r.opts.Self.PodName).Set(0)
	r.metrics.Synced.Set(0)
	r.webhook.SetKeepAlivesEnabled(false)
	r.log.Info("left the ledger lease", "identity", r.opts.Self.Identity())
}

// handOver gives the lease away on a stop: no more approvals, the address out of the slice, the
// connections closed, the pending approvals confirmed, and then the lease released by the election.
// An approval whose write has not landed yet is kept: the write may still land, in a store that is
// slow to save it, and a new leader that took the lease at once would approve without it. The lease
// is held until every approval is confirmed or the fence window has passed since the last one.
func (r *runner) handOver() {
	if !r.leader.Load() {
		return
	}
	start := time.Now()
	r.synced.Store(false)
	r.webhook.SetKeepAlivesEnabled(false)
	ctx, cancel := context.WithTimeout(context.Background(), MinFenceWindow+r.timing.WebhookTimeout+time.Second)
	defer cancel()
	if err := r.endpoints.Withdraw(ctx, r.opts.Self.PodName, "leader"); err != nil {
		r.log.Error(err, "withdraw the address of the leader")
	}
	// the API server takes an approval for up to the webhook timeout after it was given
	time.Sleep(r.timing.WebhookTimeout)
	for r.state.PendingCount() > 0 && time.Since(start) < MinFenceWindow {
		r.resolve(ctx, time.Now().Add(-drainAge), false)
		time.Sleep(drainAge)
	}
	r.metrics.HandoverDrain.Observe(time.Since(start).Seconds())
}

// resolve settles the pending approvals older than the time with a quorum read of their quota: the
// ones the read shows written, and with settle the ones it shows not written too, which are old
// enough to have failed.
func (r *runner) resolve(ctx context.Context, before time.Time, settle bool) {
	for _, aged := range r.state.Aged(before) {
		rq := new(corev1.ResourceQuota)
		err := r.reader.Get(ctx, client.ObjectKey{Namespace: aged.Namespace, Name: v1alpha3.ProjectQuotaName}, rq)
		switch {
		case apierrors.IsNotFound(err):
			r.state.DeleteQuota(aged.Namespace)
			continue
		case err != nil:
			r.log.Error(err, "read the quota of a pending approval", "namespace", aged.Namespace)
			continue
		}
		outcome := "get_unchanged"
		if ParseResourceVersion(rq.ResourceVersion) > aged.RVOld {
			outcome = "get_newer"
		}
		if n := r.state.Resolve(rq, aged.RVOld, before, settle); n > 0 {
			r.metrics.PendingResolved.WithLabelValues(outcome).Add(float64(n))
		}
	}
}

// catchUp takes a quorum list of what the ledger watches and waits until the informers and the state
// they feed have it all. It retries a failed list and returns only once caught up or when the context
// ends.
func (r *runner) catchUp(ctx context.Context) error {
	for {
		lists, err := r.quorumLists(ctx)
		if err != nil {
			r.log.Error(err, "take the quorum list; retrying")
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(r.timing.Retry):
			}
			continue
		}
		deadline := time.Now().Add(relistAfter)
		for time.Now().Before(deadline) {
			done, err := r.covered(ctx, lists)
			if err != nil {
				r.log.Error(err, "compare the informers with the quorum list")
			}
			if done {
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(50 * time.Millisecond):
			}
		}
	}
}

type quorumList struct {
	cached client.ObjectList
	listed []metav1.Object
	listRV uint64
}

func (r *runner) quorumLists(ctx context.Context) ([]quorumList, error) {
	inProject, err := labels.NewRequirement(v1alpha3.ResourceLabelProject, selection.Exists, nil)
	if err != nil {
		return nil, fmt.Errorf("select the namespaces of projects: %w", err)
	}
	kinds := []struct {
		list    client.ObjectList
		cached  client.ObjectList
		options []client.ListOption
	}{
		{&corev1.ResourceQuotaList{}, &corev1.ResourceQuotaList{}, []client.ListOption{client.MatchingFields{"metadata.name": v1alpha3.ProjectQuotaName}}},
		{&corev1.NamespaceList{}, &corev1.NamespaceList{}, []client.ListOption{client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(*inProject)}}},
		{&v1alpha1.ProjectQuotaList{}, &v1alpha1.ProjectQuotaList{}, nil},
	}
	out := make([]quorumList, 0, len(kinds))
	for _, kind := range kinds {
		if err := r.reader.List(ctx, kind.list, kind.options...); err != nil {
			return nil, fmt.Errorf("list %T: %w", kind.list, err)
		}
		objects, err := metaObjects(kind.list)
		if err != nil {
			return nil, err
		}
		listMeta, err := meta.ListAccessor(kind.list)
		if err != nil {
			return nil, fmt.Errorf("read the version of %T: %w", kind.list, err)
		}
		out = append(out, quorumList{cached: kind.cached, listed: objects, listRV: ParseResourceVersion(listMeta.GetResourceVersion())})
	}
	return out, nil
}

func (r *runner) covered(ctx context.Context, lists []quorumList) (bool, error) {
	// An object deleted after the list is covered once its delete is in the state: it is left out of
	// both checks. The record comes after the state and after the store of the informer, so a delete
	// that is recorded is in both.
	remaining := make([][]metav1.Object, len(lists))
	for i, list := range lists {
		remaining[i] = r.deleted.remaining(list.listed)
	}
	// the event handlers fill the state after the store of the informer, so the state is checked too
	for i := range lists {
		if !r.state.Covers(remaining[i]) {
			return false, nil
		}
	}
	for i, list := range lists {
		if err := r.cache.List(ctx, list.cached); err != nil {
			return false, fmt.Errorf("list the informer of %T: %w", list.cached, err)
		}
		cached, err := metaObjects(list.cached)
		if err != nil {
			return false, err
		}
		if !caughtUp(remaining[i], cached, list.listRV) {
			return false, nil
		}
	}
	return true, nil
}

func metaObjects(list client.ObjectList) ([]metav1.Object, error) {
	items, err := meta.ExtractList(list)
	if err != nil {
		return nil, fmt.Errorf("read the items of %T: %w", list, err)
	}
	out := make([]metav1.Object, 0, len(items))
	for _, item := range items {
		object, err := meta.Accessor(item)
		if err != nil {
			return nil, fmt.Errorf("read an item of %T: %w", list, err)
		}
		out = append(out, object)
	}
	return out, nil
}

// loop is the main loop of every replica: the watchdog of a follower, the gauges, and the tick the
// liveness probe reads.
func (r *runner) loop(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		r.lastTick.Store(time.Now().UnixNano())
		if !r.leader.Load() && !r.stopping.Load() {
			if err := r.watchdog.Tick(ctx); err != nil {
				r.log.Error(err, "watch the ledger lease")
			}
		}
		r.metrics.Pending.Set(float64(r.state.PendingCount()))
		r.metrics.InflightRatioMax.Set(r.state.InflightRatioMax())
		r.metrics.LeaseRenewAge.Set(r.lock.RenewAge().Seconds())
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// healthServer serves the probes: /healthz while the process runs, /livez while the main loop ticks,
// and /ledger-health with the state of this replica for the other one.
func (r *runner) healthServer() *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/livez", func(w http.ResponseWriter, _ *http.Request) {
		if time.Since(time.Unix(0, r.lastTick.Load())) > livenessWindow {
			http.Error(w, "the main loop of the ledger is stuck", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/ledger-health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]bool{"leader": r.leader.Load(), "synced": r.synced.Load()}); err != nil {
			return
		}
	})
	return &http.Server{Addr: r.opts.HealthAddress, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
}
