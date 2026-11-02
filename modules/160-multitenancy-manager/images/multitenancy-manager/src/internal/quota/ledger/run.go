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
	"net"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/quota"
	"controller/internal/quota/evaluator"
)

const (
	// confirmAfter is the age of a pending approval at which a quorum read settles it.
	confirmAfter = 15 * time.Second
	// drainAge is the age of a pending approval a leader that steps down settles at once.
	drainAge = 100 * time.Millisecond
	// livenessWindow is how old the last tick of the main loop may be before the liveness probe fails.
	livenessWindow = 60 * time.Second
	// auditEvery is the interval of the drift audit.
	auditEvery = 60 * time.Second
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
	// Families are the families of objects the ledger counts besides the ResourceQuota objects, the
	// ones whose module is enabled: FamilyVM.
	Families []string
}

// virtualMachineGVK is the kind the vm family counts.
var virtualMachineGVK = schema.GroupVersionKind{Group: "virtualization.deckhouse.io", Version: "v1alpha2", Kind: "VirtualMachine"}

// served reports whether the API server serves the resource of the kind.
func served(config *rest.Config, gvk schema.GroupVersionKind, resource string) bool {
	client, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return false
	}
	// a failed call is tried again, so that one error does not drop a family until the next check
	var resources *metav1.APIResourceList
	for attempt := range servedAttempts {
		if resources, err = client.ServerResourcesForGroupVersion(gvk.GroupVersion().String()); err == nil || apierrors.IsNotFound(err) {
			break
		}
		if attempt < servedAttempts-1 {
			time.Sleep(time.Second)
		}
	}
	if err != nil {
		return false
	}
	for _, r := range resources.APIResources {
		if r.Name == resource {
			return true
		}
	}
	return false
}

// countsVirtualMachines reports whether the ledger counts the vm family.
func (o Options) countsVirtualMachines() bool { return slices.Contains(o.Families, FamilyVM) }

func newVirtualMachine() *unstructured.Unstructured {
	vm := new(unstructured.Unstructured)
	vm.SetGroupVersionKind(virtualMachineGVK)
	return vm
}

func newVirtualMachineList() *unstructured.UnstructuredList {
	list := new(unstructured.UnstructuredList)
	list.SetGroupVersionKind(virtualMachineGVK.GroupVersion().WithKind(virtualMachineGVK.Kind + "List"))
	return list
}

// newVirtualMachineMetadataList is a list of the metadata of the virtual machines: the quorum list of
// catch-up compares only the names and the versions, and the whole machines of a cluster would be the
// peak of the memory of the ledger at every change of the leader.
func newVirtualMachineMetadataList() *metav1.PartialObjectMetadataList {
	list := new(metav1.PartialObjectMetadataList)
	list.SetGroupVersionKind(virtualMachineGVK.GroupVersion().WithKind(virtualMachineGVK.Kind + "List"))
	return list
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

	leader    atomic.Bool
	synced    atomic.Bool
	stopping  atomic.Bool
	ready     atomic.Bool
	suspended atomic.Bool
	// awaitsVirtualMachines tells that the ledger is to count the virtual machines but they were not
	// served when it started.
	awaitsVirtualMachines bool
	lastTick              atomic.Int64

	nonces  *Nonces
	canary  *Canary
	clients *apiServerClients

	roundMu     sync.Mutex
	endRound    context.CancelFunc
	holdOffTill time.Time

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

	// A quorum list of a resource that is not served would fail and keep the ledger from answering
	// at all. The ledger starts without the family and restarts once the resource is served: the
	// module that provides it may come up after this one.
	waitForVM := opts.countsVirtualMachines() && !served(opts.Config, virtualMachineGVK, "virtualmachines")
	r.awaitsVirtualMachines = waitForVM
	if waitForVM {
		opts.Logger.Info("the virtual machines are not served yet; the ledger restarts to count them once they are")
		opts.Families = slices.DeleteFunc(slices.Clone(opts.Families), func(family string) bool { return family == FamilyVM })
		r.opts = opts
	}
	ctx, restart := context.WithCancelCause(ctx)
	defer restart(nil)
	if waitForVM {
		go func() {
			ticker := time.NewTicker(servedCheckEvery)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
				if served(opts.Config, virtualMachineGVK, "virtualmachines") {
					restart(errFamilyServed)
					return
				}
			}
		}()
	}
	if r.cache, err = newCache(opts.Config, scheme, opts.countsVirtualMachines()); err != nil {
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

	r.ready.Store(true)
	r.nonces = &Nonces{}
	r.clients = &apiServerClients{config: opts.Config, clients: map[string]kubernetes.Interface{}}
	r.canary = &Canary{
		Nonces:    r.nonces,
		Prober:    r.clients,
		Servers:   r.apiServers,
		PeerReady: r.peerReady,
		StepDown:  r.stepDownVoluntarily,
		Report:    r.reportNotCalled,
		Load:      r.loadNotCalled,

		RecordSingle: r.recordSingleProbes,
		LoadSingle:   r.loadSingleProbes,
		Metrics:      r.metrics,
		Log:          r.log.WithName("canary"),
	}
	mux := r.routes()
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
	if errors.Is(context.Cause(ctx), errFamilyServed) {
		return errFamilyServed
	}
	return nil
}

