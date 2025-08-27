/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package controller

import (
	"context"
	"log/slog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"infra-controller-manager/api/v1alpha1"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// VCDAffinityRuleReconciler reconciles a VCDAffinityRule object
type VCDAffinityRuleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Logger *log.Logger
}

// +kubebuilder:rbac:groups=deckhouse.io,resources=vcdaffinityrules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=deckhouse.io,resources=vcdaffinityrules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=deckhouse.io,resources=vcdaffinityrules/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the VCDAffinityRule object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *VCDAffinityRuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Logger = r.Logger.With(
		slog.String("resource", req.Name),
	)

	r.Logger.Info("starting reconciliation")

	vcdaffinityrule := &v1alpha1.VCDAffinityRule{}
	if err := r.Client.Get(ctx, req.NamespacedName, vcdaffinityrule); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var nodes corev1.NodeList
	if err := r.Client.List(ctx, &nodes, client.MatchingLabels(vcdaffinityrule.Spec.NodeLabelSelector)); err != nil {
		// log.Error(err, "failed to list nodes for node group", "nodeGroup", vcdaffinityrule.Name)
		return ctrl.Result{}, err
	}

	NodeStatus := make([]v1alpha1.VCDAffinityRuleStatusNode, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		NodeStatus = append(NodeStatus, v1alpha1.VCDAffinityRuleStatusNode{
			Name:       node.Name,
			ProviderID: node.Spec.ProviderID,
		})
	}

	if len(nodes.Items) < 2 {
		vcdaffinityrule.Status.Message = "Not enough nodes for building an affinity rule"
	} else {
		vcdaffinityrule.Status.Message = "Ok"
	}

	vcdaffinityrule.Status.Nodes = NodeStatus
	
	r.Status().Update(ctx, vcdaffinityrule)
	
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VCDAffinityRuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.VCDAffinityRule{}).
		Named("vcdaffinityrule").
		Complete(r)
}
