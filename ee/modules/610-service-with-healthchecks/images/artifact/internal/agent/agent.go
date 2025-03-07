/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package agent

import (
	"context"
	"reflect"
	"service-with-healthchecks/internal/kubernetes"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	networkv1alpha1 "service-with-healthchecks/api/v1alpha1"
)

const (
	endpointServiceNameLabelKey = "kubernetes.io/service-name"
	endpointControllerLabelKey  = "endpointslice.kubernetes.io/managed-by"
	controllerName              = "servicewithhealthchecks"
)

// ServiceWithHealthchecksReconciler reconciles a ServiceWithHealthchecks object
type ServiceWithHealthchecksReconciler struct {
	workersCount int
	nodeName     string
	mu           sync.RWMutex
	muInProcess  sync.RWMutex
	client.Client
	Scheme                                     *runtime.Scheme
	logger                                     logr.Logger
	taskQueue                                  *TaskQueue
	tasksResults                               chan ProbeResult
	events                                     chan event.GenericEvent
	cancelFunc                                 context.CancelFunc
	servicesWithHealthchecks                   sync.Map
	healthecksResultsByServiceWithHealthchecks map[types.NamespacedName][]HealthcheckTarget
	secretController                           *PostgreSQLCredentialsReconciler
}

func NewServiceWithHealthchecksReconciler(client client.Client, workersCount int, nodeName string, scheme *runtime.Scheme, logger logr.Logger, secretController *PostgreSQLCredentialsReconciler) *ServiceWithHealthchecksReconciler {
	return &ServiceWithHealthchecksReconciler{
		workersCount: workersCount,
		nodeName:     nodeName,
		Client:       client,
		Scheme:       scheme,
		logger:       logger,
		taskQueue:    NewTaskQueue(),
		tasksResults: make(chan ProbeResult, workersCount*10),
		events:       make(chan event.GenericEvent),
		healthecksResultsByServiceWithHealthchecks: make(map[types.NamespacedName][]HealthcheckTarget),
		secretController: secretController,
	}
}

// +kubebuilder:rbac:groups=network.deckhouse.io,resources=servicewithhealthchecks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.deckhouse.io,resources=servicewithhealthchecks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=network.deckhouse.io,resources=servicewithhealthchecks/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ServiceWithHealthchecks object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.2/pkg/reconcile
func (r *ServiceWithHealthchecksReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		serviceWithHC networkv1alpha1.ServiceWithHealthchecks
		podList       corev1.PodList
		err           error
	)
	r.logger.V(1).Info("reconciling ServiceWithHealthchecks", "name", req.Name, "namespace", req.Namespace)
	if err := r.Get(ctx, req.NamespacedName, &serviceWithHC); err != nil {
		r.logger.Error(err, "unable to fetch ServiceWithHealthchecks")
		if errors.IsNotFound(err) {
			r.deleteServiceWithHealthcheks(req.NamespacedName)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Delete helthchecks from internal map because ServiceWithHealthchecks was deleted
	if serviceWithHC.DeletionTimestamp != nil {
		r.logger.V(1).Info("ServiceWithHealthchecks is being deleted")
		r.deleteServiceWithHealthcheks(req.NamespacedName)
		return ctrl.Result{}, nil
	}

	// Select only pods in target namespace, with specified label and on current node
	if err := r.List(ctx, &podList, client.InNamespace(serviceWithHC.GetNamespace()), client.MatchingLabels(serviceWithHC.Spec.Selector), client.MatchingFields{"spec.nodeName": r.nodeName}); err != nil {
		return ctrl.Result{}, err
	}

	// Create internal value with spec
	value, ok := r.servicesWithHealthchecks.Load(req.NamespacedName)
	if !ok || !reflect.DeepEqual(value.(networkv1alpha1.ServiceWithHealthchecksSpec), serviceWithHC.Spec) {
		r.servicesWithHealthchecks.Store(req.NamespacedName, serviceWithHC.Spec)
	}

	// sync internal probes targets with exsiting pods
	r.syncResultsMapWithPodList(serviceWithHC, podList)

	// update endpointslices unless ClusterIP is None
	if serviceWithHC.Spec.ClusterIP != "None" {
		err = r.updateEPSForServiceWithHealthchecks(ctx, serviceWithHC)
		if err != nil {
			r.logger.Error(err, "unable to update EPS for ServiceWithHealthchecks")
			return ctrl.Result{}, err
		}
	}

	// update status
	updatedServiceWithHC := serviceWithHC.DeepCopy()
	patch := client.MergeFrom(serviceWithHC.DeepCopy())
	isNew := len(updatedServiceWithHC.Status.Conditions) == 0

	newStatus := r.buildRenewedStatus(updatedServiceWithHC)
	updatedServiceWithHC.Status.HealthcheckCondition = newStatus.HealthcheckCondition
	updatedServiceWithHC.Status.EndpointStatuses = newStatus.EndpointStatuses
	updatedServiceWithHC.Status.Conditions = kubernetes.UpdateStatusWithConditions(serviceWithHC.Status.Conditions, newStatus.Conditions)

	if isNew {
		r.logger.V(1).Info("updating status of ServiceWithHealthchecks with healthchecks probes results", "name", updatedServiceWithHC.GetName(), "namespace", updatedServiceWithHC.GetNamespace())
		updatedServiceWithHC.Status = *newStatus
		err = r.Status().Update(ctx, updatedServiceWithHC)
		if err != nil {
			r.logger.Error(err, "unable to update status of ServiceWithHealthchecks", "name", updatedServiceWithHC.GetName(), "namespace", updatedServiceWithHC.GetNamespace())
			return ctrl.Result{}, err
		}
	}
	err = r.Status().Patch(ctx, updatedServiceWithHC, patch)
	if err != nil {
		r.logger.Error(err, "unable to patch status of ServiceWithHealthchecks", "name", updatedServiceWithHC.GetName(), "namespace", updatedServiceWithHC.GetNamespace())
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceWithHealthchecksReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &corev1.Pod{}, "spec.nodeName", func(rawObj client.Object) []string {
		pod := rawObj.(*corev1.Pod)
		return []string{pod.Spec.NodeName}
	}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 4,
		}).
		For(&networkv1alpha1.ServiceWithHealthchecks{}).
		Watches(&corev1.Pod{}, handler.EnqueueRequestsFromMapFunc(r.getExposedServiceWithHCForPod)).
		WatchesRawSource(source.Channel(r.events, &handler.EnqueueRequestForObject{})).
		Complete(r)
}

