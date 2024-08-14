/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package agent

import (
	"context"
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
	client.Client
	Scheme              *runtime.Scheme
	logger              logr.Logger
	healthecksByService map[types.NamespacedName][]HealthcheckTarget
	tasks               chan ProbeTask
	tasksResults        chan ProbeResult
	events              chan event.GenericEvent
	cancelFunc          context.CancelFunc
}

func NewServiceWithHealthchecksReconciler(client client.Client, workersCount int, nodeName string, scheme *runtime.Scheme, logger logr.Logger) *ServiceWithHealthchecksReconciler {
	return &ServiceWithHealthchecksReconciler{
		workersCount:        workersCount,
		nodeName:            nodeName,
		Client:              client,
		Scheme:              scheme,
		logger:              logger,
		tasks:               make(chan ProbeTask, workersCount),
		tasksResults:        make(chan ProbeResult, workersCount),
		events:              make(chan event.GenericEvent),
		healthecksByService: make(map[types.NamespacedName][]HealthcheckTarget),
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
	)
	r.logger.Info("reconciling ServiceWithHealthchecks", "name", req.Name, "namespace", req.Namespace)
	if err := r.Get(ctx, req.NamespacedName, &serviceWithHC); err != nil {
		r.logger.Error(err, "unable to fetch ServiceWithHealthchecks")
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// delete helthchecks from internal map because service was deleted
	if serviceWithHC.DeletionTimestamp != nil {
		r.logger.Info("ServiceWithHealthchecks is being deleted")
		r.mu.Lock()
		delete(r.healthecksByService, req.NamespacedName)
		r.mu.Unlock()
		return ctrl.Result{}, nil
	}

	// only pods in target namespace, with specified label and on current node
	if err := r.List(ctx, &podList, client.InNamespace(serviceWithHC.GetNamespace()), client.MatchingLabels(serviceWithHC.Spec.Selector), client.MatchingFields{"spec.nodeName": r.nodeName}); err != nil {
		return ctrl.Result{}, err
	}

	podsReadinessMap := getPodsReadinessMap(podList)

	// clean unused pod IPs from targets slice
	r.mu.Lock()
	n := 0
	if len(r.healthecksByService[req.NamespacedName]) > 0 {
		for _, target := range r.healthecksByService[req.NamespacedName] {
			targetFound := false
			for _, pod := range podList.Items {
				if pod.Status.PodIP == target.targetHost {
					targetFound = true
					// TODO: need to deep compare healthchecks?
					break
				}
			}
			if targetFound {
				// delete target from targets slice
				r.healthecksByService[req.NamespacedName][n] = target
				n++
			}
		}
		r.healthecksByService[req.NamespacedName] = r.healthecksByService[req.NamespacedName][:n]
	}

	// add new pods IPs to targets slice
	for _, pod := range podList.Items {
		targetNotFound := true
		var oldTarget HealthcheckTarget
		var oldIndex int
		for i, target := range r.healthecksByService[req.NamespacedName] {
			if target.targetHost == pod.Status.PodIP {
				targetNotFound = false
				oldTarget = target
				oldIndex = i
				break
			}
		}

		if len(r.healthecksByService[req.NamespacedName]) == 0 {
			r.healthecksByService[req.NamespacedName] = make([]HealthcheckTarget, 0, 4)
		}

		newTarget := HealthcheckTarget{
			targetHost:         pod.Status.PodIP,
			creationTime:       pod.CreationTimestamp.Time,
			periodSeconds:      int64(serviceWithHC.Spec.Healthcheck.PeriodSeconds),
			initialDelay:       time.Duration(serviceWithHC.Spec.Healthcheck.InitialDelaySeconds) * time.Second,
			probes:             r.getProbesFromServiceWithHealthchecks(serviceWithHC, pod.Status.PodIP),
			probeResultDetails: []ProbeResultDetail{},
			podName:            pod.GetName(),
			podUID:             pod.GetUID(),
			podReady:           podsReadinessMap[types.NamespacedName{Name: pod.GetName(), Namespace: req.Namespace}],
		}
		if targetNotFound {
			r.logger.Info("append target pod for service", "podName", newTarget.podName, "serviceName", req.Name, "namespace", req.Namespace)
			r.healthecksByService[req.NamespacedName] = append(r.healthecksByService[req.NamespacedName], newTarget)
		}

		// Update target if needed
		if !targetNotFound && !newTarget.EqualTo(oldTarget) {
			r.logger.Info("update target pod for service", "podName", newTarget.podName, "serviceName", req.Name, "namespace", req.Namespace)
			r.healthecksByService[req.NamespacedName][oldIndex] = newTarget
		}
	}
	r.mu.Unlock()

	// update EPS
	err := r.updateEPSForServiceWithHealthchecks(ctx, serviceWithHC)
	if err != nil {
		r.logger.Error(err, "unable to update EPS")
		return ctrl.Result{}, err
	}

	// update status
	updatedServiceWithHC := serviceWithHC.DeepCopy()
	patch := client.MergeFrom(serviceWithHC.DeepCopy())
	isNew := len(updatedServiceWithHC.Status.Conditions) == 0
	updatedServiceWithHC.Status.EndpointStatuses = r.buildEndpointStatuses(updatedServiceWithHC)
	updatedServiceWithHC.Status.HealthcheckCondition.ObservedGeneration = updatedServiceWithHC.Generation
	updatedServiceWithHC.Status.HealthcheckCondition.Endpoints = int32(len(updatedServiceWithHC.Status.EndpointStatuses))
	updatedServiceWithHC.Status.HealthcheckCondition.ReadyEndpoints = onlyReadyEndpoints(updatedServiceWithHC.Status.EndpointStatuses)

	updatedServiceWithHC.Status.Conditions = []metav1.Condition{
		{
			Type:               "Ready",
			LastTransitionTime: metav1.Now(),
			Status:             isEqualReadyAndAll(updatedServiceWithHC.Status.HealthcheckCondition.Endpoints, updatedServiceWithHC.Status.HealthcheckCondition.ReadyEndpoints),
			Reason:             "AllEndpointsAreReady",
			Message:            "All endpoints are ready",
		},
	}

	if isNew {
		r.logger.Info("updating status of service with healthchecks first time", "name", updatedServiceWithHC.GetName(), "namespace", updatedServiceWithHC.GetNamespace())
		err = r.Status().Update(ctx, updatedServiceWithHC)
	} else {
		r.logger.Info("updating status of service with healthchecks", "name", updatedServiceWithHC.GetName(), "namespace", updatedServiceWithHC.GetNamespace())
		err = r.Status().Patch(ctx, updatedServiceWithHC, patch)
	}
	if err != nil {
		r.logger.Error(err, "unable to update status of service with healthchecks", "name", updatedServiceWithHC.GetName(), "namespace", updatedServiceWithHC.GetNamespace())
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
	for _, result := range r.healthecksByService[types.NamespacedName{Name: svc.GetName(), Namespace: svc.GetNamespace()}] {
		endpointStatuses = append(endpointStatuses, networkv1alpha1.EndpointStatus{
			PodName:          result.podName,
			NodeName:         r.nodeName,
			Ready:            false, //TODO: pod readiness
			ProbesSuccessful: *areAllProbesSucceeed(result.probeResultDetails),
			FailedProbes:     result.FailedProbes(),
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

	r.mu.RLock()
	defer r.mu.RUnlock()

	// check if pod labels matches services selectors
	for serviceName := range r.healthecksByService {
		// get service labels
		service := &networkv1alpha1.ServiceWithHealthchecks{}
		if err := r.Client.Get(ctx, serviceName, service); err != nil {
			r.logger.Error(err, "failed to get service", "serviceName", serviceName)
			continue
		}

		//get pod labels
		podLabels := pod.GetLabels()

		if labels.ValidatedSetSelector(service.Spec.Selector).Matches(labels.Set(podLabels)) {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      service.GetName(),
					Namespace: service.GetNamespace(),
				},
			})
		}

	}
	return requests
}

func (r *ServiceWithHealthchecksReconciler) RunWorkers(ctx context.Context) error {
	r.logger.Info("starting workers", "workersCount", r.workersCount)

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
	close(r.tasks)
	close(r.tasksResults)
	close(r.events)
}

func (r *ServiceWithHealthchecksReconciler) RunTasksScheduler(ctx context.Context) {
	r.logger.Info("making tasks")
	ticker := time.NewTicker(time.Second * 1)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// write task to channel
			r.mu.RLock()
			for serviceName := range r.healthecksByService {
				for i := range r.healthecksByService[serviceName] {
					healthcheckTarget := r.healthecksByService[serviceName][i]
					if !healthcheckTarget.podReady {
						// skip pods which are not ready
						continue
					}
					now := time.Now()
					diff := now.Sub(healthcheckTarget.creationTime).Seconds()
					if diff < healthcheckTarget.initialDelay.Seconds() {
						continue // skip task while initial delay
					}
					diff = now.Sub(healthcheckTarget.lastCheck).Seconds()
					if diff < float64(healthcheckTarget.periodSeconds) {
						continue // skip task while period elapsed
					}
					r.tasks <- ProbeTask{
						host:        healthcheckTarget.targetHost,
						serviceName: serviceName,
						probes:      healthcheckTarget.GetRenewedProbes(),
					}
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
		if _, exists := r.healthecksByService[result.serviceName]; !exists {
			r.logger.Info("Could not update probes result for service - service is not founded", "name", result.serviceName.String())
		} else {
			for i, target := range r.healthecksByService[result.serviceName] {
				if target.targetHost == result.host {
					r.healthecksByService[result.serviceName][i].lastCheck = time.Now()
					r.healthecksByService[result.serviceName][i].probeResultDetails = result.probeDetails
					//generate event for watcher
					//TODO: add comprasion between old and new events?
					r.events <- event.GenericEvent{Object: &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: result.serviceName.Name, Namespace: result.serviceName.Namespace}}}
				}
			}
		}
		r.mu.Unlock()
	}
}

func (r *ServiceWithHealthchecksReconciler) RunTaskWorker(ctx context.Context) {
	r.logger.Info("running task")
	for task := range r.tasks {
		r.logger.Info("running task", "host", task.host, "serviceName", task.serviceName.String())
		g, _ := errgroup.WithContext(ctx)
		probesResultDetails := make([]ProbeResultDetail, len(task.probes))
		for i, probe := range task.probes {
			i, probe := i, probe
			g.Go(func() error {
				err := probe.PerformCheck()
				successCount, failureCount := calculateCounts(err, probe.SuccessCount(), probe.FailureCount())
				probesResultDetails[i] = ProbeResultDetail{
					id:               probe.GetID(),
					successful:       err == nil,
					mode:             probe.GetMode(),
					targetPort:       probe.GetPort(),
					successCount:     successCount,
					failureCount:     failureCount,
					successThreshold: probe.SuccessThreshold(),
					failureThreshold: probe.FailureThreshold(),
				}
				r.logger.Info("probe result", "host", task.host, "successful", err == nil, "mode", probe.GetMode(), "targetPort", probe.GetPort())
				return err
			})
		}
		err := g.Wait()
		r.tasksResults <- ProbeResult{
			host:         task.host,
			serviceName:  task.serviceName,
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
	r.logger.Info("updating endpoints for service", "serviceName", svc.GetName(), "namespace", svc.GetNamespace())
	desiredNameForEndpointSlice := svc.GetName() + "-" + r.nodeName
	eps := discoveryv1.EndpointSlice{}
	err := r.Get(ctx, client.ObjectKey{Namespace: svc.GetNamespace(), Name: desiredNameForEndpointSlice}, &eps)

	if errors.IsNotFound(err) {
		// need to create endpoint slice
		r.logger.Info("could not found endpoints for service and node, create one...", "serviceName", svc.GetName(), "namespace", svc.GetNamespace(), "node", r.nodeName)
		eps = r.BuildEndpointSlice(desiredNameForEndpointSlice, svc)

		// if EPS enpoints are empty we don't need to create one
		if len(eps.Endpoints) == 0 {
			return nil
		}

		if err := r.Create(ctx, &eps); err != nil {
			r.logger.Error(err, "couldn't create endpoints for service and node", "serviceName", svc.GetName(), "namespace", svc.GetNamespace(), "node", r.nodeName)
			return err
		}
		return nil
	}
	if err != nil {
		r.logger.Error(err, "couldn't get endpoints for service and node", "serviceName", svc.GetName(), "namespace", svc.GetNamespace(), "node", r.nodeName)
		return err
	}

	oldEnpoints := MakeSliceCopy(eps.Endpoints)
	newEnpoints := r.buildEndpoints(svc)
	if !endpointsAreEqual(oldEnpoints, newEnpoints) {
		eps.Endpoints = newEnpoints
		if err := r.Update(ctx, &eps); err != nil {
			r.logger.Error(err, "couldn't update endpoints for service and node", "serviceName", svc.GetName(), "namespace", svc.GetNamespace(), "node", r.nodeName)
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
		ports = append(ports, discoveryv1.EndpointPort{
			Name:     &port.Name,
			Port:     &port.Port,
			Protocol: &port.Protocol,
		})
	}
	return ports
}

func (r *ServiceWithHealthchecksReconciler) buildEndpoints(svc networkv1alpha1.ServiceWithHealthchecks) []discoveryv1.Endpoint {
	endpoints := []discoveryv1.Endpoint{}
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, probeResult := range r.healthecksByService[types.NamespacedName{Name: svc.GetName(), Namespace: svc.GetNamespace()}] {
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

func (r *ServiceWithHealthchecksReconciler) getProbesFromServiceWithHealthchecks(svc networkv1alpha1.ServiceWithHealthchecks, targetHost string) []Prober {
	probes := make([]Prober, 0, len(svc.Spec.Healthcheck.Probes))
	for _, serviceProbe := range svc.Spec.Healthcheck.Probes {
		switch strings.ToLower(serviceProbe.Mode) {
		case "http":
			probes = append(probes, HTTPProbeTarget{
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
			creds, err := r.getPostgreSQLCredentials(serviceProbe.PostgreSQL, svc.GetNamespace())
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
	var creds PostgreSQLCredentials
	var secret corev1.Secret
	err := r.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: sqlHandler.AuthSecretName}, &secret)
	if err != nil {
		return creds, err
	}
	creds.TlsMode = getNativeTLSMode(string(secret.Data["tlsMode"]))
	creds.User = string(secret.Data["user"])
	creds.Password = string(secret.Data["password"])
	creds.ClientCert = string(secret.Data["clientCert"])
	creds.ClientKey = string(secret.Data["clientKey"])
	creds.CaCert = string(secret.Data["caCert"])
	return creds, nil
}

func areAllProbesSucceeed(probeResultDetail []ProbeResultDetail) *bool {
	successfullCount := 0
	for _, probeResultDetail := range probeResultDetail {
		if probeResultDetail.successCount >= probeResultDetail.successThreshold || probeResultDetail.failureCount < probeResultDetail.failureThreshold {
			successfullCount++
		}
	}
	result := successfullCount == len(probeResultDetail)
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

func isEqualReadyAndAll(endpoints int32, endpoints2 int32) metav1.ConditionStatus {
	if endpoints == endpoints2 {
		return metav1.ConditionTrue
	}
	return metav1.ConditionFalse
}
