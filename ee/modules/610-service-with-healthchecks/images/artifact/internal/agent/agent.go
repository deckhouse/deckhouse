/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package agent

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/deckhouse/deckhouse/pkg/log"

	networkv1alpha1 "service-with-healthchecks/api/v1alpha1"
	"service-with-healthchecks/internal/kubernetes"
)

const (
	endpointServiceNameLabelKey = "kubernetes.io/service-name"
	endpointControllerLabelKey  = "endpointslice.kubernetes.io/managed-by"
	controllerName              = "servicewithhealthchecks"

	// heritageLabelKey marks every object the module manages as belonging to Deckhouse, the same
	// way the rest of the platform labels its own resources.
	heritageLabelKey   = "heritage"
	heritageLabelValue = "deckhouse"

	// resyncPeriod is a periodic, per-object re-reconcile of every ServiceWithHealthchecks.
	// Pod membership already converges from pod watch events — creations, deletions and
	// endpoint-affecting updates all enqueue the owning resource — so this is not the main
	// path. It is a backstop for the drift those events do not cover:
	//   - the child Service, which mayPublishEPS reads but the agent does not watch, so an
	//     ownership clash appearing or clearing is only noticed on the next reconcile;
	//   - the EndpointSlice this node publishes, which is not watched either, so an
	//     out-of-band edit or deletion is repaired on the next reconcile;
	//   - a dropped watch enqueue, capping the worst case at this period instead of the
	//     cache resync (10h by default).
	// It cannot repair a stale informer cache, because the reconcile reads from that same
	// cache. It is read-only in the steady state: neither the status nor the EndpointSlice
	// is written when nothing changed.
	resyncPeriod = time.Minute
)

// ServiceWithHealthchecksReconciler reconciles a ServiceWithHealthchecks object
type ServiceWithHealthchecksReconciler struct {
	workersCount  int
	nodeName      string
	verboseStatus bool
	mu            sync.RWMutex
	client.Client
	Scheme                                       *runtime.Scheme
	logger                                       *log.Logger
	taskQueue                                    *TaskQueue
	tasksResults                                 chan ProbeResult
	events                                       chan event.GenericEvent
	cancelFunc                                   context.CancelFunc
	servicesWithHealthchecks                     sync.Map
	healthchecksResultsByServiceWithHealthchecks map[types.NamespacedName][]HealthcheckTarget
	secretController                             *PostgreSQLCredentialsReconciler
}

func NewServiceWithHealthchecksReconciler(client client.Client, workersCount int, nodeName string, verboseStatus bool, scheme *runtime.Scheme, logger *log.Logger, secretController *PostgreSQLCredentialsReconciler) *ServiceWithHealthchecksReconciler {
	return &ServiceWithHealthchecksReconciler{
		workersCount:  workersCount,
		nodeName:      nodeName,
		verboseStatus: verboseStatus,
		Client:        client,
		Scheme:        scheme,
		logger:        logger,
		taskQueue:     NewTaskQueue(),
		tasksResults:  make(chan ProbeResult, workersCount*10),
		events:        make(chan event.GenericEvent),
		healthchecksResultsByServiceWithHealthchecks: make(map[types.NamespacedName][]HealthcheckTarget),
		secretController: secretController,
	}
}

// +kubebuilder:rbac:groups=network.deckhouse.io,resources=servicewithhealthchecks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.deckhouse.io,resources=servicewithhealthchecks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=network.deckhouse.io,resources=servicewithhealthchecks/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ServiceWithHealthchecksReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		serviceWithHC networkv1alpha1.ServiceWithHealthchecks
		podList       corev1.PodList
		err           error
	)
	r.logger.Debug("reconciling ServiceWithHealthchecks", "name", req.Name, "namespace", req.Namespace)
	if err = r.Get(ctx, req.NamespacedName, &serviceWithHC); err != nil {
		if errors.IsNotFound(err) {
			r.logger.Debug("ServiceWithHealthchecks not found, assuming deleted", "name", req.NamespacedName)
			r.deleteServiceWithHealthchecks(req.NamespacedName)
			return ctrl.Result{}, nil
		}
		r.logger.Error("unable to fetch ServiceWithHealthchecks", log.Err(err))
		return ctrl.Result{}, err
	}

	// Delete helthchecks from internal map because ServiceWithHealthchecks was deleted
	if serviceWithHC.DeletionTimestamp != nil {
		r.logger.Debug("ServiceWithHealthchecks is being deleted")
		r.deleteServiceWithHealthchecks(req.NamespacedName)
		return ctrl.Result{}, nil
	}

	// Select only pods in target namespace, with specified label and on current node
	if err = r.List(ctx, &podList, client.InNamespace(serviceWithHC.GetNamespace()), client.MatchingLabels(serviceWithHC.Spec.Selector), client.MatchingFields{"spec.nodeName": r.nodeName}); err != nil {
		return ctrl.Result{}, err
	}

	// Create internal value with spec
	value, ok := r.servicesWithHealthchecks.Load(req.NamespacedName)
	if !ok || !reflect.DeepEqual(value.(networkv1alpha1.ServiceWithHealthchecksSpec), serviceWithHC.Spec) {
		r.servicesWithHealthchecks.Store(req.NamespacedName, serviceWithHC.Spec)
	}

	// sync internal probes targets with existing pods
	r.syncResultsMapWithPodList(serviceWithHC, podList)

	// update endpointslices unless ClusterIP is None
	if serviceWithHC.Spec.ClusterIP != "None" {
		mayPublish, mayErr := r.mayPublishEPS(ctx, serviceWithHC)
		if mayErr != nil {
			r.logger.Error("unable to check the owner of the child Service", log.Err(mayErr))
			return ctrl.Result{}, mayErr
		}

		if mayPublish {
			err = r.updateEPSForServiceWithHealthchecks(ctx, serviceWithHC)
		} else {
			// Withdraw what this node published before the clash appeared. Doing it here rather
			// than from the controller keeps the two components from undoing each other while
			// they are rolled out one after another.
			err = r.deleteEPSForNode(ctx, serviceWithHC)
		}
		if err != nil {
			r.logger.Error("unable to update EPS for ServiceWithHealthchecks", log.Err(err))
			return ctrl.Result{}, err
		}
	}

	// update status
	updatedServiceWithHC := serviceWithHC.DeepCopy()
	patch := client.MergeFrom(&serviceWithHC)

	newStatus := r.buildRenewedStatus(updatedServiceWithHC)
	updatedServiceWithHC.Status.HealthcheckCondition = newStatus.HealthcheckCondition
	updatedServiceWithHC.Status.EndpointStatuses = newStatus.EndpointStatuses
	updatedServiceWithHC.Status.Conditions = kubernetes.UpdateStatusWithConditions(updatedServiceWithHC.Status.Conditions, newStatus.Conditions)

	sortEndpointStatuses(updatedServiceWithHC.Status.EndpointStatuses)
	sortEndpointStatuses(serviceWithHC.Status.EndpointStatuses)

	kubernetes.SortConditions(updatedServiceWithHC.Status.Conditions)
	kubernetes.SortConditions(serviceWithHC.Status.Conditions)

	if reflect.DeepEqual(serviceWithHC.Status, updatedServiceWithHC.Status) {
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	err = r.Status().Patch(ctx, updatedServiceWithHC, patch)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to patch status of ServiceWithHealthchecks: %w", err)
	}
	return ctrl.Result{RequeueAfter: resyncPeriod}, nil
}