func (r *ServiceWithHealthchecksReconciler) buildEndpointStatuses(svc *networkv1alpha1.ServiceWithHealthchecks) []networkv1alpha1.EndpointStatus {
	var endpointStatuses []networkv1alpha1.EndpointStatus
	r.mu.RLock()
	defer r.mu.RUnlock()

	// delete previous statuses for current node
	n := 0
	for _, endpointStatus := range svc.Status.EndpointStatuses {
		if endpointStatus.NodeName != r.nodeName {
			svc.Status.EndpointStatuses[n] = endpointStatus
			n++
		}
	}
	endpointStatuses = svc.Status.EndpointStatuses[:n]

	// add new healthchecks probes results
	for _, result := range r.healthecksResultsByServiceWithHealthchecks[types.NamespacedName{Name: svc.GetName(), Namespace: svc.GetNamespace()}] {
		probesSuccessful := true
		failedProbes := []string{}

		// there are always success if svc options set to PublishNotReadyAddresses, otherwise need to evaluate
		if !svc.Spec.PublishNotReadyAddresses {
			probesSuccessful = *areAllProbesSucceeed(result.probeResultDetails)
			failedProbes = result.FailedProbes()
		}

		endpointStatuses = append(endpointStatuses, networkv1alpha1.EndpointStatus{
			PodName:          result.podName,
			NodeName:         r.nodeName,
			Ready:            false, //TODO: pod readiness
			ProbesSuccessful: probesSuccessful,
			FailedProbes:     failedProbes,
			LastProbeTime:    metav1.Time{Time: result.lastCheck},
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
		svcWithHCSpec := value.(networkv1alpha1.ServiceWithHealthchecksSpec)
		podsLabels := pod.GetLabels()

		if labels.ValidatedSetSelector(svcWithHCSpec.Selector).Matches(labels.Set(podsLabels)) {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      svcWithHCName.Name,
					Namespace: svcWithHCName.Namespace,
				},
			})
			return true
		}
		return false
	})
	return requests
}

