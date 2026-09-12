/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package controller

import (
	"context"
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/deckhouse/deckhouse/pkg/log"

	networkv1alpha1 "service-with-healthchecks/api/v1alpha1"
	"service-with-healthchecks/internal/kubernetes"
)

const (
	endpointControllerLabelKey  = "endpointslice.kubernetes.io/managed-by"
	controllerName              = "servicewithhealthchecks"
	serviceWithHealthchecksKind = "ServiceWithHealthchecks"

	childServiceConditionType = "ChildServiceReady"
	// The condition used to be called differently. A stale copy is dropped from the status, so
	// that a resource reconciled by an older version does not keep reporting through it.
	legacyChildServiceConditionType = "ChildService"

	// resyncPeriod bounds how long an EndpointSlice of a node that no longer exists may stay
	// published. clearNotUsedEPS runs on reconciliation only, and nothing watches Nodes, so
	// without the resync a removed node would keep a dead endpoint in the child Service until
	// the next unrelated event on the object. It is longer than the agent's own resync because
	// this only performs cleanup, and each pass lists Nodes cluster-wide from the cache.
	resyncPeriod = 5 * time.Minute

	// A conflicting Service is not owned by the module, so its changes never reach this
	// controller through Owns() either. The resync alone would eventually notice the clash
	// being resolved; this shorter interval is what makes the recovery prompt.
	childServiceConflictRetry = time.Minute
)

// ServiceWithHealthchecksReconciler reconciles a ServiceWithHealthchecks object
type ServiceWithHealthchecksReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Logger *log.Logger
}