// SetupWithManager registers the pod field indexer and a startup Runnable that prefills the
// in-memory cache and only then registers the controller. The controller — and therefore its pod
// watch and mapper — must not exist before the cache is populated, or the first pod events would
// be mapped against an empty cache and dropped; see cachePrefillRunnable for why an ordinary
// Runnable cannot enforce that ordering.
//
// The field indexer is registered here, before mgr.Start, because an index has to be added before
// its informer starts; the controller is deferred, but the index is not.
func (r *ServiceWithHealthchecksReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &corev1.Pod{}, "spec.nodeName", func(rawObj client.Object) []string {
		pod := rawObj.(*corev1.Pod)
		return []string{pod.Spec.NodeName}
	}); err != nil {
		return err
	}
	return mgr.Add(&cachePrefillRunnable{reconciler: r, mgr: mgr})
}

// registerController wires the ServiceWithHealthchecks controller into the manager. It is called
// from cachePrefillRunnable after the cache prefill has completed, rather than from
// SetupWithManager, so the pod watch starts only once the in-memory cache holds every
// ServiceWithHealthchecks spec. The manager is already running by then, which is supported:
// manager.Add on a started manager enqueues the controller into its runnable group and starts it.
func (r *ServiceWithHealthchecksReconciler) registerController(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 4,
		}).
		For(&networkv1alpha1.ServiceWithHealthchecks{}).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(r.getExposedServiceWithHCForPod),
			// Only endpoint-affecting pod changes should wake a reconcile. Without this, every
			// status-subresource heartbeat or annotation edit of any node-local pod enqueues a full
			// reconcile, and under pod churn that noise competes with the meaningful readiness change
			// for the reconcile workers and delays convergence.
			builder.WithPredicates(podEndpointRelevantPredicate()),
		).
		WatchesRawSource(source.Channel(r.events, &handler.EnqueueRequestForObject{})).
		Complete(r)
}

// cachePrefillRunnable prefills the in-memory ServiceWithHealthchecks cache and then registers the
// controller, in that order — which is the whole point of it. The controller cannot be registered
// in SetupWithManager with the prefill as a separate Runnable, because a manager Runnable's body is
// not awaited before the controllers start: the runnable-group readiness check runs in its own
// goroutine and, for an ordinary Runnable, is trivially true at once, so the group's Start returns
// while the body is still running. That holds on every controller-runtime version, the warmup
// group included. By registering the controller only after PrefillServiceCache returns, the pod
// watch is guaranteed to see a populated cache from its very first event.
//
// It reports that it does not need leader election so the manager runs it in the Others group,
// which starts after the caches have synced — so PrefillServiceCache can read from the cache — and
// ahead of the leader-election group the controller lands in.
type cachePrefillRunnable struct {
	reconciler *ServiceWithHealthchecksReconciler
	mgr        ctrl.Manager
}

func (c *cachePrefillRunnable) Start(ctx context.Context) error {
	if err := c.reconciler.PrefillServiceCache(ctx); err != nil {
		// Non-fatal: the controller still gets registered below, and each resource's initial
		// reconcile fills the cache. Only the head start is lost, so log and carry on.
		c.reconciler.logger.Error("prefilling ServiceWithHealthchecks cache failed; registering controller anyway", log.Err(err))
	}
	return c.reconciler.registerController(c.mgr)
}

func (c *cachePrefillRunnable) NeedLeaderElection() bool { return false }

// PrefillServiceCache loads the in-memory ServiceWithHealthchecks specs from the manager cache
// once, at startup, so the pod watch mapper (getExposedServiceWithHCForPod) can match a node-local
// pod to its owning resource from the first event. The mapper reads only this map, which is
// otherwise filled by each resource's own reconcile, so without the prefill a pod event arriving
// before that reconcile would be mapped against an empty cache and dropped.
//
// It is called from cachePrefillRunnable before the controller is registered, so its result is in
// place before any pod event can be processed. Correctness does not hinge on that ordering — every
// resource is reconciled once on start, which fills the map and performs a full pod sync — so a
// failure here only costs the head start and is returned for logging, not treated as fatal.
//
// It stores exactly what Reconcile stores: the spec keyed by NamespacedName, skipping a resource
// being deleted, so the value type the mapper and the task scheduler assert on stays consistent.
// Reads go through the cached client; the list lazily starts and syncs the ServiceWithHealthchecks
// informer, so it reflects the same cache the controller reconciles from.
func (r *ServiceWithHealthchecksReconciler) PrefillServiceCache(ctx context.Context) error {
	var list networkv1alpha1.ServiceWithHealthchecksList
	if err := r.List(ctx, &list); err != nil {
		return fmt.Errorf("listing ServiceWithHealthchecks for cache prefill: %w", err)
	}

	count := 0
	for i := range list.Items {
		swh := &list.Items[i]
		if swh.DeletionTimestamp != nil {
			// Reconcile does not store a resource that is being deleted; mirror that here so a
			// resource mid-deletion is not resurrected in the map until a live event arrives.
			continue
		}
		r.servicesWithHealthchecks.Store(types.NamespacedName{Namespace: swh.GetNamespace(), Name: swh.GetName()}, swh.Spec)
		count++
	}

	r.logger.Info("prefilled ServiceWithHealthchecks cache", "count", count)
	return nil
}

