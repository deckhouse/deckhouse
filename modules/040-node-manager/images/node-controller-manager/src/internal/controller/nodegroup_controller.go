/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/event"
	"github.com/deckhouse/node-controller/internal/scope"
)

const (
	// RequeueInterval is the default interval for requeueing
	RequeueInterval = 30 * time.Second

	// NodeGroupLabel is the label that indicates which NodeGroup a node belongs to
	NodeGroupLabel = "node.deckhouse.io/group"

	// NodeGroupFinalizer is the finalizer for NodeGroup
	NodeGroupFinalizer = "nodegroup.deckhouse.io/finalizer"
)

// NodeGroupReconciler reconciles a NodeGroup object
type NodeGroupReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Config   *rest.Config
	Recorder *event.Recorder
}

// +kubebuilder:rbac:groups=deckhouse.io,resources=nodegroups,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=deckhouse.io,resources=nodegroups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=deckhouse.io,resources=nodegroups/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete

// Reconcile reconciles the NodeGroup object.
func (r *NodeGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	logger := ctrl.LoggerFrom(ctx).WithValues("nodeGroup", req.Name)
	ctx = ctrl.LoggerInto(ctx, logger)

	logger.V(1).Info("Reconciling NodeGroup")

	// Fetch the NodeGroup
	nodeGroup := &v1.NodeGroup{}
	err = r.Get(ctx, req.NamespacedName, nodeGroup)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(1).Info("NodeGroup not found, probably deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to get NodeGroup")
		return ctrl.Result{}, err
	}

	// Create scope
	baseScope, err := scope.NewScope(r.Client, r.Config, logger)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to create base scope")
	}

	nodeGroupScope, err := scope.NewNodeGroupScope(baseScope, nodeGroup, ctx)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to create nodegroup scope")
	}
	defer func() {
		closeErr := nodeGroupScope.Close(ctx)
		if closeErr != nil {
			logger.Error(closeErr, "failed to close nodegroup scope")
		}
	}()

	// Load nodes belonging to this NodeGroup
	if err := nodeGroupScope.LoadNodes(ctx); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to load nodes")
	}

	// Handle deleted NodeGroup
	if !nodeGroup.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.V(1).Info("Reconciling delete NodeGroup")
		return r.reconcileDelete(ctx, nodeGroupScope)
	}

	result, reconcileErr := r.reconcileNormal(ctx, nodeGroupScope)
	if reconcileErr != nil {
		logger.Error(reconcileErr, "failed to reconcile NodeGroup")
	}

	return result, reconcileErr
}

func (r *NodeGroupReconciler) reconcileNormal(
	ctx context.Context,
	nodeGroupScope *scope.NodeGroupScope,
) (ctrl.Result, error) {
	nodeGroupScope.Logger.V(1).Info("Reconciling NodeGroup normal")

	// Reconcile each node
	for i := range nodeGroupScope.Nodes {
		node := &nodeGroupScope.Nodes[i]
		if err := r.reconcileNode(ctx, nodeGroupScope, node); err != nil {
			nodeGroupScope.Logger.Error(err, "failed to reconcile node", "node", node.Name)
			// Continue with other nodes
		}
	}

	// Update NodeGroup status
	nodeGroupScope.UpdateStatus()

	// Set Ready condition
	if nodeGroupScope.NodeGroup.Status.Ready == nodeGroupScope.NodeGroup.Status.Nodes &&
		nodeGroupScope.NodeGroup.Status.Nodes > 0 {
		nodeGroupScope.SetCondition("Ready", metav1.ConditionTrue, "AllNodesReady", "All nodes are ready")
	} else if nodeGroupScope.NodeGroup.Status.Nodes == 0 {
		nodeGroupScope.SetCondition("Ready", metav1.ConditionFalse, "NoNodes", "No nodes in group")
	} else {
		nodeGroupScope.SetCondition("Ready", metav1.ConditionFalse, "NodesNotReady", "Some nodes are not ready")
	}

	// Requeue after interval
	return ctrl.Result{RequeueAfter: RequeueInterval}, nil
}