// errFamilyServed restarts the ledger: a family it waited for is served now.
var errFamilyServed = errors.New("restart to count the virtual machines, which the API server serves now")

// servedCheckEvery is how often a ledger that waits for a family checks whether it is served.
const servedCheckEvery = 30 * time.Second

// servedAttempts is how many times a failed discovery call is made before a resource is taken as not
// served.
const servedAttempts = 3

// newCache makes the informers of the ledger: only d8-project-quota among the ResourceQuota objects,
// only the namespaces of projects, and the objects without their managed fields.
func newCache(config *rest.Config, scheme *runtime.Scheme, countsVirtualMachines bool) (cache.Cache, error) {
	inProject, err := labels.NewRequirement(v1alpha3.ResourceLabelProject, selection.Exists, nil)
	if err != nil {
		return nil, fmt.Errorf("select the namespaces of projects: %w", err)
	}
	options := cache.Options{
		Scheme: scheme,
		ByObject: map[client.Object]cache.ByObject{
			&corev1.ResourceQuota{}: {Field: fields.OneTermEqualSelector("metadata.name", v1alpha3.ProjectQuotaName)},
			&corev1.Namespace{}:     {Label: labels.NewSelector().Add(*inProject)},
			&corev1.ConfigMap{}: {
				Namespaces: map[string]cache.Config{Namespace: {}},
				Field:      fields.OneTermEqualSelector("metadata.name", SuspendConfigMap),
			},
		},
		DefaultTransform: cache.TransformStripManagedFields(),
	}
	if countsVirtualMachines {
		// every virtual machine of the cluster, with only what the vm family counts it by
		options.ByObject[newVirtualMachine()] = cache.ByObject{Transform: evaluator.TrimVirtualMachine}
	}
	c, err := cache.New(config, options)
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
		&corev1.ConfigMap{}: {
			AddFunc:    func(any) { r.setSuspended(true) },
			UpdateFunc: func(_, _ any) { r.setSuspended(true) },
			DeleteFunc: func(any) { r.setSuspended(false) },
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
	if r.opts.countsVirtualMachines() {
		handlers[newVirtualMachine()] = toolscache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj any) { r.onVirtualMachine(obj) },
			UpdateFunc: func(_, obj any) { r.onVirtualMachine(obj) },
			DeleteFunc: func(obj any) {
				if vm, ok := unwrap(obj).(*unstructured.Unstructured); ok {
					r.state.DeleteObject(FamilyVM, vm.GetNamespace(), vm.GetName())
					r.deleted.record(vm)
				}
			},
		}
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

func (r *runner) setSuspended(suspended bool) {
	if r.suspended.Swap(suspended) != suspended {
		r.log.Info("the quota ledger is suspended by the "+SuspendConfigMap+" ConfigMap: every write passes uncounted", "suspended", suspended)
	}
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

// onVirtualMachine records the usage of a virtual machine. A machine whose usage cannot be read
// counts as nothing: the webhook refuses a write it cannot read, so only an object written past it
// can be such.
func (r *runner) onVirtualMachine(obj any) {
	vm, ok := unwrap(obj).(*unstructured.Unstructured)
	if !ok {
		return
	}
	usage, err := evaluator.VirtualMachine{}.Usage(vm)
	if err != nil {
		r.log.Error(err, "evaluate a virtual machine", "namespace", vm.GetNamespace(), "name", vm.GetName())
		usage = corev1.ResourceList{}
	}
	// A spec the virtualization module has not processed yet may be smaller than what the machine runs
	// with, the GPUs above all, which status.resources does not show: the usage does not go down until
	// the module has said what the change waits for.
	if evaluator.SpecUnobserved(vm) {
		if before, ok := r.state.ObjectUsed(FamilyVM, vm.GetNamespace(), vm.GetName()); ok {
			for key, value := range before {
				if current, ok := usage[key]; !ok || value.Cmp(current) > 0 {
					usage[key] = value.DeepCopy()
				}
			}
		}
	}
	r.state.SetObject(FamilyVM, vm.GetNamespace(), vm.GetName(), ParseResourceVersion(vm.GetResourceVersion()), usage)
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
		r.roundMu.Lock()
		wait := time.Until(r.holdOffTill)
		r.roundMu.Unlock()
		if wait > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
		}
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
		round, endRound := context.WithCancel(ctx)
		r.roundMu.Lock()
		r.endRound = endRound
		r.roundMu.Unlock()
		elector.Run(round)
		endRound()
	}
}