func (r *ServiceWithHealthchecksReconciler) buildEndpointStatuses(svc *networkv1alpha1.ServiceWithHealthchecks) []networkv1alpha1.EndpointStatus {
	var endpointStatuses []networkv1alpha1.EndpointStatus
	r.mu.RLock()
	defer r.mu.RUnlock()

	// save old statuses to preserve LastTransitionTime
	oldStatusesMap := make(map[string]networkv1alpha1.EndpointStatus)
	for _, status := range svc.Status.EndpointStatuses {
		if status.NodeName == r.nodeName {
			oldStatusesMap[status.PodName] = status
		}
	}

	// keep statuses from other nodes (build a new slice instead of mutating the input)
	for _, endpointStatus := range svc.Status.EndpointStatuses {
		if endpointStatus.NodeName != r.nodeName {
			endpointStatuses = append(endpointStatuses, endpointStatus)
		}
	}

	// add new healthchecks probes results
	for _, result := range r.healthchecksResultsByServiceWithHealthchecks[types.NamespacedName{Name: svc.GetName(), Namespace: svc.GetNamespace()}] {
		probesSuccessful := true
		var failedProbes []string

		// Probes matter only when the resource actually runs some. With PublishNotReadyAddresses,
		// or with no effective probes (none configured, or all targeting UDP ports), a target is
		// always considered probe-successful and its readiness is decided by the pod alone.
		if !svc.Spec.PublishNotReadyAddresses && hasEffectiveProbes(svc.Spec) {
			probesSuccessful = *areAllProbesSucceed(result.probeResultDetails)
			failedProbes = result.FailedProbes()
		}

		// An endpoint is ready when the pod is ready and its probes pass, which is the same
		// condition the resource aggregates into ReadyEndpoints. Reporting pod readiness alone here
		// made the status read as a contradiction: ready: true next to a non-empty failedProbes and
		// a resource-level "Not all endpoints are ready". probesSuccessful still isolates the probe
		// half, so ready: false with probesSuccessful: true means the pod itself is not ready.
		ready := result.podReady && probesSuccessful

		lastTransitionTime := metav1.Now()
		lastProbeTime := metav1.Time{}

		if oldStatus, ok := oldStatusesMap[result.podName]; ok {
			// If state didn't change, preserve old transition time.
			failedProbesEqual := len(oldStatus.FailedProbes) == 0 && len(failedProbes) == 0 ||
				reflect.DeepEqual(oldStatus.FailedProbes, failedProbes)
			stateChanged := oldStatus.ProbesSuccessful != probesSuccessful ||
				!failedProbesEqual ||
				oldStatus.Ready != ready
			if !stateChanged {
				lastTransitionTime = oldStatus.LastTransitionTime
			}

			if !r.verboseStatus {
				lastProbeTime = oldStatus.LastProbeTime
			}
		}

		if r.verboseStatus {
			lastProbeTime = metav1.Time{Time: result.lastCheck}
		}

		endpointStatuses = append(endpointStatuses, networkv1alpha1.EndpointStatus{
			PodName:            result.podName,
			NodeName:           r.nodeName,
			Ready:              ready,
			ProbesSuccessful:   probesSuccessful,
			FailedProbes:       failedProbes,
			LastTransitionTime: lastTransitionTime,
			LastProbeTime:      lastProbeTime,
		})
	}
	return endpointStatuses
}

func (r *ServiceWithHealthchecksReconciler) getExposedServiceWithHCForPod(ctx context.Context, object client.Object) []reconcile.Request {
	requests := []reconcile.Request{}

	pod, ok := object.(*corev1.Pod)
	if !ok || pod.Spec.NodeName != r.nodeName {
		return requests // it is not a pod or pod is on different node
	}

	// iterate over saved services specifications and check if it matches pod labels
	r.servicesWithHealthchecks.Range(func(key, value any) bool {
		svcWithHCName := key.(types.NamespacedName)

		// A ServiceWithHealthchecks only ever selects pods of its own namespace, see the List
		// call in Reconcile. Checked before the spec is read out of the interface, so that an
		// entry of another namespace costs a string comparison instead of a struct copy.
		if svcWithHCName.Namespace != pod.GetNamespace() {
			return true
		}

		svcWithHCSpec := value.(networkv1alpha1.ServiceWithHealthchecksSpec)
		if labels.ValidatedSetSelector(svcWithHCSpec.Selector).Matches(labels.Set(pod.GetLabels())) {
			requests = append(requests, reconcile.Request{NamespacedName: svcWithHCName})
		}

		// Every stored ServiceWithHealthchecks has to be examined, so the iteration is never
		// stopped: sync.Map.Range treats a false return as "stop", and stopping at the first
		// non-matching entry silently drops the event for all the entries behind it. Range
		// order is randomized, so a pod of the Nth ServiceWithHealthchecks would only be
		// noticed when it happens to be visited first — and a missed pod creation leaves the
		// EndpointSlice empty until something else triggers a reconciliation.
		return true
	})
	return requests
}

// podEndpointRelevantPredicate keeps the pod watch reacting to creations and deletions, and to updates
// that can change the endpoints this agent publishes. Updates that touch nothing relevant (heartbeats,
// unrelated status-subresource or annotation churn) are dropped before they reach the mapper, so the
// reconcile workers are free to process a real readiness change without queueing behind the noise.
func podEndpointRelevantPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return true },
		DeleteFunc:  func(event.DeleteEvent) bool { return true },
		GenericFunc: func(event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldPod, oldOK := e.ObjectOld.(*corev1.Pod)
			newPod, newOK := e.ObjectNew.(*corev1.Pod)
			if !oldOK || !newOK {
				return true // unexpected type: do not drop the event
			}
			return podEndpointStateChanged(oldPod, newPod)
		},
	}
}

// podEndpointStateChanged reports whether a pod update touched a field that can change the endpoints the
// agent publishes: its IP or phase (both gate whether the pod is tracked at all), its readiness, its
// terminating state, or its labels (which decide selector membership).
func podEndpointStateChanged(oldPod, newPod *corev1.Pod) bool {
	switch {
	case oldPod.Status.PodIP != newPod.Status.PodIP:
		return true
	case oldPod.Status.Phase != newPod.Status.Phase:
		return true
	case isPodReady(oldPod) != isPodReady(newPod):
		return true
	case isPodTerminating(oldPod) != isPodTerminating(newPod):
		return true
	case !reflect.DeepEqual(oldPod.Labels, newPod.Labels):
		return true
	default:
		return false
	}
}