func (r *NodeGroupReconciler) reconcileNode(
	ctx context.Context,
	nodeGroupScope *scope.NodeGroupScope,
	node *corev1.Node,
) error {
	ng := nodeGroupScope.NodeGroup

	if ng.Spec.NodeTemplate == nil {
		return nil
	}

	// Create a copy for safe modification
	nodeCopy := node.DeepCopy()
	needsUpdate := false

	// Initialize maps if nil
	if nodeCopy.Labels == nil {
		nodeCopy.Labels = make(map[string]string)
	}
	if nodeCopy.Annotations == nil {
		nodeCopy.Annotations = make(map[string]string)
	}

	// Apply labels from nodeTemplate
	for key, value := range ng.Spec.NodeTemplate.Labels {
		if nodeCopy.Labels[key] != value {
			nodeCopy.Labels[key] = value
			needsUpdate = true
			nodeGroupScope.Logger.V(1).Info("updating label", "node", node.Name, "key", key, "value", value)
		}
	}

	// Apply annotations from nodeTemplate
	for key, value := range ng.Spec.NodeTemplate.Annotations {
		if nodeCopy.Annotations[key] != value {
			nodeCopy.Annotations[key] = value
			needsUpdate = true
			nodeGroupScope.Logger.V(1).Info("updating annotation", "node", node.Name, "key", key, "value", value)
		}
	}

	// Apply taints from nodeTemplate
	if ng.Spec.NodeTemplate.Taints != nil {
		taintsChanged := r.reconcileTaints(nodeCopy, ng.Spec.NodeTemplate.Taints)
		if taintsChanged {
			needsUpdate = true
			nodeGroupScope.Logger.V(1).Info("updating taints", "node", node.Name)
		}
	}

	// Update node if needed
	if needsUpdate {
		if err := r.Update(ctx, nodeCopy); err != nil {
			return errors.Wrapf(err, "failed to update node %s", node.Name)
		}
		nodeGroupScope.Logger.Info("node updated", "node", node.Name)

		// Record event
		if r.Recorder != nil {
			r.Recorder.SendNormalEvent(nodeCopy, ng.Name, "NodeUpdated", "Node configuration updated")
		}
	}

	return nil
}

func (r *NodeGroupReconciler) reconcileTaints(node *corev1.Node, desiredTaints []corev1.Taint) bool {
	// Build map of desired taints
	desiredTaintsMap := make(map[string]corev1.Taint)
	for _, taint := range desiredTaints {
		key := taint.Key + ":" + string(taint.Effect)
		desiredTaintsMap[key] = taint
	}

	// Build map of current taints
	currentTaintsMap := make(map[string]corev1.Taint)
	for _, taint := range node.Spec.Taints {
		key := taint.Key + ":" + string(taint.Effect)
		currentTaintsMap[key] = taint
	}

	changed := false

	// Add missing taints
	for key, taint := range desiredTaintsMap {
		if _, exists := currentTaintsMap[key]; !exists {
			node.Spec.Taints = append(node.Spec.Taints, taint)
			changed = true
		} else if currentTaintsMap[key].Value != taint.Value {
			// Update taint value if changed
			for i := range node.Spec.Taints {
				if node.Spec.Taints[i].Key == taint.Key && node.Spec.Taints[i].Effect == taint.Effect {
					node.Spec.Taints[i].Value = taint.Value
					changed = true
					break
				}
			}
		}
	}

	return changed
}

func (r *NodeGroupReconciler) reconcileDelete(
	ctx context.Context,
	nodeGroupScope *scope.NodeGroupScope,
) (ctrl.Result, error) {
	nodeGroupScope.Logger.V(1).Info("Reconciling NodeGroup deletion")

	// Check if there are any nodes
	if len(nodeGroupScope.Nodes) > 0 {
		nodeGroupScope.Logger.Info("cannot delete NodeGroup with active nodes", "nodeCount", len(nodeGroupScope.Nodes))
		// Don't block deletion, just warn
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeGroupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.NodeGroup{}).
		Watches(
			&corev1.Node{},
			handler.EnqueueRequestsFromMapFunc(r.nodeToNodeGroupMapFunc()),
		).
		Complete(r)
}

// nodeToNodeGroupMapFunc returns a handler that maps Node events to NodeGroup reconcile requests.
func (r *NodeGroupReconciler) nodeToNodeGroupMapFunc() handler.MapFunc {
	return func(ctx context.Context, object client.Object) []reconcile.Request {
		node, ok := object.(*corev1.Node)
		if !ok {
			return nil
		}

		// Get the NodeGroup name from the node label
		nodeGroupName, exists := node.Labels[NodeGroupLabel]
		if !exists {
			return nil
		}

		return []reconcile.Request{
			{
				NamespacedName: client.ObjectKey{
					Name: nodeGroupName,
				},
			},
		}
	}
}
