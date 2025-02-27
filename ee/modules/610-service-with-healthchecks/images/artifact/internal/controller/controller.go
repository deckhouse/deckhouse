/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package controller

import (
	"context"
	"fmt"
	networkv1alpha1 "service-with-healthchecks/api/v1alpha1"
	"service-with-healthchecks/internal/kubernetes"
	"slices"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	endpointControllerLabelKey = "endpointslice.kubernetes.io/managed-by"
	controllerName             = "servicewithhealthchecks"
)

var gvk = schema.GroupVersionKind{Group: "network.deckhouse.io", Version: "v1alpha1", Kind: "ServiceWithHealthchecks"}

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
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ServiceWithHealthchecks object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.2/pkg/reconcile
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

	var service corev1.Service
	err := r.Get(ctx, req.NamespacedName, &service)
	if err == nil && IsSpecForServiceEqual(service, serviceWithHC) {
		r.Logger.V(1).Info("no need to update child Service", "name", req.Name, "namespace", req.Namespace)
		return ctrl.Result{}, nil
	}

	// create or update child service
	var childService corev1.Service
	if err == nil {
		r.Logger.V(1).Info("updating existing child Service", "name", req.Name, "namespace", req.Namespace)
		childService = *service.DeepCopy()
	} else {
		r.Logger.V(1).Info("creating child Service", "name", req.Name, "namespace", req.Namespace)
		childService = corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      req.Name,
				Namespace: req.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(serviceWithHC, gvk),
				},
			},
		}
	}
	op, errUpdatingSvc := controllerutil.CreateOrUpdate(context.TODO(), r.Client, &childService, func() error {
		childService.Spec = corev1.ServiceSpec{
			Selector:                 map[string]string{},
			PublishNotReadyAddresses: service.Spec.PublishNotReadyAddresses,
			Ports:                    serviceWithHC.Spec.Ports,
			Type:                     serviceWithHC.Spec.Type,
			InternalTrafficPolicy:    serviceWithHC.Spec.InternalTrafficPolicy,
			ExternalTrafficPolicy:    serviceWithHC.Spec.ExternalTrafficPolicy,
		}
		return nil
	})
	if errUpdatingSvc != nil {
		r.Logger.Error(errUpdatingSvc, "failed to create/update child Service for ServiceWithHealthchecks", "name", req.Name, "namespace", req.Namespace)
	}
	r.Logger.V(1).Info("child Service has been reconciled", "name", req.Name, "namespace", req.Namespace, "operation", op)

	patch := client.MergeFrom(serviceWithHC.DeepCopy())

	// update ServiceWithHealthchecks Status
	if serviceWithHC.Spec.Type == corev1.ServiceTypeLoadBalancer {
		r.Logger.V(1).Info("update status for ServiceWithHealtchecks", "name", req.Name, "namespace", req.Namespace, "operation", op)
		err := r.Get(ctx, req.NamespacedName, &service)
		if err != nil {
			r.Logger.Error(err, "failed to get child Service for ServiceWithHealthchecks", "name", req.Name, "namespace", req.Namespace)
			return ctrl.Result{}, nil
		}
		serviceWithHC.Status.LoadBalancer = childService.Status.LoadBalancer
	}
	newCondition := createStatusConditionForService(errUpdatingSvc, serviceWithHC.Name)
	serviceWithHC.Status.Conditions = kubernetes.UpdateStatusWithCondition(serviceWithHC.Status.Conditions, newCondition)
	if err := r.Status().Patch(ctx, serviceWithHC, patch); err != nil {
		r.Logger.Error(err, "failed to update ServiceWithHealthchecks Status", "name", req.Name, "namespace", req.Namespace)
		return ctrl.Result{}, nil
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

func genAllPossibleNames(swhs *networkv1alpha1.ServiceWithHealthchecksList, nodes *corev1.NodeList) map[string]struct{} {
	result := make(map[string]struct{})
	for _, swh := range swhs.Items {
		for _, node := range nodes.Items {
			result[swh.Name+"-"+node.Name] = struct{}{}
		}
	}
	return result
}