// holdingOff reports whether this replica stepped down lately and does not try to take the lease yet.
func (r *runner) holdingOff() bool {
	r.roundMu.Lock()
	defer r.roundMu.Unlock()
	return time.Now().Before(r.holdOffTill)
}

// stepDownVoluntarily hands the lease over without stopping the process, when the API servers do not
// reach this replica: the handover of a stop, then the lease released, and no new attempt to take it
// for a while.
func (r *runner) stepDownVoluntarily(reason string) {
	r.handOver()
	r.leader.Store(false)
	r.metrics.Handovers.WithLabelValues(reason).Inc()
	r.roundMu.Lock()
	r.holdOffTill = time.Now().Add(stepDownHoldOff)
	endRound := r.endRound
	r.roundMu.Unlock()
	if endRound != nil {
		endRound()
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

	r.canary.Reset()
	go r.runCanary(ctx)

	ticker := time.NewTicker(r.timing.Retry)
	defer ticker.Stop()
	lastAudit := time.Now()
	for {
		if r.mayAct() {
			fenced, cancel := r.withinFence(ctx)
			if err := r.endpoints.Publish(fenced, r.mayAct); err != nil {
				r.log.Error(err, "publish the address of the leader")
			}
			r.wake(fenced)
			cancel()
		}
		r.resolve(ctx, time.Now().Add(-confirmAfter), true)
		r.resolveObjects(ctx, time.Now().Add(-confirmAfter), true)
		if time.Since(lastAudit) >= auditEvery {
			lastAudit = time.Now()
			r.audit(ctx)
		}
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

// errNotActing stops a write of a replica that may no longer act as the leader.
var errNotActing = errors.New("not acting as the leader of the ledger: the lease is not renewed within the fence")

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

// runCanary writes through every API server every few seconds while this replica leads.
func (r *runner) runCanary(ctx context.Context) {
	ticker := time.NewTicker(canaryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if r.mayAct() {
			r.canary.Round(ctx)
		}
	}
}

// reportNotCalled writes the API servers that do not call the webhook into the state ConfigMap of the
// ledger, which the controller turns into the Enforced condition of every quota.
func (r *runner) reportNotCalled(ctx context.Context, notCalled []string) error {
	// a canary round probes every API server and may outlast the fence it started within
	if !r.mayAct() {
		return errNotActing
	}
	ctx, cancel := r.withinFence(ctx)
	defer cancel()
	state := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: Namespace, Name: quota.LedgerStateConfigMapName}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.reader, state, func() error {
		if !r.mayAct() {
			return errNotActing
		}
		if state.Data == nil {
			state.Data = map[string]string{}
		}
		state.Data[quota.LedgerStateNotCalledKey] = strings.Join(notCalled, ",")
		return nil
	})
	if err != nil {
		return fmt.Errorf("write the %s ConfigMap: %w", quota.LedgerStateConfigMapName, err)
	}
	if len(notCalled) > 0 {
		r.log.Info("API servers admit objects without calling the webhook of the ledger", "servers", notCalled)
	}
	return nil
}

// recordSingleProbes keeps when the leaders last stepped down for each single failing API server.
func (r *runner) recordSingleProbes(ctx context.Context, probes map[string]time.Time) error {
	if !r.mayAct() {
		return errNotActing
	}
	value, err := json.Marshal(probes)
	if err != nil {
		return fmt.Errorf("encode the single probes: %w", err)
	}
	ctx, cancel := r.withinFence(ctx)
	defer cancel()
	state := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: Namespace, Name: quota.LedgerStateConfigMapName}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.reader, state, func() error {
		if !r.mayAct() {
			return errNotActing
		}
		if state.Data == nil {
			state.Data = map[string]string{}
		}
		state.Data[quota.LedgerStateSingleProbesKey] = string(value)
		return nil
	}); err != nil {
		return fmt.Errorf("write the %s ConfigMap: %w", quota.LedgerStateConfigMapName, err)
	}
	return nil
}