func (r *ServiceWithHealthchecksReconciler) RunWorkers(ctx context.Context) error {
	r.logger.Debug("starting workers", "workers_count", r.workersCount)

	ctx, cancel := context.WithCancel(ctx)
	r.cancelFunc = cancel
	// run function to make tasks for worker (fan-out)
	go r.RunTasksScheduler(ctx)

	// run workers
	for i := 0; i < r.workersCount; i++ {
		go r.RunTaskWorker(ctx)
	}

	go r.RunTaskResultsAnalyzer(ctx)
	return nil
}

func (r *ServiceWithHealthchecksReconciler) Shutdown() {
	r.cancelFunc()
	close(r.tasksResults)
	close(r.events)
}

func (r *ServiceWithHealthchecksReconciler) RunTasksScheduler(ctx context.Context) {
	r.logger.Info("making tasks")
	ticker := time.NewTicker(time.Millisecond * 500)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// write task to channel
			r.mu.RLock()
			for swhName := range r.healthchecksResultsByServiceWithHealthchecks {
				for i := range r.healthchecksResultsByServiceWithHealthchecks[swhName] {
					healthcheckTarget := r.healthchecksResultsByServiceWithHealthchecks[swhName][i]
					if !healthcheckTarget.podReady && !healthcheckTarget.podTerminating {
						// skip pods which are neither ready nor shutting down gracefully
						continue
					}
					value, ok := r.servicesWithHealthchecks.Load(swhName)
					if !ok {
						continue // can not receive stored ServiceWithHealthchecks specification
					}
					swhSpec, ok := value.(networkv1alpha1.ServiceWithHealthchecksSpec)
					if !ok {
						continue // can not receive stored ServiceWithHealthchecks specification
					}

					if swhSpec.PublishNotReadyAddresses || swhSpec.ClusterIP == "None" {
						continue // not need to check connections probe to pod, they are always successful
					}

					if !hasEffectiveProbes(swhSpec) {
						continue // no probes to run; publishing follows pod readiness only
					}

					now := time.Now()
					diff := now.Sub(healthcheckTarget.creationTime).Seconds()
					if diff < float64(swhSpec.Healthcheck.InitialDelaySeconds) {
						continue // skip task while initial delay
					}
					diff = now.Sub(healthcheckTarget.lastCheck).Seconds()
					if diff < float64(swhSpec.Healthcheck.PeriodSeconds) {
						continue // skip task while period elapsed
					}

					probes := r.getProbesFromServiceWithHealthchecks(swhSpec, healthcheckTarget.targetHost, healthcheckTarget.podNamespace)
					r.taskQueue.Enqueue(&ProbeTask{
						host:     healthcheckTarget.targetHost,
						swhName:  swhName,
						probes:   healthcheckTarget.GetRenewedProbes(probes),
						previous: healthcheckTarget.GetProbeResultDetailsMap(),
					})
				}
			}
			r.mu.RUnlock()
		case <-ctx.Done():
			return
		}
	}
}

func (r *ServiceWithHealthchecksReconciler) RunTaskResultsAnalyzer(ctx context.Context) {
	r.logger.Info("analyzing results")
	for result := range r.tasksResults {
		r.mu.Lock()
		if _, exists := r.healthchecksResultsByServiceWithHealthchecks[result.swhName]; !exists {
			r.logger.Info("Could not update probes result for ServiceWithHealthchecks - ServiceWithHealthchecks is not found", "name", result.swhName.String())
			r.mu.Unlock()
			continue
		}

		for i, target := range r.healthchecksResultsByServiceWithHealthchecks[result.swhName] {
			if target.targetHost == result.host {
				r.healthchecksResultsByServiceWithHealthchecks[result.swhName][i].lastCheck = time.Now()
				r.healthchecksResultsByServiceWithHealthchecks[result.swhName][i].probeResultDetails = result.probeDetails
				// generate event for watcher
				r.events <- event.GenericEvent{Object: &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: result.swhName.Name, Namespace: result.swhName.Namespace}}}
			}
		}

		r.mu.Unlock()
	}
}

func (r *ServiceWithHealthchecksReconciler) RunTaskWorker(ctx context.Context) {
	r.logger.Debug("running task")
	for {
		task := r.taskQueue.Dequeue()
		r.logger.Debug("running task", "host", task.host, "swh_name", task.swhName.String())
		g, _ := errgroup.WithContext(ctx)
		probesResultDetails := make([]ProbeResultDetail, len(task.probes))
		for i, probe := range task.probes {
			g.Go(func() error {
				err := probe.PerformCheck()
				successCount, failureCount := calculateCounts(err, probe.SuccessCount(), probe.FailureCount())
				successful := probeSuccessful(
					task.previous[probe.GetID()].successful,
					successCount, failureCount,
					probe.SuccessThreshold(), probe.FailureThreshold(),
				)
				probesResultDetails[i] = ProbeResultDetail{
					id:               probe.GetID(),
					successful:       successful,
					mode:             probe.GetMode(),
					targetPort:       probe.GetPort(),
					successCount:     successCount,
					failureCount:     failureCount,
					successThreshold: probe.SuccessThreshold(),
					failureThreshold: probe.FailureThreshold(),
				}
				return err
			})
		}
		err := g.Wait()
		if err != nil {
			r.logger.Debug("error performing probes", "error", err.Error(), "host", task.host, "swh_name", task.swhName.String())
		}
		r.tasksResults <- ProbeResult{
			host:         task.host,
			swhName:      task.swhName,
			probeDetails: probesResultDetails,
			successful:   err == nil,
		}
	}
}

// probeSuccessful applies the threshold semantics the CRD documents: a probe is considered failed
// only after failureThreshold consecutive failures following a success, and successful again only
// after successThreshold consecutive successes following a failure. In between it keeps the state
// it already had.
//
// Recomputing the state from the counters alone, as this used to do, silently disabled
// failureThreshold: the state defaulted to "failed" on every run, so a single failed check withdrew
// the endpoint no matter how high the threshold was set. The zero state is still "failed", which is
// what makes a target wait for successThreshold checks before it is published for the first time.
func probeSuccessful(previous bool, successCount, failureCount, successThreshold, failureThreshold int32) bool {
	switch {
	case successCount > 0 && successCount >= successThreshold:
		return true
	case failureCount > 0 && failureCount >= failureThreshold:
		return false
	default:
		return previous
	}
}

