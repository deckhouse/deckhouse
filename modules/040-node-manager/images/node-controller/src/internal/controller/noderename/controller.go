/*
Copyright 2026 Flant JSC

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

// Package noderename collects the Node object a renamed node left behind.
//
// A Node object cannot be renamed, so /var/lib/bashible/rename_node.sh renames a
// node by giving kubelet a new identity and letting it register again. That
// leaves the cluster holding two Node objects for one machine: the new one, which
// is running, and the old one, which nothing will ever report to again. The
// renamed node marks itself with the name it used to have, and this controller
// removes the Node object of that name.
//
// It removes one only when every one of these holds:
//
//   - the annotation is on a Static or CloudStatic node - the only kinds whose
//     name is their own and not their machine's;
//   - the renamed node is Ready, so a rename that did not work leaves the old
//     Node object in place to go back to;
//   - the old Node is not Ready, so a name still in use is never taken away;
//   - the old Node is the same machine, by the system UUID and machine ID kubelet
//     reports. This is the load-bearing check: without it the annotation would be
//     a way for one node to delete another, and a node can write its own
//     annotations.
package noderename

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	register.RegisterController("node-rename", &corev1.Node{}, &Reconciler{})
}

const (
	// RenamedFromAnnotation carries the name the node was known by before it was
	// renamed. rename_node.sh puts it there through bashible; this controller is
	// what takes it off again, once the old Node object is gone.
	RenamedFromAnnotation = "node.deckhouse.io/renamed-from"

	nodeTypeStatic      = "Static"
	nodeTypeCloudStatic = "CloudStatic"

	// How long to wait before looking again at a rename that is not finished
	// yet - the renamed node still settling, or the old kubelet still reporting.
	// Node events do not carry the old Node's transitions to the new Node's
	// reconcile, so this is what moves the rename along.
	retryInterval = 30 * time.Second
)

type Reconciler struct {
	register.Base
}

func (r *Reconciler) SetupWatches(_ register.Watcher) {}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("node", req.Name)

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, req.NamespacedName, node); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	oldName := node.Annotations[RenamedFromAnnotation]
	if oldName == "" {
		return ctrl.Result{}, nil
	}

	logger = logger.WithValues("renamedFrom", oldName)

	if oldName == node.Name {
		// Nothing was renamed. Whatever put this here has nothing to ask for.
		return ctrl.Result{}, r.clearAnnotation(ctx, node)
	}

	switch node.Labels[nodecommon.NodeTypeLabel] {
	case nodeTypeStatic, nodeTypeCloudStatic:
	default:
		logger.Info("refusing to collect the previous Node of a node that is not static",
			"nodeType", node.Labels[nodecommon.NodeTypeLabel])
		return ctrl.Result{}, r.clearAnnotation(ctx, node)
	}

	if !isReady(node) {
		logger.V(1).Info("the renamed node is not Ready yet, keeping its previous Node object")
		return ctrl.Result{RequeueAfter: retryInterval}, nil
	}

	old := &corev1.Node{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: oldName}, old)
	if apierrors.IsNotFound(err) {
		logger.V(1).Info("the previous Node object is already gone")
		return ctrl.Result{}, r.clearAnnotation(ctx, node)
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	if isReady(old) {
		// Two live nodes claim to be one machine. Deleting either would be worse
		// than leaving both: say so and stop.
		logger.Info("refusing to collect the previous Node object: it is still Ready")
		r.Recorder.Eventf(node, corev1.EventTypeWarning, "NodeRenameBlocked",
			"Node %s was renamed from %s, but %s is still Ready and was left alone", node.Name, oldName, oldName)
		return ctrl.Result{RequeueAfter: retryInterval}, nil
	}

	if !sameMachine(old, node) {
		logger.Info("refusing to collect the previous Node object: it is a different machine",
			"oldSystemUUID", old.Status.NodeInfo.SystemUUID, "newSystemUUID", node.Status.NodeInfo.SystemUUID)
		r.Recorder.Eventf(node, corev1.EventTypeWarning, "NodeRenameRejected",
			"Node %s claims to have been renamed from %s, but %s is a different machine and was left alone", node.Name, oldName, oldName)
		return ctrl.Result{}, r.clearAnnotation(ctx, node)
	}

	logger.Info("collecting the Node object left behind by the rename")
	if err := r.Client.Delete(ctx, old); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("delete node %s: %w", oldName, err)
	}

	r.Recorder.Eventf(node, corev1.EventTypeNormal, "NodeRenamed",
		"Node was renamed from %s; the Node object %s has been removed", oldName, oldName)

	return ctrl.Result{}, r.clearAnnotation(ctx, node)
}

// clearAnnotation closes the handshake: while the annotation is there the node is
// still asking, and every reconcile of it would ask again.
func (r *Reconciler) clearAnnotation(ctx context.Context, node *corev1.Node) error {
	patch, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]any{
				RenamedFromAnnotation: nil,
			},
		},
	})
	if err != nil {
		return err
	}

	if err := r.Client.Patch(ctx, node, client.RawPatch(types.MergePatchType, patch)); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("clear the %s annotation on node %s: %w", RenamedFromAnnotation, node.Name, err)
	}
	return nil
}

// sameMachine reports whether two Node objects are the same physical or virtual
// machine. Both identifiers have to be present and equal: an empty one proves
// nothing, and a machine ID alone is shared by every VM cloned from one image.
func sameMachine(a, b *corev1.Node) bool {
	if a.Status.NodeInfo.SystemUUID == "" || a.Status.NodeInfo.MachineID == "" {
		return false
	}
	return a.Status.NodeInfo.SystemUUID == b.Status.NodeInfo.SystemUUID &&
		a.Status.NodeInfo.MachineID == b.Status.NodeInfo.MachineID
}

func isReady(node *corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}