// loadSingleProbes reads when the leaders last stepped down for each single failing API server. A
// value that cannot be read counts as none.
func (r *runner) loadSingleProbes(ctx context.Context) (map[string]time.Time, error) {
	state := new(corev1.ConfigMap)
	switch err := r.reader.Get(ctx, client.ObjectKey{Namespace: Namespace, Name: quota.LedgerStateConfigMapName}, state); {
	case apierrors.IsNotFound(err):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("get the %s ConfigMap: %w", quota.LedgerStateConfigMapName, err)
	}
	probes := map[string]time.Time{}
	if value := state.Data[quota.LedgerStateSingleProbesKey]; value != "" {
		if err := json.Unmarshal([]byte(value), &probes); err != nil {
			r.log.Error(err, "read the single probes of the former leaders; taking none")
			return map[string]time.Time{}, nil
		}
	}
	return probes, nil
}

// loadNotCalled reads the API servers that the canary of a former leadership found not calling the
// webhook.
func (r *runner) loadNotCalled(ctx context.Context) ([]string, error) {
	state := new(corev1.ConfigMap)
	switch err := r.reader.Get(ctx, client.ObjectKey{Namespace: Namespace, Name: quota.LedgerStateConfigMapName}, state); {
	case apierrors.IsNotFound(err):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("get the %s ConfigMap: %w", quota.LedgerStateConfigMapName, err)
	}
	if notCalled := state.Data[quota.LedgerStateNotCalledKey]; notCalled != "" {
		return strings.Split(notCalled, ","), nil
	}
	return nil, nil
}

// wake touches the quota of the namespaces refused lately in the projects that freed some of their
// quota, so that whatever waits on the events of those quotas tries again.
func (r *runner) wake(ctx context.Context) {
	now := time.Now()
	for _, namespace := range r.state.TakeWakes(now) {
		rq := &corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1alpha3.ProjectQuotaName}}
		patch := []byte(fmt.Sprintf(`{"metadata":{"annotations":{%q:%q}}}`, WakeAnnotation, now.UTC().Format(time.RFC3339)))
		if err := r.reader.Patch(ctx, rq, client.RawPatch(types.MergePatchType, patch)); err != nil {
			if !apierrors.IsNotFound(err) {
				r.log.Error(err, "wake the quota of a namespace", "namespace", namespace)
			}
			continue
		}
		r.metrics.WakeWrites.Inc()
	}
}

// audit compares the usage the ledger holds with a quorum read.
func (r *runner) audit(ctx context.Context) {
	list := new(corev1.ResourceQuotaList)
	if err := r.reader.List(ctx, list, client.MatchingFields{"metadata.name": v1alpha3.ProjectQuotaName}); err != nil {
		r.log.Error(err, "list the quotas for the drift audit")
		return
	}
	drift := r.state.Drift(list.Items)
	r.metrics.Drift.Set(float64(drift))
	if drift > 0 {
		r.log.Error(errors.New("the ledger differs from the API server"), "drift audit", "quotas", drift)
	}
}

