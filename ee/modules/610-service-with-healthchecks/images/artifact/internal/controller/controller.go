/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package controller

import (
	"context"
	"fmt"
	"reflect"
	"slices"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkv1alpha1 "service-with-healthchecks/api/v1alpha1"
	"service-with-healthchecks/internal/kubernetes"
)

const (
	endpointControllerLabelKey = "endpointslice.kubernetes.io/managed-by"
	controllerName             = "servicewithhealthchecks"
)

// ServiceWithHealthchecksReconciler reconciles a ServiceWithHealthchecks object
type ServiceWithHealthchecksReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Logger logr.Logger
}

// +kubebuilder:rbac:groups=network.deckhouse.io,resources=servicewithhealthchecks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.deckhouse.io,resources=servicewithhealthchecks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=network.deckhouse.io,resources=servicewithhealthchecks/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ServiceWithHealthchecksReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Logger.V(1).Info("reconciling ServiceWithHealthchecks", "name", req.Name, "namespace", req.Namespace)
	serviceWithHC := &networkv1alpha1.ServiceWithHealthchecks{}
	if err := r.Get(ctx, req.NamespacedName, serviceWithHC); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		r.Logger.Error(err, "failed to reconcile ServiceWithHealthchecks", "name", req.Name, "namespace", req.Namespace)
		return ctrl.Result{}, err
	}

	// clear EPS for disappeared Nodes
	deletedCount := r.clearNotUsedEPS(ctx, req)
	r.Logger.V(1).Info("deleted orphan EndpointSlices", "namespace", req.Namespace, "count", deletedCount)

	// create or update child service (skip if spec already matches)
	var childService corev1.Service
	childService.Name = req.Name
	childService.Namespace = req.Namespace

	var errUpdatingSvc error
	var service corev1.Service
	err := r.Get(ctx, req.NamespacedName, &service)
	if err == nil && IsSpecForServiceEqual(service, serviceWithHC) {
		r.Logger.V(1).Info("no need to update child Service", "name", req.Name, "namespace", req.Namespace)
		childService = service
	} else {
		var op controllerutil.OperationResult
		op, errUpdatingSvc = controllerutil.CreateOrUpdate(ctx, r.Client, &childService, func() error {
			// Ensure owner reference is always set (idempotent — restores it if accidentally removed)
			if err := controllerutil.SetControllerReference(serviceWithHC, &childService, r.Scheme); err != nil {
				return err
			}

			childService.Spec.Selector = map[string]string{}
			childService.Spec.Ports = serviceWithHC.Spec.Ports
			childService.Spec.Type = serviceWithHC.Spec.Type
			childService.Spec.PublishNotReadyAddresses = serviceWithHC.Spec.PublishNotReadyAddresses
			childService.Spec.InternalTrafficPolicy = serviceWithHC.Spec.InternalTrafficPolicy

			// ExternalTrafficPolicy is only valid for LoadBalancer and NodePort.
			if serviceWithHC.Spec.Type == corev1.ServiceTypeLoadBalancer || serviceWithHC.Spec.Type == corev1.ServiceTypeNodePort {
				childService.Spec.ExternalTrafficPolicy = serviceWithHC.Spec.ExternalTrafficPolicy
			} else {
				childService.Spec.ExternalTrafficPolicy = ""
			}
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
			failedCondition := createStatusConditionForService(errUpdatingSvc, serviceWithHC.Name)
			failedCondition.ObservedGeneration = serviceWithHC.Generation
			serviceWithHC.Status.Conditions = kubernetes.UpdateStatusWithCondition(serviceWithHC.Status.Conditions, failedCondition)
			if patchErr := r.Status().Patch(ctx, serviceWithHC, patch); patchErr != nil {
				r.Logger.Error(patchErr, "failed to patch failure condition into status", "name", req.Name, "namespace", req.Namespace)
			}
			return ctrl.Result{}, fmt.Errorf("failed to create/update child Service for ServiceWithHealthchecks %s/%s: %w", req.Namespace, req.Name, errUpdatingSvc)
		}
		r.Logger.V(1).Info("child Service has been reconciled", "name", req.Name, "namespace", req.Namespace, "operation", op)
	}

	// Always update status — even if the child Service spec didn't change,
	// the status/conditions may need recovery from a previous failed reconciliation.
	originalServiceWithHC := serviceWithHC.DeepCopy()
	patch := client.MergeFrom(originalServiceWithHC)

	if serviceWithHC.Spec.Type == corev1.ServiceTypeLoadBalancer {
		r.Logger.V(1).Info("update status for ServiceWithHealthchecks", "name", req.Name, "namespace", req.Namespace)
		serviceWithHC.Status.LoadBalancer = childService.Status.LoadBalancer
	} else {
		serviceWithHC.Status.LoadBalancer = corev1.LoadBalancerStatus{}
	}
	newCondition := createStatusConditionForService(errUpdatingSvc, serviceWithHC.Name)
	newCondition.ObservedGeneration = serviceWithHC.Generation
	serviceWithHC.Status.Conditions = kubernetes.UpdateStatusWithCondition(serviceWithHC.Status.Conditions, newCondition)

	kubernetes.SortConditions(serviceWithHC.Status.Conditions)
	kubernetes.SortConditions(originalServiceWithHC.Status.Conditions)

	if reflect.DeepEqual(originalServiceWithHC.Status, serviceWithHC.Status) {
		return ctrl.Result{}, nil
	}

	if err := r.Status().Patch(ctx, serviceWithHC, patch); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update ServiceWithHealthchecks Status: %w", err)
	}
	return ctrl.Result{}, nil
}

