/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package controller

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"infra-controller-manager/api/v1alpha1"
	"infra-controller-manager/internal/vcd"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/vmware/go-vcloud-director/v2/types/v56"
)

// VCDAffinityRuleReconciler reconciles a VCDAffinityRule object
type VCDAffinityRuleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Logger *log.Logger
	Config *vcd.Config
}

var (
	finalizer = "vcdaffinityrule.deckhouse.io"
)

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

	// Add finalizer first if not exist to avoid the race condition between init and delete
	if !controllerutil.ContainsFinalizer(vcdaffinityrule, finalizer) {
		controllerutil.AddFinalizer(vcdaffinityrule, finalizer)
		return ctrl.Result{RequeueAfter: 0}, nil
	}

	var nodes corev1.NodeList
	if err := r.Client.List(ctx, &nodes, client.MatchingLabels(vcdaffinityrule.Spec.NodeLabelSelector)); err != nil {
		r.Logger.Error("failed to list nodes for node group", slog.String("error", err.Error()))
		return ctrl.Result{}, err
	}

	nodeStatus := make([]v1alpha1.VCDAffinityRuleStatusNode, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		nodeStatus = append(nodeStatus, v1alpha1.VCDAffinityRuleStatusNode{
			Name: node.Name,
			ID:   filepath.Base(node.Spec.ProviderID),
		})
	}
	vcdaffinityrule.Status.Nodes = nodeStatus

	vdcClient, err := r.Config.NewVDCClient()
	if err != nil {
		r.Logger.Error("failed to create vdc client", slog.String("error", err.Error()))
		return ctrl.Result{}, err
	}

	vappClient, err := r.Config.NewVAppClientFromVDCClient(vdcClient)
	if err != nil {
		r.Logger.Error("failed to create vapp client", slog.String("error", err.Error()))
		return ctrl.Result{}, err
	}

	if !vcdaffinityrule.DeletionTimestamp.IsZero() {
		if vcdaffinityrule.Status.RuleID != "" {
			r.Logger.Info("deleting affinity rule in VCD", slog.String("ruleID", vcdaffinityrule.Status.RuleID))
			vmAffinityRule, err := vdcClient.GetVmAffinityRuleById(vcdaffinityrule.Status.RuleID)
			if err != nil {
				r.Logger.Error("failed to get vm affinity rule by id", slog.String("error", err.Error()))
			}

			err = vmAffinityRule.Delete()
			if err != nil {
				r.Logger.Error("failed to delete vm affinity rule", slog.String("error", err.Error()))
			}

			controllerutil.RemoveFinalizer(vcdaffinityrule, finalizer)
			return ctrl.Result{}, nil
		} else {
			r.Logger.Warn("no ruleID in status, trying to find rule by name and polarity")

			vmAffinityRules, err := vdcClient.GetVmAffinityRulesByName(vcdaffinityrule.GetName(), vcdaffinityrule.Spec.Polarity)
			if err != nil {
				r.Logger.Error("failed to get vm affinity rule by name and polarity", slog.String("error", err.Error()))
			}

			if len(vmAffinityRules) == 0 {
				r.Logger.Warn("no affinity rule found, nothing to delete")
				return ctrl.Result{}, nil
			} else if len(vmAffinityRules) > 1 {
				r.Logger.Warn("multiple affinity rules found with same name and polarity, unable to determine which to delete")
				return ctrl.Result{}, nil
			} else {
				err := vmAffinityRules[0].Delete()
				if err != nil {
					r.Logger.Error("failed to delete vm affinity rule", slog.String("error", err.Error()))
					return ctrl.Result{}, err
				}

				controllerutil.RemoveFinalizer(vcdaffinityrule, finalizer)
				return ctrl.Result{}, nil
			}

		}

	} else {
		if len(nodes.Items) < 2 {
			vcdaffinityrule.Status.Message = "Not enough nodes for building an affinity rule"
			r.Status().Update(ctx, vcdaffinityrule)
			return ctrl.Result{}, err
		} else {
			vmReference := make([]*types.Reference, len(nodes.Items))

			for _, node := range nodeStatus {
				r.Logger.With(slog.String("node", node.Name), slog.String("providerID", node.ID))

				vm, err := vappClient.GetVMById(node.ID, false)
				if err != nil {
					r.Logger.Error("failed to get vm by id", slog.String("error", err.Error()))
					return ctrl.Result{}, err
				}

				vmReference = append(vmReference, &types.Reference{
					HREF: vm.VM.HREF,
					Name: vm.VM.Name,
					ID:   vm.VM.ID,
					Type: vm.VM.Type,
				})
			}

			vmReferences := make([]*types.VMs, 1)
			vmReferences[0] = &types.VMs{
				VMReference: vmReference,
			}

			ruleEnabled := true
			ruleMandatory := false

			vmAffinityRuleDefinition := &types.VmAffinityRule{
				Name:         vcdaffinityrule.GetName(),
				Polarity:     vcdaffinityrule.Spec.Polarity,
				IsEnabled:    &ruleEnabled,
				IsMandatory:  &ruleMandatory,
				VmReferences: vmReferences,
			}

			vmAffinityRule, err := vdcClient.CreateVmAffinityRule(vmAffinityRuleDefinition)
			if err != nil {
				r.Logger.Error("failed to create vm affinity rule", slog.String("error", err.Error()))
				vcdaffinityrule.Status.Message = fmt.Sprintf("Failed to create affinity rule in VCD: %s", err.Error())
				r.Status().Update(ctx, vcdaffinityrule)
				return ctrl.Result{}, err
			}

			vcdaffinityrule.Status.RuleID = vmAffinityRule.VmAffinityRule.ID
			vcdaffinityrule.Status.Message = "Ok"
		}
	}
	r.Status().Update(ctx, vcdaffinityrule)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VCDAffinityRuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.VCDAffinityRule{}).
		WithEventFilter(predicate.Or(
			predicate.GenerationChangedPredicate{},
			predicate.AnnotationChangedPredicate{},
			predicate.LabelChangedPredicate{},
		)).
		Named("vcdaffinityrule").
		Complete(r)
}