// apiServers returns the addresses of the API servers from the endpoints of the kubernetes Service.
func (r *runner) apiServers(ctx context.Context) ([]string, error) {
	list := new(discoveryv1.EndpointSliceList)
	if err := r.reader.List(ctx, list, client.InNamespace("default"), client.MatchingLabels{discoveryv1.LabelServiceName: "kubernetes"}); err != nil {
		return nil, fmt.Errorf("list the endpoints of the kubernetes Service: %w", err)
	}
	var out []string
	for _, slice := range list.Items {
		port := int32(443)
		for _, p := range slice.Ports {
			if p.Port != nil && (p.Name == nil || *p.Name == "https") {
				port = *p.Port
			}
		}
		for _, endpoint := range slice.Endpoints {
			for _, address := range endpoint.Addresses {
				out = append(out, net.JoinHostPort(address, fmt.Sprint(port)))
			}
		}
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

// peerReady asks the other replicas whether one of them has its informers synced and would take the
// lease now.
func (r *runner) peerReady(ctx context.Context) bool {
	pods := new(corev1.PodList)
	if err := r.reader.List(ctx, pods, client.InNamespace(Namespace), client.MatchingLabels{"app": "multitenancy-manager"}); err != nil {
		r.log.Error(err, "list the replicas of the ledger")
		return false
	}
	httpClient := &http.Client{Timeout: 500 * time.Millisecond}
	for _, pod := range pods.Items {
		if pod.Name == r.opts.Self.PodName || pod.Status.PodIP == "" {
			continue
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+net.JoinHostPort(pod.Status.PodIP, "9445")+"/ledger-health", nil)
		if err != nil {
			continue
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			continue
		}
		var health struct {
			Ready      bool `json:"ready"`
			HoldingOff bool `json:"holdingOff"`
		}
		err = json.NewDecoder(resp.Body).Decode(&health)
		resp.Body.Close()
		// a replica that stepped down itself a moment ago does not take the lease until its hold-off ends
		if err == nil && health.Ready && !health.HoldingOff {
			return true
		}
	}
	return false
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
	// the drift is the finding of the leader; a former one does not keep pointing at itself
	r.metrics.Drift.Set(0)
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
	for r.state.PendingCount()+r.state.ObjectPendingCount() > 0 && time.Since(start) < MinFenceWindow {
		r.resolve(ctx, time.Now().Add(-drainAge), false)
		r.resolveObjects(ctx, time.Now().Add(-drainAge), false)
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

// routes are the webhooks the ledger answers.
func (r *runner) routes() *http.ServeMux {
	handler := &Handler{State: r.state, Gate: r.gate, Metrics: r.metrics, Nonces: r.nonces, Suspended: r.suspended.Load}
	mux := http.NewServeMux()
	mux.Handle("/validate/quota-ledger", handler)
	// The webhook of the virtual machines is there as soon as the module that serves them is enabled,
	// before the ledger counts them: until it does, it answers that it cannot count them yet rather than
	// with a missing path.
	if r.opts.countsVirtualMachines() || r.awaitsVirtualMachines {
		gate := r.gate
		if r.awaitsVirtualMachines {
			gate = func() (string, string) { return ResultSyncing, "the virtual machines are not counted yet" }
		}
		mux.Handle("/validate/quota-vm", &ObjectHandler{Family: FamilyVM, Evaluator: evaluator.VirtualMachine{}, State: r.state, Gate: gate,
			Metrics: r.metrics, Suspended: r.suspended.Load, Sources: &Handler{}})
	}
	return mux
}

// resolveObjects settles the approvals of objects older than the time with a quorum read of the
// object: the write did not go in, or the informer is late. Without settle only the approvals the
// read shows written are settled.
func (r *runner) resolveObjects(ctx context.Context, before time.Time, settle bool) {
	aged := r.state.AgedObjects(before)
	if !settle {
		// no approval is old enough to be taken as failed
		before = time.Time{}
	}
	for _, aged := range aged {
		if aged.Family != FamilyVM {
			continue
		}
		vm := newVirtualMachine()
		err := r.reader.Get(ctx, client.ObjectKey{Namespace: aged.Namespace, Name: aged.Name}, vm)
		switch {
		case apierrors.IsNotFound(err):
			r.state.ResolveObject(aged.Family, aged.Namespace, aged.Name, before, false, 0, nil)
			continue
		case err != nil:
			r.log.Error(err, "read the object of a pending approval", "namespace", aged.Namespace, "name", aged.Name)
			continue
		}
		usage, err := evaluator.VirtualMachine{}.Usage(vm)
		if err != nil {
			usage = corev1.ResourceList{}
		}
		r.state.ResolveObject(aged.Family, aged.Namespace, aged.Name, before, true, ParseResourceVersion(vm.GetResourceVersion()), usage)
		r.metrics.PendingResolved.WithLabelValues("object").Inc()
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
	if r.opts.countsVirtualMachines() {
		kinds = append(kinds, struct {
			list    client.ObjectList
			cached  client.ObjectList
			options []client.ListOption
		}{newVirtualMachineMetadataList(), newVirtualMachineList(), nil})
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
		if _, ok := kind.list.(*metav1.PartialObjectMetadataList); ok {
			// the items of a list of metadata come without the kind the list names
			for _, object := range objects {
				if item, ok := object.(*metav1.PartialObjectMetadata); ok {
					item.SetGroupVersionKind(virtualMachineGVK)
				}
			}
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
		r.metrics.Pending.Set(float64(r.state.PendingCount() + r.state.ObjectPendingCount()))
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
		state := map[string]bool{"leader": r.leader.Load(), "synced": r.synced.Load(), "ready": r.ready.Load(), "holdingOff": r.holdingOff()}
		if err := json.NewEncoder(w).Encode(state); err != nil {
			return
		}
	})
	return &http.Server{Addr: r.opts.HealthAddress, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
}