func calculateCounts(err error, successCount int32, failureCount int32) (int32, int32) {
	if err != nil {
		failureCount++
		successCount = 0
	} else {
		failureCount = 0
		successCount++
	}
	return successCount, failureCount
}

func (r *ServiceWithHealthchecksReconciler) GetNodeName() string {
	return r.nodeName
}

// mayPublishEPS reports whether the module may publish EndpointSlices under the name of this
// resource. A Service the module does not control keeps its own selector and its own endpoints,
// and kube-proxy balances over the union of every slice carrying the service name — so publishing
// next to it would send traffic meant for somebody else's Service into the pods of this resource.
//
// The child Service is read from the cache of the manager. The agent does not watch Services, so a
// Service appearing under this name is noticed on the next reconciliation rather than immediately;
// the resync above bounds that window.
func (r *ServiceWithHealthchecksReconciler) mayPublishEPS(ctx context.Context, svc networkv1alpha1.ServiceWithHealthchecks) (bool, error) {
	var childService corev1.Service
	err := r.Get(ctx, client.ObjectKey{Namespace: svc.GetNamespace(), Name: svc.GetName()}, &childService)
	if errors.IsNotFound(err) {
		// The controller has not created it yet. Slices are matched to a Service by name, so
		// publishing ahead of it changes nothing until the Service shows up.
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return kubernetes.IsOwnedByServiceWithHealthchecks(&childService, svc.GetName()), nil
}

// deleteEPSForNode removes the EndpointSlice this node maintains for the resource, if any.
//
// The slice is read from the cache first, for two reasons. A name clash means somebody else's
// objects are around, and a slice that is not ours is not ours to delete. And in the steady state
// there is nothing to delete at all — for a resource with no pods on this node, or one stuck in a
// conflict — so without the lookup every node would issue a DELETE on every resync.
func (r *ServiceWithHealthchecksReconciler) deleteEPSForNode(ctx context.Context, svc networkv1alpha1.ServiceWithHealthchecks) error {
	name := endpointSliceNameForNode(svc.GetName(), r.nodeName)

	var existing discoveryv1.EndpointSlice
	err := r.Get(ctx, client.ObjectKey{Namespace: svc.GetNamespace(), Name: name}, &existing)
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		r.logger.Error("could not get EndpointSlice", log.Err(err), "name", name)
		return err
	}
	if existing.Labels[endpointControllerLabelKey] != controllerName {
		r.logger.Info("leaving an EndpointSlice of another controller alone", "name", name,
			"managed_by", existing.Labels[endpointControllerLabelKey])
		return nil
	}

	if err := r.Delete(ctx, &existing); err != nil && !errors.IsNotFound(err) {
		r.logger.Error("could not delete EndpointSlice", log.Err(err), "name", name)
		return err
	}
	return nil
}

func endpointSliceNameForNode(svcName, nodeName string) string {
	return svcName + "-" + nodeName
}

func (r *ServiceWithHealthchecksReconciler) updateEPSForServiceWithHealthchecks(ctx context.Context, svc networkv1alpha1.ServiceWithHealthchecks) error {
	r.logger.Debug("updating endpoints for service", "swh_name", svc.GetName(), "namespace", svc.GetNamespace())
	desiredNameForEndpointSlice := endpointSliceNameForNode(svc.GetName(), r.nodeName)

	// Build the desired state
	desiredEPS := r.BuildEndpointSlice(desiredNameForEndpointSlice, svc)

	// If there are no endpoints, the slice should not exist on this node
	if len(desiredEPS.Endpoints) == 0 {
		// Exit here after deleting (or if already deleted), do not proceed to Get/Update.
		return r.deleteEPSForNode(ctx, svc)
	}

	// Try to get the existing one to see if we need to update it.
	// Use an empty struct to prevent ObjectMeta corruption from unmarshaling into a populated struct.
	existingEPS := &discoveryv1.EndpointSlice{}
	err := r.Get(ctx, client.ObjectKey{Namespace: svc.GetNamespace(), Name: desiredNameForEndpointSlice}, existingEPS)

	if errors.IsNotFound(err) {
		r.logger.Info("creating new EndpointSlice", "name", desiredNameForEndpointSlice)
		if err = r.Create(ctx, &desiredEPS); err != nil {
			r.logger.Error("couldn't create EndpointSlice", log.Err(err), "name", desiredNameForEndpointSlice)
			return err
		}
		return nil
	}
	if err != nil {
		r.logger.Error("couldn't get EndpointSlice", log.Err(err), "name", desiredNameForEndpointSlice)
		return err
	}

	// A slice created before the owner reference was introduced, or left over from a recreated
	// parent, is adopted here instead of being recreated.
	ownerIsOutdated := !reflect.DeepEqual(existingEPS.OwnerReferences, desiredEPS.OwnerReferences)
	// A slice created before the heritage label was introduced is missing it; add it on the next
	// update instead of leaving old slices unlabelled.
	heritageOutdated := existingEPS.Labels[heritageLabelKey] != heritageLabelValue

	// Use Patch instead of Update to avoid conflicts and ResourceVersion issues.
	if ownerIsOutdated || heritageOutdated || !endpointsAreEqual(existingEPS.Endpoints, desiredEPS.Endpoints) {
		patch := client.MergeFrom(existingEPS.DeepCopy())
		existingEPS.Endpoints = desiredEPS.Endpoints
		existingEPS.OwnerReferences = desiredEPS.OwnerReferences
		if existingEPS.Labels == nil {
			existingEPS.Labels = map[string]string{}
		}
		existingEPS.Labels[heritageLabelKey] = heritageLabelValue
		if err := r.Patch(ctx, existingEPS, patch); err != nil {
			r.logger.Error("couldn't patch EndpointSlice", log.Err(err), "name", desiredNameForEndpointSlice)
			return err
		}
	}
	return nil
}

func (r *ServiceWithHealthchecksReconciler) BuildEndpointSlice(desiredName string, svc networkv1alpha1.ServiceWithHealthchecks) discoveryv1.EndpointSlice {
	eps := discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      desiredName,
			Namespace: svc.GetNamespace(),
			Labels: map[string]string{
				endpointServiceNameLabelKey: svc.GetName(),
				endpointControllerLabelKey:  controllerName,
				heritageLabelKey:            heritageLabelValue,
			},
			OwnerReferences: []metav1.OwnerReference{ownerReferenceForServiceWithHealthchecks(svc)},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Ports:       r.buildPortsForEndpointslice(svc),
	}

	eps.Endpoints = r.buildEndpoints(svc)
	return eps
}

