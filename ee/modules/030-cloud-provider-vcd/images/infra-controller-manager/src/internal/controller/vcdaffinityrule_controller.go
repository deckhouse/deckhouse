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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"infra-controller-manager/api/v1alpha1"
	"infra-controller-manager/internal/vcd"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/vmware/go-vcloud-director/v2/govcd"
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
	finalizer = "vcdaffinityrule.deckhouse.io/finalizer"
)

// +kubebuilder:rbac:groups=deckhouse.io,resources=vcdaffinityrules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=deckhouse.io,resources=vcdaffinityrules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=deckhouse.io,resources=vcdaffinityrules/finalizers,verbs=update

// SetupWithManager sets up the controller with the Manager.
func (r *VCDAffinityRuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	nodePredicate := predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool {
			return true
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldNode := e.ObjectOld.(*corev1.Node)
			newNode := e.ObjectNew.(*corev1.Node)
			return oldNode.Spec.ProviderID != newNode.Spec.ProviderID
		},
	}

	enqueueOnlyRelevantResources := func(ctx context.Context, a client.Object) []reconcile.Request {
		node, ok := a.(*corev1.Node)
		if !ok {
			return nil
		}

		var vcdAffinityRulesList v1alpha1.VCDAffinityRuleList
		var requests []reconcile.Request

		if err := r.Client.List(ctx, &vcdAffinityRulesList); err != nil {
			return nil
		}

		for _, item := range vcdAffinityRulesList.Items {
			nodeLabelSelector, err := metav1.LabelSelectorAsSelector(&item.Spec.NodeLabelSelector)
			if err != nil {
				continue
			}

			if satisfiesNodeSelector(node, nodeLabelSelector) {
				requests = append(requests, reconcile.Request{
					NamespacedName: client.ObjectKey{
						Namespace: item.Namespace,
						Name:      item.Name,
					},
				})
			}
		}
		return requests
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.VCDAffinityRule{}).
		WithEventFilter(predicate.Or(
			predicate.GenerationChangedPredicate{},
			predicate.AnnotationChangedPredicate{},
			predicate.LabelChangedPredicate{},
		)).
		Named("vcdaffinityrule").
		Watches(
			&corev1.Node{},
			handler.EnqueueRequestsFromMapFunc(enqueueOnlyRelevantResources),
			builder.WithPredicates(
				predicate.Or(
					predicate.LabelChangedPredicate{},
					nodePredicate,
				)),
		).
		Complete(r)
}

