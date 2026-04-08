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

package template

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/dynr"
	"github.com/deckhouse/node-controller/internal/rcname"
)

const (
	nodeGroupNameLabel                = "node.deckhouse.io/group"
	lastAppliedNodeTemplateAnnotation = "node-manager.deckhouse.io/last-applied-node-template"
	nodeUninitializedTaintKey         = "node.deckhouse.io/uninitialized"
	nodeTypeLabel                     = "node.deckhouse.io/type"
	scaleDownDisabledAnnotation       = "cluster-autoscaler.kubernetes.io/scale-down-disabled"
)

func init() {
	dynr.RegisterReconciler(rcname.NodeTemplate, &corev1.Node{}, &Reconciler{})
}

var _ dynr.Reconciler = (*Reconciler)(nil)

// Reconciler syncs labels, annotations and taints from NodeGroup.spec.nodeTemplate
// to the corresponding Nodes. It tracks applied state via the
// "node-manager.deckhouse.io/last-applied-node-template" annotation and removes
// the "node.deckhouse.io/uninitialized" taint once the template is applied.
type Reconciler struct {
	dynr.Base
}

func (r *Reconciler) SetupForPredicates() []predicate.Predicate {
	return []predicate.Predicate{nodeTemplatePredicate()}
}

func (r *Reconciler) SetupWatches(w dynr.Watcher) {
	w.Watches(
		&deckhousev1.NodeGroup{},
		handler.EnqueueRequestsFromMapFunc(r.nodeGroupToNodes),
		builder.WithPredicates(nodeGroupTemplateChangedPredicate()),
	)
}

// nodeGroupToNodes maps a NodeGroup change to reconcile requests for all Nodes
// that belong to that NodeGroup.
func (r *Reconciler) nodeGroupToNodes(ctx context.Context, obj client.Object) []reconcile.Request {
	ng, ok := obj.(*deckhousev1.NodeGroup)
	if !ok {
		return nil
	}

	nodeList := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodeList, client.MatchingLabels{nodeGroupNameLabel: ng.Name}); err != nil {
		logf.FromContext(ctx).Error(err, "failed to list nodes for node group", "nodeGroup", ng.Name)
		return nil
	}

	requests := make([]reconcile.Request, 0, len(nodeList.Items))
	for _, node := range nodeList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: node.Name},
		})
	}
	return requests
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Get the Node.
	node := &corev1.Node{}
	if err := r.Client.Get(ctx, req.NamespacedName, node); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get node %s: %w", req.Name, err)
	}

	// 2. Get the NodeGroup for this node.
	ngName := node.Labels[nodeGroupNameLabel]

	// 3. Get the NodeGroup.
	ng := &deckhousev1.NodeGroup{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: ngName}, ng); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("node group not found, skipping", "nodeGroup", ngName)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get node group %s: %w", ngName, err)
	}

	// 4. Determine desired template values.
	var desiredLabels, desiredAnnotations map[string]string
	var desiredTaints []corev1.Taint
	if ng.Spec.NodeTemplate != nil {
		desiredLabels = ng.Spec.NodeTemplate.Labels
		desiredAnnotations = ng.Spec.NodeTemplate.Annotations
		desiredTaints = ng.Spec.NodeTemplate.Taints
	}

	// 5. Parse last-applied-node-template from the node annotation.
	var lastAppliedLabels, lastAppliedAnnotations map[string]string
	var lastAppliedTaints []corev1.Taint
	if raw, exists := node.Annotations[lastAppliedNodeTemplateAnnotation]; exists && raw != "" {
		lastApplied, err := parseLastApplied(raw)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("parse last-applied-node-template on node %s: %w", req.Name, err)
		}
		lastAppliedLabels = lastApplied.Labels
		lastAppliedAnnotations = lastApplied.Annotations
		lastAppliedTaints = lastApplied.Taints
	}

	patch := client.MergeFrom(node.DeepCopy())
	changed := false

	// 6. Apply labels.
	newLabels, labelsChanged := applyTemplateMap(node.Labels, desiredLabels, lastAppliedLabels)

	// 6.1. Ensure node-role label for the node group.
	roleLabel := "node-role.kubernetes.io/" + ng.Name
	if v, ok := newLabels[roleLabel]; !ok || v != "" {
		labelsChanged = true
	}
	newLabels[roleLabel] = ""

	// 6.2. Ensure node type label.
	desiredType := string(ng.Spec.NodeType)
	if v, ok := newLabels[nodeTypeLabel]; !ok || v != desiredType {
		labelsChanged = true
	}
	newLabels[nodeTypeLabel] = desiredType

	// 6.3. Master node group: enforce control-plane and master roles.
	if ng.Name == "master" {
		if _, ok := newLabels["node-role.kubernetes.io/control-plane"]; !ok {
			labelsChanged = true
		}
		newLabels["node-role.kubernetes.io/control-plane"] = ""

		if _, ok := newLabels["node-role.kubernetes.io/master"]; !ok {
			labelsChanged = true
		}
		newLabels["node-role.kubernetes.io/master"] = ""
	}

	if labelsChanged {
		node.SetLabels(newLabels)
		changed = true
	}

	// 7. Apply annotations.
	newAnnotations, annotationsChanged := applyTemplateMap(node.Annotations, desiredAnnotations, lastAppliedAnnotations)

	// 7.1. Prevent scale-down for non-ephemeral nodes.
	if isScaleDownProtected(ng.Spec.NodeType) {
		if v, ok := newAnnotations[scaleDownDisabledAnnotation]; !ok || v != "true" {
			annotationsChanged = true
		}
		newAnnotations[scaleDownDisabledAnnotation] = "true"
	}

	// 7.2. Build and store last-applied-node-template annotation.
	newLastApplied, err := buildLastApplied(desiredLabels, desiredAnnotations, desiredTaints)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("marshal last-applied-node-template: %w", err)
	}
	if v, ok := newAnnotations[lastAppliedNodeTemplateAnnotation]; !ok || v != newLastApplied {
		annotationsChanged = true
	}
	newAnnotations[lastAppliedNodeTemplateAnnotation] = newLastApplied

	if annotationsChanged {
		node.SetAnnotations(newAnnotations)
		changed = true
	}

	// 8. Apply taints.
	newTaints, taintsChanged := applyTemplateTaints(node.Spec.Taints, desiredTaints, lastAppliedTaints)

	// 8.1. Remove the uninitialized taint.
	if taintsHasKey(newTaints, nodeUninitializedTaintKey) {
		taintsChanged = true
		newTaints = taintsWithoutKey(newTaints, nodeUninitializedTaintKey)
	}

	// 8.2. Master node group: fix master/control-plane taint consistency.
	if ng.Name == "master" {
		newTaints, taintsChanged = fixMasterTaints(newTaints, desiredTaints, taintsChanged)
	}

	if taintsChanged {
		if len(newTaints) == 0 {
			node.Spec.Taints = nil
		} else {
			node.Spec.Taints = newTaints
		}
		changed = true
	}

	// 9. Patch the node if anything changed.
	if !changed {
		log.V(1).Info("node template already up to date", "node", req.Name, "nodeGroup", ngName)
		return ctrl.Result{}, nil
	}

	// Clear status before patching — we only patch metadata + spec.
	node.Status = corev1.NodeStatus{}
	if err := r.Client.Patch(ctx, node, patch); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch node %s: %w", req.Name, err)
	}

	log.Info("applied node template", "node", req.Name, "nodeGroup", ngName)
	return ctrl.Result{}, nil
}