// +kubebuilder:rbac:groups=network.deckhouse.io,resources=servicewithhealthchecks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.deckhouse.io,resources=servicewithhealthchecks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=network.deckhouse.io,resources=servicewithhealthchecks/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ServiceWithHealthchecksReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Logger.Debug("reconciling ServiceWithHealthchecks", "name", req.Name, "namespace", req.Namespace)
	serviceWithHC := &networkv1alpha1.ServiceWithHealthchecks{}
	if err := r.Get(ctx, req.NamespacedName, serviceWithHC); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		r.Logger.Error("failed to reconcile ServiceWithHealthchecks", log.Err(err), "name", req.Name, "namespace", req.Namespace)
		return ctrl.Result{}, err
	}

	// clear EPS for disappeared Nodes
	deletedCount := r.clearNotUsedEPS(ctx, req)
	r.Logger.Debug("deleted orphan EndpointSlices", "namespace", req.Namespace, "count", deletedCount)

	// create or update child service (skip if spec already matches)
	var childService corev1.Service
	childService.Name = req.Name
	childService.Namespace = req.Namespace

	var errUpdatingSvc error
	var conflictErr, immutableErr *childServiceProblem
	var service corev1.Service
	err := r.Get(ctx, req.NamespacedName, &service)
	if err != nil && !errors.IsNotFound(err) {
		r.Logger.Error("failed to get child Service", log.Err(err), "name", req.Name, "namespace", req.Namespace)
		return ctrl.Result{}, err
	}
	if err == nil {
		conflictErr = childServiceConflict(&service, serviceWithHC)
		immutableErr = clusterIPMismatch(&service, serviceWithHC)
	}

	switch {
	case conflictErr != nil:
		// The Service belongs to somebody else. It is neither modified nor claimed, and the
		// agents keep publishing their EndpointSlices for it: they are built from the
		// ServiceWithHealthchecks alone, so the healthchecks stay in effect while the user
		// resolves the clash.
		r.Logger.Info("child Service conflict", "name", req.Name, "namespace", req.Namespace, "reason", conflictErr.Error())
		childService = service

	// The owner reference is part of the desired state as much as the spec is: a Service left
	// over from a version that did not set it, or one whose reference was stripped, has to be
	// adopted here, otherwise nothing ever collects it.
	case err == nil && metav1.IsControlledBy(&service, serviceWithHC) &&
		IsSpecForServiceEqual(service, serviceWithHC) && IsMetadataForServiceEqual(service, serviceWithHC):
		r.Logger.Debug("no need to update child Service", "name", req.Name, "namespace", req.Namespace)
		childService = service

	default:
		var op controllerutil.OperationResult
		op, errUpdatingSvc = controllerutil.CreateOrUpdate(ctx, r.Client, &childService, func() error {
			// Ensure owner reference is always set (idempotent — restores it if accidentally removed).
			//
			// BlockOwnerDeletion is deliberately turned off: it only matters for a foreground
			// deletion of the parent, which nothing here needs, while with the
			// OwnerReferencesPermissionEnforcement admission plugin enabled it would require the
			// controller to have access to the servicewithhealthchecks/finalizers subresource.
			if err := controllerutil.SetControllerReference(serviceWithHC, &childService, r.Scheme,
				controllerutil.WithBlockOwnerDeletion(false)); err != nil {
				return err
			}

			// External controllers (MetalLB, cloud providers) read the load balancer
			// settings from the Service, so the parent metadata has to reach it.
			desiredAnnotations := propagatedAnnotations(serviceWithHC.Annotations)
			desiredLabels := serviceWithHC.Labels

			annotations := mergePropagated(childService.Annotations, desiredAnnotations, childService.Annotations[propagatedAnnotationsKey])
			childService.Labels = mergePropagated(childService.Labels, desiredLabels, childService.Annotations[propagatedLabelsKey])

			annotations = setPropagatedKeys(annotations, propagatedAnnotationsKey, desiredAnnotations)
			childService.Annotations = setPropagatedKeys(annotations, propagatedLabelsKey, desiredLabels)

			// The address can only be requested while the Service is being created: it is
			// immutable afterwards, and an existing Service always carries one already.
			//
			// Only spec.clusterIP is propagated. spec.clusterIPs is absent from the CRD schema,
			// so the API server prunes it and it is always empty here; carrying it over would
			// also require spec.ipFamilyPolicy and spec.ipFamilies to reach the child Service.
			if childService.Spec.ClusterIP == "" {
				childService.Spec.ClusterIP = serviceWithHC.Spec.ClusterIP
			}

			childService.Spec.Selector = map[string]string{}
			childService.Spec.PublishNotReadyAddresses = serviceWithHC.Spec.PublishNotReadyAddresses

			// The desired values are written, not the literal ones: an update that drops a field
			// the API server fills in would be undone and reissued forever.
			childService.Spec.Ports = desiredPorts(childService, serviceWithHC)
			childService.Spec.Type = desiredServiceType(serviceWithHC)
			childService.Spec.InternalTrafficPolicy = desiredInternalTrafficPolicy(serviceWithHC)
			// ExternalTrafficPolicy is only valid for LoadBalancer and NodePort.
			childService.Spec.ExternalTrafficPolicy = desiredExternalTrafficPolicy(serviceWithHC)
			return nil
		})

		if errUpdatingSvc != nil {
			if errors.IsConflict(errUpdatingSvc) {
				return ctrl.Result{Requeue: true}, nil
			}
			// Record the failure in status condition before returning the error,
			// so the user can see the reason in the resource status.
			originalServiceWithHC := serviceWithHC.DeepCopy()
			patch := client.MergeFrom(originalServiceWithHC)
			failedCondition := createStatusConditionForService(errUpdatingSvc, nil, serviceWithHC.Name)
			failedCondition.ObservedGeneration = serviceWithHC.Generation
			serviceWithHC.Status.Conditions = kubernetes.RemoveStatusCondition(serviceWithHC.Status.Conditions, legacyChildServiceConditionType)
			serviceWithHC.Status.Conditions = kubernetes.UpdateStatusWithCondition(serviceWithHC.Status.Conditions, failedCondition)
			if patchErr := r.Status().Patch(ctx, serviceWithHC, patch); patchErr != nil {
				r.Logger.Error("failed to patch failure condition into status", log.Err(patchErr), "name", req.Name, "namespace", req.Namespace)
			}
			return ctrl.Result{}, fmt.Errorf("failed to create/update child Service for ServiceWithHealthchecks %s/%s: %w", req.Namespace, req.Name, errUpdatingSvc)
		}
		r.Logger.Debug("child Service has been reconciled", "name", req.Name, "namespace", req.Namespace, "operation", op)
	}

	// Nothing watches Nodes, and a Service the module does not own raises no event either, so
	// the reconciliation is repeated on a timer even when everything is settled.
	result := ctrl.Result{RequeueAfter: resyncPeriod}
	if conflictErr != nil {
		result.RequeueAfter = childServiceConflictRetry
	}

	// Always update status — even if the child Service spec didn't change,
	// the status/conditions may need recovery from a previous failed reconciliation.
	originalServiceWithHC := serviceWithHC.DeepCopy()
	patch := client.MergeFrom(originalServiceWithHC)

	if desiredServiceType(serviceWithHC) == corev1.ServiceTypeLoadBalancer {
		r.Logger.Debug("update status for ServiceWithHealthchecks", "name", req.Name, "namespace", req.Namespace)
		serviceWithHC.Status.LoadBalancer = childService.Status.LoadBalancer
	} else {
		serviceWithHC.Status.LoadBalancer = corev1.LoadBalancerStatus{}
	}
	// The address of the child Service is observed state, so it is reported here instead of being
	// written back into the spec of the ServiceWithHealthchecks.
	serviceWithHC.Status.ClusterIP = childService.Spec.ClusterIP
	serviceWithHC.Status.ClusterIPs = childService.Spec.ClusterIPs

	// A Service the module does not manage takes precedence: while it is in the way, the address
	// requested in the spec is not what the reconciliation is stuck on.
	problem := conflictErr
	if problem == nil {
		problem = immutableErr
	}
	newCondition := createStatusConditionForService(errUpdatingSvc, problem, serviceWithHC.Name)
	newCondition.ObservedGeneration = serviceWithHC.Generation
	serviceWithHC.Status.Conditions = kubernetes.RemoveStatusCondition(serviceWithHC.Status.Conditions, legacyChildServiceConditionType)
	serviceWithHC.Status.Conditions = kubernetes.UpdateStatusWithCondition(serviceWithHC.Status.Conditions, newCondition)

	kubernetes.SortConditions(serviceWithHC.Status.Conditions)
	kubernetes.SortConditions(originalServiceWithHC.Status.Conditions)

	if reflect.DeepEqual(originalServiceWithHC.Status, serviceWithHC.Status) {
		return result, nil
	}

	if err := r.Status().Patch(ctx, serviceWithHC, patch); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update ServiceWithHealthchecks Status: %w", err)
	}
	return result, nil
}