// ownerReferenceForServiceWithHealthchecks ties a slice to the resource it was built from, so
// that the garbage collector removes the slices of every node once that resource is gone. The
// child Service is owned by the same resource, so both branches of the tree are cleaned up.
//
// BlockOwnerDeletion is deliberately left unset: with the OwnerReferencesPermissionEnforcement
// admission plugin enabled it would require the agent to have access to the
// servicewithhealthchecks/finalizers subresource, which it has no other reason to hold.
func ownerReferenceForServiceWithHealthchecks(svc networkv1alpha1.ServiceWithHealthchecks) metav1.OwnerReference {
	isController := true
	return metav1.OwnerReference{
		APIVersion: networkv1alpha1.GroupVersion.String(),
		Kind:       kubernetes.ServiceWithHealthchecksKind,
		Name:       svc.GetName(),
		UID:        svc.GetUID(),
		Controller: &isController,
	}
}

func (r *ServiceWithHealthchecksReconciler) buildPortsForEndpointslice(svc networkv1alpha1.ServiceWithHealthchecks) []discoveryv1.EndpointPort {
	ports := make([]discoveryv1.EndpointPort, 0, len(svc.Spec.Ports))
	for _, port := range svc.Spec.Ports {
		portTarget := int32(port.TargetPort.IntValue())
		ports = append(ports, discoveryv1.EndpointPort{
			Name:     &port.Name,
			Port:     &portTarget,
			Protocol: &port.Protocol,
		})
	}
	return ports
}

func (r *ServiceWithHealthchecksReconciler) buildEndpoints(svc networkv1alpha1.ServiceWithHealthchecks) []discoveryv1.Endpoint {
	endpoints := []discoveryv1.Endpoint{}
	r.mu.RLock()
	defer r.mu.RUnlock()

	// With no effective probes (none configured, or all targeting UDP ports) the resource behaves
	// like a plain Service: a target is publishable on pod readiness alone. With probes, it becomes
	// publishable once they pass.
	probesConfigured := hasEffectiveProbes(svc.Spec)

	for _, probeResult := range r.healthchecksResultsByServiceWithHealthchecks[types.NamespacedName{Name: svc.GetName(), Namespace: svc.GetNamespace()}] {
		probesSucceed := !probesConfigured || *areAllProbesSucceed(probeResult.probeResultDetails)
		healthy := probesSucceed
		if !probesConfigured {
			healthy = probeResult.podReady
		}

		// a terminating pod stays published until it disappears, so that consumers may fall
		// back to it while no ready endpoint is left
		if !svc.Spec.PublishNotReadyAddresses && !healthy && !probeResult.podTerminating {
			continue
		}

		// a terminating pod is never ready, but may still serve traffic while shutting down
		serving := probeResult.podReady && probesSucceed
		if probeResult.podTerminating {
			serving = probesSucceed
		}
		ready := serving && !probeResult.podTerminating
		terminating := probeResult.podTerminating

		endpoint := discoveryv1.Endpoint{
			Addresses: []string{probeResult.targetHost},
			NodeName:  &r.nodeName,
			TargetRef: &corev1.ObjectReference{
				Kind:      "Pod",
				Name:      probeResult.podName,
				Namespace: svc.GetNamespace(), UID: probeResult.podUID,
			},
			Conditions: discoveryv1.EndpointConditions{
				Ready:       &ready,
				Serving:     &serving,
				Terminating: &terminating,
			},
		}
		endpoints = append(endpoints, endpoint)
	}
	return endpoints
}

// podShouldBeTracked reports whether the pod may be published as an endpoint. Pods in a
// terminal phase keep their podIP, which in DVP clusters may already be served by another
// pod on another node. Pods being deleted are still published, as terminating endpoints.
func podShouldBeTracked(pod *corev1.Pod) bool {
	if pod.Status.PodIP == "" {
		return false
	}
	return pod.Status.Phase != corev1.PodFailed && pod.Status.Phase != corev1.PodSucceeded
}

func isPodTerminating(pod *corev1.Pod) bool {
	return pod.DeletionTimestamp != nil
}

// isPodReady relies on the PodReady condition instead of container statuses: the latter is
// empty for pods that failed before their containers started, so they would look ready.
func isPodReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

type podState struct {
	ready       bool
	terminating bool
}

// getPodsStateMap returns the state of the pods eligible for publishing. The pods left out
// are absent from the map, so syncResultsMapWithPodList drops them from the targets.
func getPodsStateMap(podList corev1.PodList) map[types.NamespacedName]podState {
	podsStateMap := make(map[types.NamespacedName]podState)
	for i := range podList.Items {
		pod := &podList.Items[i]
		if !podShouldBeTracked(pod) {
			continue
		}
		podsStateMap[types.NamespacedName{Name: pod.GetName(), Namespace: pod.GetNamespace()}] = podState{
			ready:       isPodReady(pod),
			terminating: isPodTerminating(pod),
		}
	}
	return podsStateMap
}

func (r *ServiceWithHealthchecksReconciler) deleteServiceWithHealthchecks(swhName types.NamespacedName) {
	r.servicesWithHealthchecks.Delete(swhName)
	r.mu.Lock()
	delete(r.healthchecksResultsByServiceWithHealthchecks, swhName)
	r.mu.Unlock()
}