// lastAppliedNodeTemplate mirrors the JSON structure stored in the last-applied annotation.
type lastAppliedNodeTemplate struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Taints      []corev1.Taint    `json:"taints"`
}

// parseLastApplied decodes the last-applied-node-template annotation value.
func parseLastApplied(raw string) (*lastAppliedNodeTemplate, error) {
	var la lastAppliedNodeTemplate
	if err := json.Unmarshal([]byte(raw), &la); err != nil {
		return nil, err
	}
	return &la, nil
}

// buildLastApplied constructs the JSON string for the last-applied-node-template annotation.
// It mimics the hook behaviour: empty maps/slices are always present in the output.
func buildLastApplied(labels, annotations map[string]string, taints []corev1.Taint) (string, error) {
	m := map[string]interface{}{
		"annotations": make(map[string]string),
		"labels":      make(map[string]string),
		"taints":      make([]corev1.Taint, 0),
	}
	if len(annotations) > 0 {
		m["annotations"] = annotations
	}
	if len(labels) > 0 {
		m["labels"] = labels
	}
	if len(taints) > 0 {
		m["taints"] = taints
	}

	data, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// isScaleDownProtected returns true for node types that should have
// the cluster-autoscaler scale-down-disabled annotation.
func isScaleDownProtected(nodeType deckhousev1.NodeType) bool {
	switch nodeType {
	case deckhousev1.NodeTypeCloudPermanent, deckhousev1.NodeTypeCloudStatic, deckhousev1.NodeTypeStatic:
		return true
	default:
		return false
	}
}

// fixMasterTaints ensures consistency between master and control-plane taints
// on master node group nodes. If control-plane taint was removed (single-node
// installation), the master taint is also removed unless it is explicitly set
// in the NodeGroup template.
func fixMasterTaints(taints []corev1.Taint, ngTaints []corev1.Taint, alreadyChanged bool) ([]corev1.Taint, bool) {
	if len(taints) == 0 {
		return taints, alreadyChanged
	}

	taintsByKey := make(map[string]*corev1.Taint, len(taints))
	for i := range taints {
		taintsByKey[taints[i].Key] = &taints[i]
	}

	ngTaintKeys := make(map[string]struct{}, len(ngTaints))
	for _, t := range ngTaints {
		ngTaintKeys[t.Key] = struct{}{}
	}

	// If the control-plane taint is absent, it means a single-node installation
	// where control-plane taint was intentionally removed. In this case, also remove
	// the master taint unless it is explicitly set in the NG template.
	if _, hasControlPlane := taintsByKey["node-role.kubernetes.io/control-plane"]; !hasControlPlane {
		_, masterInNG := ngTaintKeys["node-role.kubernetes.io/master"]
		_, masterOnNode := taintsByKey["node-role.kubernetes.io/master"]
		if masterOnNode && !masterInNG {
			delete(taintsByKey, "node-role.kubernetes.io/master")
			result := make([]corev1.Taint, 0, len(taintsByKey))
			for _, t := range taintsByKey {
				result = append(result, *t)
			}
			return result, true
		}
	}

	return taints, alreadyChanged
}