// childServiceProblem is something about the child Service that the controller can not fix by
// itself and reports through the status instead.
type childServiceProblem struct {
	reason  string
	message string
}

func (p *childServiceProblem) Error() string { return p.message }

// childServiceConflict reports whether the Service belongs to somebody else. Such a Service is
// never modified and never claimed: the module keeps publishing EndpointSlices for it, but
// rewriting the spec of an object another controller owns, or one a user created for their own
// purpose, would do more damage than the name clash itself.
func childServiceConflict(service *corev1.Service, shc *networkv1alpha1.ServiceWithHealthchecks) *childServiceProblem {
	var cause string
	ref := foreignOwnerReference(service, shc.Name)
	switch {
	case ref != nil:
		cause = fmt.Sprintf("is owned by another resource, %s %q", ref.Kind, ref.Name)
	// A Service the module did not create is adopted only when it already looks exactly like the
	// one the module would have created, so that adoption never changes what the Service does.
	case !metav1.IsControlledBy(service, shc) && !IsSpecForServiceEqual(*service, shc):
		cause = "already existed and its spec differs from the spec of the ServiceWithHealthchecks"
	default:
		return nil
	}
	return &childServiceProblem{
		reason: "ChildServiceConflict",
		message: fmt.Sprintf("the Service %q %s, so it is not managed by this resource: it is left untouched and no owner "+
			"reference is set on it. The EndpointSlices are still published for it, so the healthchecks stay in effect. "+
			"Either delete the Service, or rename the ServiceWithHealthchecks", shc.Name, cause),
	}
}

// clusterIPMismatch reports an address requested in the spec that the child Service does not
// carry. spec.clusterIP is only applied while the Service is being created and is immutable
// afterwards, so the request can never be fulfilled by an update.
func clusterIPMismatch(service *corev1.Service, shc *networkv1alpha1.ServiceWithHealthchecks) *childServiceProblem {
	if shc.Spec.ClusterIP == "" || shc.Spec.ClusterIP == service.Spec.ClusterIP {
		return nil
	}
	return &childServiceProblem{
		reason: "ChildServiceClusterIPImmutable",
		message: fmt.Sprintf("the Service %q has the address %q, while the ServiceWithHealthchecks asks for %q. "+
			"The field is immutable, so the address can only be applied to a newly created Service: delete the Service "+
			"to have it recreated, or drop spec.clusterIP from the ServiceWithHealthchecks. The current address is "+
			"reported in status.clusterIP", shc.Name, service.Spec.ClusterIP, shc.Spec.ClusterIP),
	}
}