func createStatusConditionForService(err error, svcName string) metav1.Condition {
	if err != nil {
		return metav1.Condition{
			Type:               "ChildService",
			Status:             metav1.ConditionFalse,
			Message:            fmt.Sprintf("can't create child Service \"%s\": %s", svcName, err.Error()),
			Reason:             "ChildServiceWasNotCreated",
			LastTransitionTime: metav1.Now(),
		}
	}
	return metav1.Condition{
		Type:               "ChildService",
		Status:             metav1.ConditionTrue,
		Message:            "Service was created successfully",
		Reason:             "ChildServiceWasCreated",
		LastTransitionTime: metav1.Now(),
	}
}

func IsSpecForServiceEqual(service corev1.Service, shc *networkv1alpha1.ServiceWithHealthchecks) bool {
	if !slices.Equal(service.Spec.Ports, shc.Spec.Ports) {
		return false
	}
	if service.Spec.PublishNotReadyAddresses != shc.Spec.PublishNotReadyAddresses {
		return false
	}
	if service.Spec.Type != shc.Spec.Type {
		return false
	}
	if service.Spec.ExternalTrafficPolicy != shc.Spec.ExternalTrafficPolicy {
		return false
	}
	if *service.Spec.InternalTrafficPolicy != *shc.Spec.InternalTrafficPolicy {
		return false
	}
	return true
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
		r.Logger.Error(err, "failed to list Nodes")
		return 0
	}

	swhList := &networkv1alpha1.ServiceWithHealthchecksList{}
	err = r.List(ctx, swhList, client.InNamespace(req.Namespace))
	if err != nil {
		r.Logger.Error(err, "failed to list Services")
		return 0
	}

	epsList := &discoveryv1.EndpointSliceList{}
	err = r.List(ctx, epsList, client.InNamespace(req.Namespace), client.MatchingLabels{endpointControllerLabelKey: controllerName})
	if err != nil {
		r.Logger.Error(err, "failed to list EndpointSlices")
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
			r.Logger.Error(err, "failed to delete EndpointSlice", "name", eps.Name, "namespace", eps.Namespace)
		} else {
			r.Logger.V(1).Info("deleted EndpointSlice", "name", eps.Name, "namespace", eps.Namespace)
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