func (r *ServiceWithHealthchecksReconciler) getProbesFromServiceWithHealthchecks(svcSpec networkv1alpha1.ServiceWithHealthchecksSpec, targetHost, namespace string) []Prober {
	probes := make([]Prober, 0, len(svcSpec.Healthcheck.Probes))
	udpPorts := udpTargetPorts(svcSpec)
	for _, serviceProbe := range svcSpec.Healthcheck.Probes {
		// UDP cannot be blackbox-probed, so a probe aimed at a UDP port never runs. Skipping it
		// here also keeps it out of hasEffectiveProbes, so such a target is published on readiness.
		if _, isUDP := udpPorts[probeTargetPort(serviceProbe)]; isUDP {
			r.logger.Warn("skipping probe targeting a UDP port; UDP is not suitable for blackbox healthchecking",
				"target_port", probeTargetPort(serviceProbe), "mode", serviceProbe.Mode)
			continue
		}
		switch strings.ToLower(serviceProbe.Mode) {
		case "http":
			probes = append(probes, FastHTTPProbeTarget{
				targetHost:            targetHost,
				host:                  serviceProbe.HTTPHandler.Host,
				path:                  serviceProbe.HTTPHandler.Path,
				targetPort:            serviceProbe.HTTPHandler.TargetPort.IntValue(),
				scheme:                string(serviceProbe.HTTPHandler.Scheme),
				method:                serviceProbe.HTTPHandler.Method,
				httpHeaders:           serviceProbe.HTTPHandler.HTTPHeaders,
				codes:                 serviceProbe.HTTPHandler.Code,
				insecureSkipTLSVerify: serviceProbe.HTTPHandler.InsecureSkipTLSVerify,
				caCert:                serviceProbe.HTTPHandler.CaCert,
				successThreshold:      serviceProbe.SuccessThreshold,
				failureThreshold:      serviceProbe.FailureThreshold,
				timeoutSeconds:        serviceProbe.TimeoutSeconds,
			})
		case "tcp":
			probes = append(probes, TCPProbeTarget{
				targetHost:       targetHost,
				targetPort:       serviceProbe.TCPHandler.TargetPort.IntValue(),
				successThreshold: serviceProbe.SuccessThreshold,
				failureThreshold: serviceProbe.FailureThreshold,
				timeoutSeconds:   serviceProbe.TimeoutSeconds,
			})
		case "postgresql":
			creds, err := r.getPostgreSQLCredentials(serviceProbe.PostgreSQL, namespace)
			if err != nil {
				r.logger.Error("failed to get PostgreSQL credentials", log.Err(err))
				continue
			}
			probes = append(probes, PostgreSQLProbeTarget{
				targetHost:       targetHost,
				targetPort:       serviceProbe.PostgreSQL.TargetPort.IntValue(),
				successThreshold: serviceProbe.SuccessThreshold,
				failureThreshold: serviceProbe.FailureThreshold,
				timeoutSeconds:   serviceProbe.TimeoutSeconds,
				dbName:           serviceProbe.PostgreSQL.DBName,
				query:            serviceProbe.PostgreSQL.Query,
				user:             creds.User,
				password:         creds.Password,
				clientCert:       creds.ClientCert,
				clientKey:        creds.ClientKey,
				caCert:           creds.CaCert,
				tlsMode:          creds.TLSMode,
			})
		}
	}
	return probes
}

func (r *ServiceWithHealthchecksReconciler) getPostgreSQLCredentials(sqlHandler *networkv1alpha1.PGSQLHandler, namespace string) (PostgreSQLCredentials, error) {
	return r.secretController.GetCachedSecret(types.NamespacedName{Namespace: namespace, Name: sqlHandler.AuthSecretName})
}

// udpTargetPorts returns the set of pod ports the resource exposes over UDP. UDP is not suitable for
// blackbox healthchecking, so probes aimed at these ports are skipped when building the probe set and do
// not gate endpoint publishing.
func udpTargetPorts(spec networkv1alpha1.ServiceWithHealthchecksSpec) map[int]struct{} {
	udp := make(map[int]struct{})
	for i := range spec.Ports {
		port := &spec.Ports[i]
		if port.Protocol != corev1.ProtocolUDP {
			continue
		}
		target := port.TargetPort.IntValue()
		if target == 0 {
			// An unset or named targetPort defaults to the service port number.
			target = int(port.Port)
		}
		udp[target] = struct{}{}
	}
	return udp
}

// probeTargetPort returns the numeric pod port a probe connects to, or 0 if it cannot be determined.
func probeTargetPort(probe networkv1alpha1.Probe) int {
	switch strings.ToLower(probe.Mode) {
	case "http":
		if probe.HTTPHandler != nil {
			return probe.HTTPHandler.TargetPort.IntValue()
		}
	case "tcp":
		if probe.TCPHandler != nil {
			return probe.TCPHandler.TargetPort.IntValue()
		}
	case "postgresql":
		if probe.PostgreSQL != nil {
			return probe.PostgreSQL.TargetPort.IntValue()
		}
	}
	return 0
}

// hasEffectiveProbes reports whether the resource has at least one probe that will actually run. A
// resource with no probes at all, or one whose probes all target UDP ports (which are skipped), is
// published on pod readiness alone, like a plain Service.
func hasEffectiveProbes(spec networkv1alpha1.ServiceWithHealthchecksSpec) bool {
	udp := udpTargetPorts(spec)
	for i := range spec.Healthcheck.Probes {
		if _, isUDP := udp[probeTargetPort(spec.Healthcheck.Probes[i])]; !isUDP {
			return true
		}
	}
	return false
}

func (r *ServiceWithHealthchecksReconciler) syncResultsMapWithPodList(hc networkv1alpha1.ServiceWithHealthchecks, podList corev1.PodList) {
	serviceWithHCKey := types.NamespacedName{Namespace: hc.Namespace, Name: hc.Name}
	podsStateMap := getPodsStateMap(podList)
	r.mu.Lock()
	// clean unused pod IPs from result slice
	n := 0
	for _, target := range r.healthchecksResultsByServiceWithHealthchecks[serviceWithHCKey] {
		if _, exists := podsStateMap[types.NamespacedName{Namespace: hc.Namespace, Name: target.podName}]; exists {
			r.healthchecksResultsByServiceWithHealthchecks[serviceWithHCKey][n] = target
			n++
		}
	}
	if len(r.healthchecksResultsByServiceWithHealthchecks[serviceWithHCKey]) > 0 {
		r.healthchecksResultsByServiceWithHealthchecks[serviceWithHCKey] = r.healthchecksResultsByServiceWithHealthchecks[serviceWithHCKey][:n]
	} else {
		r.healthchecksResultsByServiceWithHealthchecks[serviceWithHCKey] = make([]HealthcheckTarget, 0, 4)
	}

	// add new pods IPs to targets slice
	for _, pod := range podList.Items {
		if !podShouldBeTracked(&pod) {
			// pod has no IP address or has already reached a terminal phase
			r.logger.Debug("pod is not eligible for publishing, skipping", "pod_name", pod.GetName(), "pod_phase", pod.Status.Phase, "swh_name", hc.Name, "namespace", hc.Namespace)
			continue
		}
		state := podsStateMap[types.NamespacedName{Name: pod.GetName(), Namespace: pod.GetNamespace()}]
		targetNotFound := true
		var oldIndex int
		for i, target := range r.healthchecksResultsByServiceWithHealthchecks[serviceWithHCKey] {
			if target.podName == pod.Name {
				targetNotFound = false
				oldIndex = i
				break
			}
		}

		if targetNotFound {
			// append new target
			r.logger.Info("append target pod for service", "pod_name", pod.GetName(), "swh_name", hc.Name, "namespace", hc.Namespace)
			r.healthchecksResultsByServiceWithHealthchecks[serviceWithHCKey] = append(r.healthchecksResultsByServiceWithHealthchecks[serviceWithHCKey], HealthcheckTarget{
				targetHost:         pod.Status.PodIP,
				creationTime:       pod.CreationTimestamp.Time,
				probeResultDetails: []ProbeResultDetail{},
				podName:            pod.GetName(),
				podNamespace:       pod.GetNamespace(),
				podUID:             pod.GetUID(),
				podReady:           state.ready,
				podTerminating:     state.terminating,
			})
		} else {
			// or update existing one
			target := &r.healthchecksResultsByServiceWithHealthchecks[serviceWithHCKey][oldIndex]

			// probes run for ready and terminating pods only, and their results belong to a
			// particular pod instance and IP, so stale ones must not keep the endpoint published
			if (!state.ready && !state.terminating) || target.podUID != pod.GetUID() || target.targetHost != pod.Status.PodIP {
				target.probeResultDetails = []ProbeResultDetail{}
			}

			target.podUID = pod.GetUID()
			target.podReady = state.ready
			target.podTerminating = state.terminating
			target.targetHost = pod.Status.PodIP
			target.creationTime = pod.CreationTimestamp.Time
			r.logger.Debug("update target pod for service", "pod_name", pod.GetName(), "swh_name", hc.Name, "namespace", hc.Namespace)
		}
	}
	r.mu.Unlock()
}