func satisfiesNodeSelector(node *corev1.Node, nodeSelector labels.Selector) bool {
	nodeLabels := node.GetLabels()
	return nodeSelector.Matches(labels.Set(nodeLabels))
}

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

	vcdAffinityRule := &v1alpha1.VCDAffinityRule{}
	if err := r.Client.Get(ctx, req.NamespacedName, vcdAffinityRule); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Add finalizer first if not exist to avoid the race condition between init and delete
	if !controllerutil.ContainsFinalizer(vcdAffinityRule, finalizer) && vcdAffinityRule.DeletionTimestamp.IsZero() {
		r.Logger.Info("adding finalizer")

		controllerutil.AddFinalizer(vcdAffinityRule, finalizer)

		if err := r.Update(ctx, vcdAffinityRule); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update vcdaffinityrule with finalizer: %w", err)
		}

		return ctrl.Result{RequeueAfter: 1}, nil
	}

	var nodes corev1.NodeList
	nodeLabelSelector, err := metav1.LabelSelectorAsSelector(&vcdAffinityRule.Spec.NodeLabelSelector)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to parse node label selector: %w", err)
	}

	if err := r.Client.List(ctx, &nodes, client.MatchingLabelsSelector{Selector: nodeLabelSelector}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list nodes: %w", err)
	}

	nodeStatus := make([]v1alpha1.VCDAffinityRuleStatusNode, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		id := ""
		if node.Spec.ProviderID != "" {
			id = filepath.Base(node.Spec.ProviderID)
		}

		nodeStatus = append(nodeStatus, v1alpha1.VCDAffinityRuleStatusNode{
			Name: node.Name,
			ID:   id,
		})
	}

	vcdAffinityRule.Status.Nodes = nodeStatus
	vcdAffinityRule.Status.NodeCount = len(nodeStatus)

	vdcClient, err := r.Config.NewVDCClient()
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create vdc client: %w", err)
	}

	vappClient, err := r.Config.NewVAppClientFromVDCClient(vdcClient)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create vapp client: %w", err)
	}

	if !vcdAffinityRule.DeletionTimestamp.IsZero() && controllerutil.ContainsFinalizer(vcdAffinityRule, finalizer) {
		err := r.deleteVMAffinityRule(ctx, vcdAffinityRule, vdcClient)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to delete vm affinity rule: %w", err)
		}

		controllerutil.RemoveFinalizer(vcdAffinityRule, finalizer)
		if err := r.Update(ctx, vcdAffinityRule); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to remove finalizer from vcdaffinityrule: %w", err)
		}

		return ctrl.Result{}, nil
	}

	if len(vcdAffinityRule.Status.Nodes) < 2 {
		vcdAffinityRule.Status.Message = "Not enough nodes to build an affinity rule"

		if vcdAffinityRule.Status.RuleID != "" {
			r.Logger.Info("deleting affinity rule from VCD API due to insufficient VM count")

			err = r.deleteVMAffinityRule(ctx, vcdAffinityRule, vdcClient)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to delete vm affinity rule: %w", err)
			}
		}

		if err := r.Status().Update(ctx, vcdAffinityRule); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update vcdaffinityrule status: %w", err)
		}

		return ctrl.Result{}, nil
	}

	vmAffinityRuleDefinition, err := r.buildVMAffinityRule(vcdAffinityRule, vappClient)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to build vm affinity rule: %w", err)
	}

	if vcdAffinityRule.Status.RuleID == "" {
		vmAffinityRule, err := vdcClient.CreateVmAffinityRule(vmAffinityRuleDefinition)
		if err != nil {
			vcdAffinityRule.Status.Message = fmt.Sprintf("Failed to create affinity rule in VCD: %s", err.Error())
			if err := r.Status().Update(ctx, vcdAffinityRule); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to update vcdaffinityrule status: %w", err)
			}
			return ctrl.Result{}, fmt.Errorf("failed to create vm affinity rule: %w", err)
		}

		vcdAffinityRule.Status.RuleID = vmAffinityRule.VmAffinityRule.ID
	} else {
		vmAffinityRule, err := vdcClient.GetVmAffinityRuleById(vcdAffinityRule.Status.RuleID)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get vm affinity rule by id: %w", err)
		}

		vmAffinityRule.VmAffinityRule.Name = vmAffinityRuleDefinition.Name
		vmAffinityRule.VmAffinityRule.Polarity = vmAffinityRuleDefinition.Polarity
		vmAffinityRule.VmAffinityRule.IsEnabled = vmAffinityRuleDefinition.IsEnabled
		vmAffinityRule.VmAffinityRule.IsMandatory = vmAffinityRuleDefinition.IsMandatory
		vmAffinityRule.VmAffinityRule.VmReferences = vmAffinityRuleDefinition.VmReferences

		if err = vmAffinityRule.Update(); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update vm affinity rule: %w", err)
		}

		vcdAffinityRule.Status.RuleID = vmAffinityRule.VmAffinityRule.ID
	}

	vcdAffinityRule.Status.Message = "VM affinity rule is up to date"
	if err := r.Status().Update(ctx, vcdAffinityRule); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update vcdaffinityrule status: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *VCDAffinityRuleReconciler) buildVMAffinityRule(resource *v1alpha1.VCDAffinityRule, vapp *govcd.VApp) (*types.VmAffinityRule, error) {
	nodeStatus := resource.Status.Nodes

	vmReference := make([]*types.Reference, len(nodeStatus))

	for _, node := range nodeStatus {
		r.Logger.With(slog.String("node", node.Name), slog.String("ID", node.ID))

		idOrName := node.ID
		if idOrName == "" {
			idOrName = node.Name
			r.Logger.Warn("node has no providerID, using name instead", slog.String("node", node.Name))
		}

		vm, err := vapp.GetVMByNameOrId(idOrName, false)
		if err != nil {
			return nil, fmt.Errorf("failed to get VM %s with ID %s: %w", node.Name, node.ID, err)
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

	polarity := convertPolarity(resource.Spec.Polarity)
	if polarity == "" {
		return nil, fmt.Errorf("invalid polarity: %s", resource.Spec.Polarity)
	}

	return &types.VmAffinityRule{
		Name:         resource.GetName(),
		Polarity:     polarity,
		IsEnabled:    &ruleEnabled,
		IsMandatory:  &resource.Spec.Required,
		VmReferences: vmReferences,
	}, nil
}

func (r *VCDAffinityRuleReconciler) deleteVMAffinityRule(ctx context.Context, resource *v1alpha1.VCDAffinityRule, vdc *govcd.Vdc) error {
	if resource.Status.RuleID != "" {
		r.Logger.Info("deleting affinity rule from VCD API by id", slog.String("ruleID", resource.Status.RuleID))
		vmAffinityRule, err := vdc.GetVmAffinityRuleById(resource.Status.RuleID)

		if err != nil {
			return fmt.Errorf("failed to get vm affinity rule by id: %w", err)
		}

		err = vmAffinityRule.Delete()
		if err != nil {
			return fmt.Errorf("failed to delete vm affinity rule: %w", err)
		}

		resource.Status.RuleID = ""
		if err := r.Status().Update(ctx, resource); err != nil {
			return fmt.Errorf("failed to update vcdaffinityrule status after deletion: %w", err)
		}

		return nil
	}
	r.Logger.Warn("no ruleID in status, trying to find rule by name and polarity")

	vmAffinityRules, err := vdc.GetVmAffinityRulesByName(resource.GetName(), resource.Spec.Polarity)
	if err != nil {
		return fmt.Errorf("failed to get vm affinity rule by name and polarity: %w", err)
	}

	if len(vmAffinityRules) == 0 {
		r.Logger.Warn("no affinity rule found, nothing to delete")
		return nil
	} else if len(vmAffinityRules) > 1 {
		r.Logger.Warn("multiple affinity rules found with same name and polarity, unable to determine which to delete")
		return fmt.Errorf("multiple affinity rules found with same name and polarity, unable to determine which to delete")
	}

	err = vmAffinityRules[0].Delete()
	if err != nil {
		return fmt.Errorf("failed to delete vm affinity rule: %w", err)
	}

	return nil
}

func convertPolarity(polarity string) string {
	switch polarity {
	case "Affinity":
		return "Affinity"
	case "AntiAffinity", "Anti-Affinity":
		return "Anti-Affinity"
	default:
		return ""
	}
}
