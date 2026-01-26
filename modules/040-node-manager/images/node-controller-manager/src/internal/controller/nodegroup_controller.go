package controller

import (
	"context"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	nodev1 "github.com/deckhouse/node-controller/api/v1"
)

const (
	// NodeGroupLabel is the label indicating node's group membership
	NodeGroupLabel = "node.deckhouse.io/group"

	// Default sync interval
	syncInterval = 30 * time.Second

	// Maximum concurrent reconciles
	maxConcurrentReconciles = 3
)

// NodeGroupReconciler reconciles a NodeGroup object
type NodeGroupReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=deckhouse.io,resources=nodegroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=deckhouse.io,resources=nodegroups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=deckhouse.io,resources=nodegroups/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;update;patch

// Reconcile is the main reconciliation loop
func (r *NodeGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Reconciling NodeGroup", "name", req.Name)

	// 1. Get the NodeGroup
	var nodeGroup nodev1.NodeGroup
	if err := r.Get(ctx, req.NamespacedName, &nodeGroup); err != nil {
		// NodeGroup was deleted, nothing to do
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 2. Get all nodes belonging to this NodeGroup
	var nodes corev1.NodeList
	if err := r.List(ctx, &nodes, client.MatchingLabels{
		NodeGroupLabel: nodeGroup.Name,
	}); err != nil {
		logger.Error(err, "Failed to list nodes")
		return ctrl.Result{}, err
	}

	// 3. Reconcile each node
	// TODO: Move hook logic here
	var reconcileErrors []error
	for i := range nodes.Items {
		node := &nodes.Items[i]
		if err := r.reconcileNode(ctx, node, &nodeGroup); err != nil {
			logger.Error(err, "Failed to reconcile node", "node", node.Name)
			reconcileErrors = append(reconcileErrors, err)
		}
	}

	// 4. Update NodeGroup status
	if err := r.updateStatus(ctx, &nodeGroup, nodes.Items); err != nil {
		logger.Error(err, "Failed to update NodeGroup status")
		return ctrl.Result{}, err
	}

	// Periodic resync
	return ctrl.Result{RequeueAfter: syncInterval}, nil
}

// reconcileNode applies NodeGroup configuration to a node
// TODO: This is where hook logic will be moved
func (r *NodeGroupReconciler) reconcileNode(ctx context.Context, node *corev1.Node, ng *nodev1.NodeGroup) error {
	logger := log.FromContext(ctx)

	if ng.Spec.NodeTemplate == nil {
		return nil
	}

	// Check if updates are needed
	needsUpdate := false
	nodeCopy := node.DeepCopy()

	// =========================================
	// TODO: MIGRATE FROM HOOK: set_node_labels
	// =========================================
	if ng.Spec.NodeTemplate.Labels != nil {
		if nodeCopy.Labels == nil {
			nodeCopy.Labels = make(map[string]string)
		}
		for key, value := range ng.Spec.NodeTemplate.Labels {
			if nodeCopy.Labels[key] != value {
				nodeCopy.Labels[key] = value
				needsUpdate = true
			}
		}
	}

	// ===============================================
	// TODO: MIGRATE FROM HOOK: set_node_annotations
	// ===============================================
	if ng.Spec.NodeTemplate.Annotations != nil {
		if nodeCopy.Annotations == nil {
			nodeCopy.Annotations = make(map[string]string)
		}
		for key, value := range ng.Spec.NodeTemplate.Annotations {
			if nodeCopy.Annotations[key] != value {
				nodeCopy.Annotations[key] = value
				needsUpdate = true
			}
		}
	}

	// ==========================================
	// TODO: MIGRATE FROM HOOK: handle_node_taints
	// ==========================================
	if ng.Spec.NodeTemplate.Taints != nil {
		desiredTaints := ng.Spec.NodeTemplate.Taints
		if !taintsEqual(nodeCopy.Spec.Taints, desiredTaints) {
			nodeCopy.Spec.Taints = mergeTaints(nodeCopy.Spec.Taints, desiredTaints, ng.Name)
			needsUpdate = true
		}
	}

	// ==========================================
	// TODO: Add more hook logic migrations here:
	// - node_status
	// - discover_node_group_configuration
	// - handle_node_templates
	// - etc.
	// ==========================================

	// Update node if changes detected
	if needsUpdate {
		logger.Info("Updating node", "node", node.Name)
		if err := r.Update(ctx, nodeCopy); err != nil {
			return err
		}
	}

	return nil
}

// updateStatus updates NodeGroup status based on current nodes
func (r *NodeGroupReconciler) updateStatus(ctx context.Context, ng *nodev1.NodeGroup, nodes []corev1.Node) error {
	// Calculate status
	ng.Status.Nodes = int32(len(nodes))
	ng.Status.Ready = countReadyNodes(nodes)
	ng.Status.UpToDate = countUpToDateNodes(nodes, ng)

	// Set condition summary
	if ng.Status.ConditionSummary == nil {
		ng.Status.ConditionSummary = &nodev1.ConditionSummary{}
	}

	if ng.Status.Ready == ng.Status.Nodes && ng.Status.Nodes > 0 {
		ng.Status.ConditionSummary.Ready = "True"
		ng.Status.ConditionSummary.StatusMessage = "All nodes are ready"
	} else if ng.Status.Nodes == 0 {
		ng.Status.ConditionSummary.Ready = "False"
		ng.Status.ConditionSummary.StatusMessage = "No nodes in group"
	} else {
		ng.Status.ConditionSummary.Ready = "False"
		ng.Status.ConditionSummary.StatusMessage = "Some nodes are not ready"
	}

	// Clear error if everything is OK
	if ng.Status.Ready == ng.Status.Nodes {
		ng.Status.Error = ""
	}

	return r.Status().Update(ctx, ng)
}

// SetupWithManager sets up the controller with the Manager
func (r *NodeGroupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&nodev1.NodeGroup{}).
		// Watch Nodes and map to their NodeGroup
		Watches(
			&corev1.Node{},
			handler.EnqueueRequestsFromMapFunc(r.mapNodeToNodeGroup),
		).
		// Filter to reduce reconciliation frequency
		WithEventFilter(predicate.Or(
			predicate.GenerationChangedPredicate{},
			predicate.LabelChangedPredicate{},
			predicate.AnnotationChangedPredicate{},
		)).
		// Controller options
		WithOptions(controller.Options{
			MaxConcurrentReconciles: maxConcurrentReconciles,
		}).
		Complete(r)
}