func (r *ServiceWithHealthchecksReconciler) RunWorkers(ctx context.Context) error {
	r.logger.V(1).Info("starting workers", "workersCount", r.workersCount)

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
			for swhName := range r.healthecksResultsByServiceWithHealthchecks {
				for i := range r.healthecksResultsByServiceWithHealthchecks[swhName] {
					healthcheckTarget := r.healthecksResultsByServiceWithHealthchecks[swhName][i]
					if !healthcheckTarget.podReady {
						// skip pods which are not ready
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
						host:    healthcheckTarget.targetHost,
						swhName: swhName,
						probes:  healthcheckTarget.GetRenewedProbes(probes),
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
		if _, exists := r.healthecksResultsByServiceWithHealthchecks[result.swhName]; !exists {
			r.logger.Info("Could not update probes result for ServiceWithHealthchecks - ServiceWithHealthchecks is not found", "name", result.swhName.String())
			r.mu.Unlock()
			continue
		}

		for i, target := range r.healthecksResultsByServiceWithHealthchecks[result.swhName] {
			if target.targetHost == result.host {
				r.healthecksResultsByServiceWithHealthchecks[result.swhName][i].lastCheck = time.Now()
				r.healthecksResultsByServiceWithHealthchecks[result.swhName][i].probeResultDetails = result.probeDetails
				//generate event for watcher
				r.events <- event.GenericEvent{Object: &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: result.swhName.Name, Namespace: result.swhName.Namespace}}}
			}
		}

		r.mu.Unlock()
	}
}