func (r *ServiceWithHealthchecksReconciler) buildRenewedStatus(hc *networkv1alpha1.ServiceWithHealthchecks) *networkv1alpha1.ServiceWithHealthchecksStatus {
	endpoints := r.buildEndpointStatuses(hc)
	readyEndpoints := onlyReadyEndpoints(endpoints)

	status := isEqualReadyAndAll(int32(len(endpoints)), readyEndpoints)
	message := "All endpoints are ready"
	reason := "AllEndpointsAreReady"
	if status == metav1.ConditionFalse {
		message = "Not all endpoints are ready"
		reason = "NotAllEndpointsAreReady"
	}

	return &networkv1alpha1.ServiceWithHealthchecksStatus{
		EndpointStatuses: endpoints,
		HealthcheckCondition: networkv1alpha1.HealthcheckCondition{
			ObservedGeneration: hc.Generation,
			Endpoints:          int32(len(endpoints)),
			ReadyEndpoints:     readyEndpoints,
		},
		Conditions: []metav1.Condition{
			{
				Type:               "Ready",
				LastTransitionTime: metav1.Now(),
				Status:             status,
				Reason:             reason,
				Message:            message,
			},
		},
	}
}

func areAllProbesSucceed(probeResultDetail []ProbeResultDetail) *bool {
	successfulCount := 0
	for _, probeResultDetail := range probeResultDetail {
		if probeResultDetail.successful {
			successfulCount++
		}
	}
	result := successfulCount > 0 && successfulCount == len(probeResultDetail)
	return &result
}

func MakeSliceCopy[T any](originalSlice []T) []T {
	newSlice := make([]T, len(originalSlice))
	copy(newSlice, originalSlice)
	return newSlice
}

// endpointKey identifies an endpoint for sorting, falling back to the addresses because
// TargetRef may be absent in slices written by another actor.
func endpointKey(endpoint discoveryv1.Endpoint) string {
	if endpoint.TargetRef != nil {
		return string(endpoint.TargetRef.UID)
	}
	return strings.Join(endpoint.Addresses, ",")
}

// The three helpers below follow the EndpointSlice API defaults for unset conditions.
func endpointIsReady(endpoint discoveryv1.Endpoint) bool {
	return endpoint.Conditions.Ready == nil || *endpoint.Conditions.Ready
}

func endpointIsServing(endpoint discoveryv1.Endpoint) bool {
	if endpoint.Conditions.Serving == nil {
		return endpointIsReady(endpoint)
	}
	return *endpoint.Conditions.Serving
}

func endpointIsTerminating(endpoint discoveryv1.Endpoint) bool {
	return endpoint.Conditions.Terminating != nil && *endpoint.Conditions.Terminating
}

func endpointsAreEqual(old, new []discoveryv1.Endpoint) bool {
	if len(old) != len(new) {
		return false
	}

	// sort copies to keep the caller's slices intact
	oldSorted := MakeSliceCopy(old)
	newSorted := MakeSliceCopy(new)
	sort.Slice(oldSorted, func(i, j int) bool {
		return endpointKey(oldSorted[i]) < endpointKey(oldSorted[j])
	})
	sort.Slice(newSorted, func(i, j int) bool {
		return endpointKey(newSorted[i]) < endpointKey(newSorted[j])
	})

	for i := range oldSorted {
		if endpointKey(oldSorted[i]) != endpointKey(newSorted[i]) {
			return false
		}
		if !slices.Equal(oldSorted[i].Addresses, newSorted[i].Addresses) {
			return false
		}
		// without comparing the conditions an endpoint which became not ready would stay
		// published as ready until the set of endpoints itself changes
		if endpointIsReady(oldSorted[i]) != endpointIsReady(newSorted[i]) {
			return false
		}
		if endpointIsServing(oldSorted[i]) != endpointIsServing(newSorted[i]) {
			return false
		}
		if endpointIsTerminating(oldSorted[i]) != endpointIsTerminating(newSorted[i]) {
			return false
		}
	}
	return true
}

func sortEndpointStatuses(statuses []networkv1alpha1.EndpointStatus) {
	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].PodName < statuses[j].PodName
	})
}

func onlyReadyEndpoints(statuses []networkv1alpha1.EndpointStatus) int32 {
	result := int32(0)
	for _, status := range statuses {
		// Ready already combines kubelet readiness with the custom probes. ProbesSuccessful is still
		// required here because statuses written by another node are carried over untouched, and one
		// written by an agent that predates that change holds kubelet readiness alone.
		if status.Ready && status.ProbesSuccessful {
			result++
		}
	}
	return result
}

func isEqualReadyAndAll(endpoints int32, readyEndpoints int32) metav1.ConditionStatus {
	if endpoints > 0 && endpoints == readyEndpoints {
		return metav1.ConditionTrue
	}
	return metav1.ConditionFalse
}
