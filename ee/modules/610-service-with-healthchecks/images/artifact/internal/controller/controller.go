/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package controller

import (
	"context"
	"slices"
	"strings"

	networkv1alpha1 "service-with-healthchecks/api/v1alpha1"

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
	r.Logger.Info("reconciling service-with-healthchecks", "name", req.Name, "namespace", req.Namespace)
	serviceWithHC := &networkv1alpha1.ServiceWithHealthchecks{}
	if err := r.Get(ctx, req.NamespacedName, serviceWithHC); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		r.Logger.Error(err, "failed to reconcile service-with-healthchecks", "name", req.Name, "namespace", req.Namespace)
		return ctrl.Result{}, err
	}

	// clear EPS for NotReady nodes
	deletedCount := r.clearNotUsedEPS(ctx, serviceWithHC)
	r.Logger.Info("deleted orphan endpointslices", "namespace", req.Namespace, "count", deletedCount)

	var service corev1.Service
	err := r.Get(ctx, req.NamespacedName, &service)
	if err == nil && SpecForServiceEqual(service, serviceWithHC) {
		r.Logger.Info("no need to update child service", "name", req.Name, "namespace", req.Namespace)
		return ctrl.Result{}, nil
	}

	// create or update child service
	var childService corev1.Service
	if err == nil {
		r.Logger.Info("updating existing child service", "name", req.Name, "namespace", req.Namespace)
		childService = *service.DeepCopy()
	} else {
		r.Logger.Info("creating child service", "name", req.Name, "namespace", req.Namespace)
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
	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, &childService, func() error {
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
	if err != nil {
		r.Logger.Error(err, "failed to create/update child service for service-with-healthchecks", "name", req.Name, "namespace", req.Namespace)
		return ctrl.Result{}, err
	}
	r.Logger.Info("child service has been reconciled", "name", req.Name, "namespace", req.Namespace, "operation", op)

	return ctrl.Result{}, nil
}

func SpecForServiceEqual(service corev1.Service, shc *networkv1alpha1.ServiceWithHealthchecks) bool {
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

func (r *ServiceWithHealthchecksReconciler) clearNotUsedEPS(ctx context.Context, hc *networkv1alpha1.ServiceWithHealthchecks) int {
	nodeList := &corev1.NodeList{}
	err := r.List(ctx, nodeList)
	if err != nil {
		r.Logger.Error(err, "failed to list nodes")
		return 0
	}

	serviceList := &corev1.ServiceList{}
	err = r.List(ctx, serviceList, client.InNamespace(hc.Namespace))
	if err != nil {
		r.Logger.Error(err, "failed to list services")
		return 0
	}

	epsList := &discoveryv1.EndpointSliceList{}
	err = r.List(ctx, epsList, client.InNamespace(hc.Namespace), client.MatchingLabels{endpointControllerLabelKey: controllerName})
	if err != nil {
		r.Logger.Error(err, "failed to list endpointslices")
		return 0
	}

	readyNodesNames := getReadyNodesNames(nodeList)
	serviceNames := getServiceNames(serviceList)
	deletedCount := 0
	for _, eps := range epsList.Items {
		needToBeDeleted := false
		epsNodeName, found := strings.CutPrefix(eps.Name, hc.Name+"-")
		if found && !slices.Contains(readyNodesNames, epsNodeName) {
			// delete EPS for not ready Nodes
			needToBeDeleted = true
		}

		if nameEPSStartWithNonExistingService(eps.Name, serviceNames) {
			// delete EPS for no-existing Service
			needToBeDeleted = true
		}

		if !needToBeDeleted {
			continue
		}

		err := r.Delete(ctx, &eps)
		if err != nil {
			r.Logger.Error(err, "failed to delete endpointslice", "name", eps.Name, "namespace", eps.Namespace)
		} else {
			r.Logger.Info("deleted endpointslice", "name", eps.Name, "namespace", eps.Namespace)
			deletedCount++
		}
	}
	return deletedCount
}

func nameEPSStartWithNonExistingService(epsName string, serviceNames []string) bool {
	for _, serviceName := range serviceNames {
		if strings.HasPrefix(epsName, serviceName) {
			return false
		}
	}
	return true
}

func getServiceNames(list *corev1.ServiceList) []string {
	serviceNames := make([]string, 0, len(list.Items))
	for _, service := range list.Items {
		serviceNames = append(serviceNames, service.Name)
	}
	return serviceNames
}

func getReadyNodesNames(list *corev1.NodeList) []string {
	readyNodesNames := make([]string, 0, len(list.Items))
	for _, node := range list.Items {
		if isNodeReady(&node) {
			readyNodesNames = append(readyNodesNames, node.Name)
		}
	}
	return readyNodesNames
}

func isNodeReady(node *corev1.Node) bool {
	var cond corev1.NodeCondition
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			cond = c
			break
		}
	}
	return cond.Status == corev1.ConditionTrue
}