func (r *ServiceWithHealthchecksReconciler) RunTaskWorker(ctx context.Context) {
	r.logger.V(1).Info("running task")
	for {
		task := r.taskQueue.Dequeue()
		r.logger.V(1).Info("running task", "host", task.host, "swhName", task.swhName.String())
		g, _ := errgroup.WithContext(ctx)
		probesResultDetails := make([]ProbeResultDetail, len(task.probes))
		for i, probe := range task.probes {
			i, probe := i, probe
			g.Go(func() error {
				err := probe.PerformCheck()
				var successful bool
				successCount, failureCount := calculateCounts(err, probe.SuccessCount(), probe.FailureCount())
				if successCount >= probe.SuccessThreshold() {
					successful = true
				}
				if failureCount >= probe.FailureThreshold() {
					successful = false
				}
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
			r.logger.V(1).Error(err, "error performing probes", "host", task.host, "swhName", task.swhName.String())
		}
		r.tasksResults <- ProbeResult{
			host:         task.host,
			swhName:      task.swhName,
			probeDetails: probesResultDetails,
			successful:   err == nil,
		}
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

func (r *ServiceWithHealthchecksReconciler) updateEPSForServiceWithHealthchecks(ctx context.Context, svc networkv1alpha1.ServiceWithHealthchecks) error {
	r.logger.V(1).Info("updating endpoints for service", "swhName", svc.GetName(), "namespace", svc.GetNamespace())
	desiredNameForEndpointSlice := svc.GetName() + "-" + r.nodeName

	// discoveryv1.EndpointSlice
	eps := r.BuildEndpointSlice(desiredNameForEndpointSlice, svc)

	// if EPS enpoints are empty we need to delete existing one
	if len(eps.Endpoints) == 0 {
		err := r.Delete(ctx, &eps)
		if err != nil {
			r.logger.V(1).Info("could not delete EndpointSlice for ServiceWithHealthchecks and node", "swhName", svc.GetName(), "namespace", svc.GetNamespace(), "node", r.nodeName)
			return nil
		}
	}

	err := r.Get(ctx, client.ObjectKey{Namespace: svc.GetNamespace(), Name: desiredNameForEndpointSlice}, &eps)
	if errors.IsNotFound(err) {
		// need to create endpoint slice
		r.logger.Info("could not found EndpointSlice for ServiceWithHealthchecks and node, will create one...", "swhName", svc.GetName(), "namespace", svc.GetNamespace(), "node", r.nodeName)
		eps = r.BuildEndpointSlice(desiredNameForEndpointSlice, svc)

		// if EPS enpoints are empty we don't need to create one
		if len(eps.Endpoints) == 0 {
			return nil
		}

		if err := r.Create(ctx, &eps); err != nil {
			r.logger.Error(err, "couldn't create EndpointSlice for ServiceWithHealthchecks and node", "swhName", svc.GetName(), "namespace", svc.GetNamespace(), "node", r.nodeName)
			return err
		}
		return nil
	}
	if err != nil {
		r.logger.Error(err, "couldn't get EndpointSlice for ServiceWithHealthchecks and node", "swhName", svc.GetName(), "namespace", svc.GetNamespace(), "node", r.nodeName)
		return err
	}

	oldEnpoints := MakeSliceCopy(eps.Endpoints)
	newEnpoints := r.buildEndpoints(svc)
	if !endpointsAreEqual(oldEnpoints, newEnpoints) {
		eps.Endpoints = newEnpoints
		if err := r.Update(ctx, &eps); err != nil {
			r.logger.Error(err, "couldn't update EndpointSlice for ServiceWithHealthchecks and node", "swhName", svc.GetName(), "namespace", svc.GetNamespace(), "node", r.nodeName)
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
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Ports:       r.buildPortsForEndpointslice(svc),
	}

	eps.Endpoints = r.buildEndpoints(svc)
	return eps
}

func (r *ServiceWithHealthchecksReconciler) buildPortsForEndpointslice(svc networkv1alpha1.ServiceWithHealthchecks) []discoveryv1.EndpointPort {
	ports := []discoveryv1.EndpointPort{}
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

	for _, probeResult := range r.healthecksResultsByServiceWithHealthchecks[types.NamespacedName{Name: svc.GetName(), Namespace: svc.GetNamespace()}] {
		if svc.Spec.PublishNotReadyAddresses || *areAllProbesSucceeed(probeResult.probeResultDetails) {
			isReady := probeResult.podReady && *areAllProbesSucceeed(probeResult.probeResultDetails)
			endpoint := discoveryv1.Endpoint{
				Addresses: []string{probeResult.targetHost},
				NodeName:  &r.nodeName,
				TargetRef: &corev1.ObjectReference{
					Kind:      "Pod",
					Name:      probeResult.podName,
					Namespace: svc.GetNamespace(), UID: probeResult.podUID},
				Conditions: discoveryv1.EndpointConditions{
					Ready: &isReady,
				},
			}
			endpoints = append(endpoints, endpoint)
		}
	}
	return endpoints
}

func getPodsReadinessMap(podList corev1.PodList) map[types.NamespacedName]bool {
	podsReadinessMap := make(map[types.NamespacedName]bool)
	for _, pod := range podList.Items {
		podIsReady := true
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if !containerStatus.Ready {
				podIsReady = false
				break
			}
		}
		podsReadinessMap[types.NamespacedName{Name: pod.GetName(), Namespace: pod.GetNamespace()}] = podIsReady
	}
	return podsReadinessMap
}

func (r *ServiceWithHealthchecksReconciler) deleteServiceWithHealthcheks(swhName types.NamespacedName) {
	r.servicesWithHealthchecks.Delete(swhName)
	r.mu.Lock()
	delete(r.healthecksResultsByServiceWithHealthchecks, swhName)
	r.mu.Unlock()
}

func (r *ServiceWithHealthchecksReconciler) getProbesFromServiceWithHealthchecks(svcSpec networkv1alpha1.ServiceWithHealthchecksSpec, targetHost, namespace string) []Prober {
	probes := make([]Prober, 0, len(svcSpec.Healthcheck.Probes))
	for _, serviceProbe := range svcSpec.Healthcheck.Probes {
		switch strings.ToLower(serviceProbe.Mode) {
		case "http":
			probes = append(probes, FastHTTPProbeTarget{
				targetHost:       targetHost,
				host:             serviceProbe.HTTPHandler.Host,
				path:             serviceProbe.HTTPHandler.Path,
				targetPort:       serviceProbe.HTTPHandler.TargetPort.IntValue(),
				scheme:           string(serviceProbe.HTTPHandler.Scheme),
				method:           serviceProbe.HTTPHandler.Method,
				httpHeaders:      serviceProbe.HTTPHandler.HTTPHeaders,
				successThreshold: serviceProbe.SuccessThreshold,
				failureThreshold: serviceProbe.FailureThreshold,
				timeoutSeconds:   serviceProbe.TimeoutSeconds,
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
				r.logger.Error(err, "Failed to get PostgreSQL credentials")
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
				tlsMode:          creds.TlsMode,
			})
		}
	}
	return probes
}

func (r *ServiceWithHealthchecksReconciler) getPostgreSQLCredentials(sqlHandler *networkv1alpha1.PGSQLHandler, namespace string) (PostgreSQLCredentials, error) {
	return r.secretController.GetCachedSecret(types.NamespacedName{Namespace: namespace, Name: sqlHandler.AuthSecretName})
}

func (r *ServiceWithHealthchecksReconciler) syncResultsMapWithPodList(hc networkv1alpha1.ServiceWithHealthchecks, podList corev1.PodList) {
	serviceWithHCKey := types.NamespacedName{Namespace: hc.Namespace, Name: hc.Name}
	podsReadinessMap := getPodsReadinessMap(podList)
	r.mu.Lock()
	// clean unused pod IPs from result slice
	n := 0
	for _, target := range r.healthecksResultsByServiceWithHealthchecks[serviceWithHCKey] {
		if _, exists := podsReadinessMap[types.NamespacedName{Namespace: hc.Namespace, Name: target.podName}]; exists {
			r.healthecksResultsByServiceWithHealthchecks[serviceWithHCKey][n] = target
			n++
		}
	}
	if len(r.healthecksResultsByServiceWithHealthchecks[serviceWithHCKey]) > 0 {
		r.healthecksResultsByServiceWithHealthchecks[serviceWithHCKey] = r.healthecksResultsByServiceWithHealthchecks[serviceWithHCKey][:n]
	} else {
		r.healthecksResultsByServiceWithHealthchecks[serviceWithHCKey] = make([]HealthcheckTarget, 0, 4)
	}

	// add new pods IPs to targets slice
	for _, pod := range podList.Items {
		if pod.Status.PodIP == "" {
			// pod has no IP address (for example, it's in pending state), skipping
			r.logger.V(1).Info("pod has no IP address, skipping", "podName", pod.GetName(), "swhName", hc.Name, "namespace", hc.Namespace)
			continue
		}
		targetNotFound := true
		var oldIndex int
		for i, target := range r.healthecksResultsByServiceWithHealthchecks[serviceWithHCKey] {
			if target.podName == pod.Name {
				targetNotFound = false
				oldIndex = i
				break
			}
		}

		if targetNotFound {
			// append new target
			r.logger.Info("append target pod for service", "podName", pod.GetName(), "swhName", hc.Name, "namespace", hc.Namespace)
			r.healthecksResultsByServiceWithHealthchecks[serviceWithHCKey] = append(r.healthecksResultsByServiceWithHealthchecks[serviceWithHCKey], HealthcheckTarget{
				targetHost:         pod.Status.PodIP,
				creationTime:       pod.CreationTimestamp.Time,
				probeResultDetails: []ProbeResultDetail{},
				podName:            pod.GetName(),
				podNamespace:       pod.GetNamespace(),
				podUID:             pod.GetUID(),
				podReady:           podsReadinessMap[types.NamespacedName{Name: pod.GetName(), Namespace: pod.GetNamespace()}],
			})
		} else {
			// or update existing one
			r.healthecksResultsByServiceWithHealthchecks[serviceWithHCKey][oldIndex].podUID = pod.GetUID()
			r.healthecksResultsByServiceWithHealthchecks[serviceWithHCKey][oldIndex].podReady = podsReadinessMap[types.NamespacedName{Name: pod.GetName(), Namespace: pod.GetNamespace()}]
			r.healthecksResultsByServiceWithHealthchecks[serviceWithHCKey][oldIndex].targetHost = pod.Status.PodIP
			r.healthecksResultsByServiceWithHealthchecks[serviceWithHCKey][oldIndex].creationTime = pod.CreationTimestamp.Time
			r.logger.V(1).Info("update target pod for service", "podName", pod.GetName(), "swhName", hc.Name, "namespace", hc.Namespace)
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

func areAllProbesSucceeed(probeResultDetail []ProbeResultDetail) *bool {
	successfullCount := 0
	for _, probeResultDetail := range probeResultDetail {
		if probeResultDetail.successful {
			successfullCount++
		}
	}
	result := successfullCount > 0 && successfullCount == len(probeResultDetail)
	return &result
}

func MakeSliceCopy[T any](originalSlice []T) []T {
	newSlice := make([]T, len(originalSlice))
	copy(newSlice, originalSlice)
	return newSlice
}

func endpointsAreEqual(old, new []discoveryv1.Endpoint) bool {
	sort.Slice(old, func(i, j int) bool {
		return old[i].TargetRef.UID < old[j].TargetRef.UID
	})
	sort.Slice(new, func(i, j int) bool {
		return new[i].TargetRef.UID < new[j].TargetRef.UID
	})
	if len(old) != len(new) {
		return false
	}
	for i := range old {
		if old[i].TargetRef.UID != new[i].TargetRef.UID {
			return false
		}
		if strings.Join(old[i].Addresses, "") != strings.Join(new[i].Addresses, "") {
			return false
		}
	}
	return true
}

func onlyReadyEndpoints(statuses []networkv1alpha1.EndpointStatus) int32 {
	result := int32(0)
	for _, status := range statuses {
		if status.ProbesSuccessful {
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