// mapNodeToNodeGroup finds the NodeGroup for a given Node
func (r *NodeGroupReconciler) mapNodeToNodeGroup(ctx context.Context, obj client.Object) []reconcile.Request {
	node, ok := obj.(*corev1.Node)
	if !ok {
		return nil
	}

	groupName, exists := node.Labels[NodeGroupLabel]
	if !exists {
		return nil
	}

	return []reconcile.Request{
		{NamespacedName: client.ObjectKey{Name: groupName}},
	}
}

// =====================
// Helper functions
// =====================

func countReadyNodes(nodes []corev1.Node) int32 {
	var ready int32
	for _, node := range nodes {
		if isNodeReady(&node) {
			ready++
		}
	}
	return ready
}

func isNodeReady(node *corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

func countUpToDateNodes(nodes []corev1.Node, ng *nodev1.NodeGroup) int32 {
	// TODO: Implement proper up-to-date checking based on configuration checksum
	// For now, consider all nodes as up-to-date
	return int32(len(nodes))
}

func taintsEqual(a, b []corev1.Taint) bool {
	if len(a) != len(b) {
		return false
	}

	taintMap := make(map[string]corev1.Taint)
	for _, t := range a {
		taintMap[t.Key] = t
	}

	for _, t := range b {
		existing, ok := taintMap[t.Key]
		if !ok {
			return false
		}
		if existing.Value != t.Value || existing.Effect != t.Effect {
			return false
		}
	}

	return true
}

// mergeTaints merges existing taints with desired taints from NodeGroup
// It preserves taints not managed by this NodeGroup
func mergeTaints(existing, desired []corev1.Taint, nodeGroupName string) []corev1.Taint {
	// For now, simple replacement
	// TODO: Implement smarter merge logic that:
	// 1. Tracks which taints are managed by NodeGroup (via annotations)
	// 2. Only modifies managed taints
	// 3. Preserves taints added by other controllers or manually

	result := make([]corev1.Taint, 0, len(desired))
	desiredKeys := make(map[string]bool)

	// Add all desired taints
	for _, t := range desired {
		result = append(result, t)
		desiredKeys[t.Key] = true
	}

	// Keep existing taints not in desired (not managed by NodeGroup)
	for _, t := range existing {
		if !desiredKeys[t.Key] {
			// Check if this taint was previously set by this NodeGroup
			// TODO: Use annotation to track managed taints
			// For now, keep all taints not in desired
			result = append(result, t)
		}
	}

	return result
}

// nodeEqual compares only fields managed by the controller
func nodeEqual(a, b *corev1.Node) bool {
	return reflect.DeepEqual(a.Labels, b.Labels) &&
		reflect.DeepEqual(a.Annotations, b.Annotations) &&
		reflect.DeepEqual(a.Spec.Taints, b.Spec.Taints)
}