// foreignOwnerReference returns the controller reference of the Service when it points to
// something other than the ServiceWithHealthchecks the child Service is built for. Only a
// controller reference means ownership; a plain owner reference is an extra garbage collection
// link that anything may add, and refusing to manage our own Service because of one would be
// worse than the clash it is meant to catch. The UID is not compared: a reference to the same
// name is ours even when the parent was recreated and got a new one.
func foreignOwnerReference(service *corev1.Service, name string) *metav1.OwnerReference {
	ref := metav1.GetControllerOf(service)
	if ref == nil {
		return nil
	}
	gv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil ||
		gv.Group != networkv1alpha1.GroupVersion.Group ||
		ref.Kind != serviceWithHealthchecksKind ||
		ref.Name != name {
		return ref
	}
	return nil
}

func createStatusConditionForService(err error, problem *childServiceProblem, svcName string) metav1.Condition {
	switch {
	case err != nil:
		return metav1.Condition{
			Type:               childServiceConditionType,
			Status:             metav1.ConditionFalse,
			Message:            fmt.Sprintf("can't create child Service \"%s\": %s", svcName, err.Error()),
			Reason:             "ChildServiceWasNotCreated",
			LastTransitionTime: metav1.Now(),
		}
	case problem != nil:
		return metav1.Condition{
			Type:               childServiceConditionType,
			Status:             metav1.ConditionFalse,
			Message:            problem.message,
			Reason:             problem.reason,
			LastTransitionTime: metav1.Now(),
		}
	default:
		return metav1.Condition{
			Type:               childServiceConditionType,
			Status:             metav1.ConditionTrue,
			Message:            "Service was created successfully",
			Reason:             "ChildServiceWasCreated",
			LastTransitionTime: metav1.Now(),
		}
	}
}

func IsSpecForServiceEqual(service corev1.Service, shc *networkv1alpha1.ServiceWithHealthchecks) bool {
	// The child Service has to stay selectorless: with a selector, the EndpointSlice controller
	// of kube-controller-manager publishes its own slices for it, next to the ones the agents
	// write, and kube-proxy load balances over the union of the two — sending traffic to pods
	// that no healthcheck has ever confirmed.
	if len(service.Spec.Selector) != 0 {
		return false
	}
	// Semantic comparison rather than slices.Equal: ServicePort.AppProtocol is a pointer, and ==
	// on a struct compares it by address, so two ports carrying the same appProtocol would never
	// look equal and the Service would be rewritten on every reconciliation.
	if !equality.Semantic.DeepEqual(service.Spec.Ports, desiredPorts(service, shc)) {
		return false
	}
	if service.Spec.PublishNotReadyAddresses != shc.Spec.PublishNotReadyAddresses {
		return false
	}
	if service.Spec.Type != desiredServiceType(shc) {
		return false
	}
	if service.Spec.ExternalTrafficPolicy != desiredExternalTrafficPolicy(shc) {
		return false
	}
	if !reflect.DeepEqual(service.Spec.InternalTrafficPolicy, desiredInternalTrafficPolicy(shc)) {
		return false
	}
	return true
}

// The helpers below answer what the child Service is supposed to look like, not what the parent
// literally says. The API server fills in the fields the parent leaves out, so comparing the raw
// values would report a difference that no update can ever settle, and the controller would
// rewrite the Service on every reconciliation.

// desiredServiceType mirrors the default of the API server for an unset type.
func desiredServiceType(shc *networkv1alpha1.ServiceWithHealthchecks) corev1.ServiceType {
	if shc.Spec.Type == "" {
		return corev1.ServiceTypeClusterIP
	}
	return shc.Spec.Type
}

// desiredInternalTrafficPolicy mirrors the default of the API server, which applies to every type
// that has a ClusterIP.
func desiredInternalTrafficPolicy(shc *networkv1alpha1.ServiceWithHealthchecks) *corev1.ServiceInternalTrafficPolicy {
	if shc.Spec.InternalTrafficPolicy != nil {
		return shc.Spec.InternalTrafficPolicy
	}
	switch desiredServiceType(shc) {
	case corev1.ServiceTypeClusterIP, corev1.ServiceTypeNodePort, corev1.ServiceTypeLoadBalancer:
		policy := corev1.ServiceInternalTrafficPolicyCluster
		return &policy
	}
	return nil
}

// desiredExternalTrafficPolicy mirrors the default of the API server, which only applies to the
// types that have an externally facing address.
func desiredExternalTrafficPolicy(shc *networkv1alpha1.ServiceWithHealthchecks) corev1.ServiceExternalTrafficPolicy {
	if desiredServiceType(shc) != corev1.ServiceTypeLoadBalancer && desiredServiceType(shc) != corev1.ServiceTypeNodePort {
		return ""
	}
	if shc.Spec.ExternalTrafficPolicy != "" {
		return shc.Spec.ExternalTrafficPolicy
	}
	return corev1.ServiceExternalTrafficPolicyCluster
}

// desiredPorts renders the ports of the parent the way the API server stores them on the child
// Service: an unset targetPort means the port itself, and a nodePort the parent does not ask for
// keeps the one already allocated. Sending a zero over an allocated nodePort makes the API server
// hand out a new one, so the port of a load balancer would move on every reconciliation.
func desiredPorts(service corev1.Service, shc *networkv1alpha1.ServiceWithHealthchecks) []corev1.ServicePort {
	allocated := make(map[string]int32, len(service.Spec.Ports))
	for _, port := range service.Spec.Ports {
		allocated[portKey(port)] = port.NodePort
	}

	ports := make([]corev1.ServicePort, 0, len(shc.Spec.Ports))
	for _, port := range shc.Spec.Ports {
		if port.TargetPort == (intstr.IntOrString{}) || port.TargetPort == intstr.FromString("") {
			port.TargetPort = intstr.FromInt32(port.Port)
		}
		if port.NodePort == 0 {
			port.NodePort = allocated[portKey(port)]
		}
		ports = append(ports, port)
	}
	return ports
}

// portKey identifies the same port across the parent and the child. A port that changed its
// number is a different entry, and the API server allocates a new nodePort for it anyway.
func portKey(port corev1.ServicePort) string {
	return fmt.Sprintf("%s/%s/%d", port.Name, port.Protocol, port.Port)
}

// IsMetadataForServiceEqual reports whether the child Service already carries the metadata of the parent.
func IsMetadataForServiceEqual(service corev1.Service, shc *networkv1alpha1.ServiceWithHealthchecks) bool {
	desiredAnnotations := propagatedAnnotations(shc.Annotations)
	desiredLabels := shc.Labels

	// A key dropped from the parent changes the stored list, so removals are caught too.
	if service.Annotations[propagatedAnnotationsKey] != joinKeys(desiredAnnotations) {
		return false
	}
	if service.Annotations[propagatedLabelsKey] != joinKeys(desiredLabels) {
		return false
	}
	return isSubset(desiredAnnotations, service.Annotations) && isSubset(desiredLabels, service.Labels)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceWithHealthchecksReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: 5}).
		For(&networkv1alpha1.ServiceWithHealthchecks{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

func (r *ServiceWithHealthchecksReconciler) clearNotUsedEPS(ctx context.Context, req ctrl.Request) int {
	nodeList := &corev1.NodeList{}
	err := r.List(ctx, nodeList)
	if err != nil {
		r.Logger.Error("failed to list Nodes", log.Err(err))
		return 0
	}

	swhList := &networkv1alpha1.ServiceWithHealthchecksList{}
	err = r.List(ctx, swhList, client.InNamespace(req.Namespace))
	if err != nil {
		r.Logger.Error("failed to list Services", log.Err(err))
		return 0
	}

	epsList := &discoveryv1.EndpointSliceList{}
	err = r.List(ctx, epsList, client.InNamespace(req.Namespace), client.MatchingLabels{endpointControllerLabelKey: controllerName})
	if err != nil {
		r.Logger.Error("failed to list EndpointSlices", log.Err(err))
		return 0
	}

	possibleEPSNames := genAllPossibleNames(swhList, nodeList)
	// existingNodesNames := getExistingNodesNames(nodeList)
	// serviceNames := getServiceNames(serviceList)
	deletedCount := 0
	for _, eps := range epsList.Items {
		if _, exists := possibleEPSNames[eps.Name]; exists {
			// skip if name is in the possible names list
			continue
		}

		err := r.Delete(ctx, &eps)
		if err != nil {
			r.Logger.Error("failed to delete EndpointSlice", log.Err(err), "name", eps.Name, "namespace", eps.Namespace)
		} else {
			r.Logger.Debug("deleted EndpointSlice", "name", eps.Name, "namespace", eps.Namespace)
			deletedCount++
		}
	}
	return deletedCount
}

func genAllPossibleNames(swhc *networkv1alpha1.ServiceWithHealthchecksList, nodes *corev1.NodeList) map[string]struct{} {
	result := make(map[string]struct{})
	for _, swh := range swhc.Items {
		for _, node := range nodes.Items {
			result[swh.Name+"-"+node.Name] = struct{}{}
		}
	}
	return result
}
