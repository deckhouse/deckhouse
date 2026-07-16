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

// Package nodeusercleanup drops stale per-node entries from NodeUser.status.errors.
//
// NodeUser.status.errors is a map keyed by node name. When a node is removed its
// error entry can linger. This controller removes entries whose node no longer
// exists among the nodes carrying the node.deckhouse.io/group label.
//
// This replaces the shell-operator hook hooks/clear_nodeuser_errors.go. The hook
// ran on a 30-minute schedule plus Node/NodeUser synchronization; the controller
// reconciles a NodeUser reactively on its own changes and re-checks every NodeUser
// when a Node is deleted, so stale entries are cleared promptly instead of on the
// next cron tick.
package nodeusercleanup

import (
	"context"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	register.RegisterController("node-nodeuser-error-cleanup", &deckhousev1.NodeUser{}, &Reconciler{})
}

type Reconciler struct {
	register.Base
}

func (r *Reconciler) SetupWatches(w register.Watcher) {
	// A node deletion can strand an error entry in any NodeUser, so a Node
	// removal re-enqueues every NodeUser. Only deletions matter here; node
	// create/update never turns an existing entry stale.
	w.Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(r.nodeToNodeUsers),
		builder.WithPredicates(predicate.Funcs{
			CreateFunc:  func(event.CreateEvent) bool { return false },
			UpdateFunc:  func(event.UpdateEvent) bool { return false },
			DeleteFunc:  func(event.DeleteEvent) bool { return true },
			GenericFunc: func(event.GenericEvent) bool { return false },
		}))
}

func (r *Reconciler) nodeToNodeUsers(ctx context.Context, _ client.Object) []reconcile.Request {
	list := &deckhousev1.NodeUserList{}
	if err := r.Client.List(ctx, list); err != nil {
		log.FromContext(ctx).Error(err, "failed to list NodeUsers for node deletion")
		return nil
	}
	reqs := make([]reconcile.Request, 0, len(list.Items))
	for i := range list.Items {
		reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: list.Items[i].Name}})
	}
	return reqs
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	nodeUser := &deckhousev1.NodeUser{}
	if err := r.Client.Get(ctx, req.NamespacedName, nodeUser); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if len(nodeUser.Status.Errors) == 0 {
		return ctrl.Result{}, nil
	}

	existing, err := r.existingGroupNodes(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	stale := make([]string, 0)
	for node := range nodeUser.Status.Errors {
		if _, ok := existing[node]; !ok {
			stale = append(stale, node)
		}
	}
	if len(stale) == 0 {
		return ctrl.Result{}, nil
	}

	if err := r.clearStaleErrors(ctx, nodeUser, stale); err != nil {
		logger.Error(err, "failed to clear stale NodeUser errors", "nodeUser", nodeUser.Name)
		return ctrl.Result{}, err
	}

	logger.Info("cleared stale NodeUser errors", "nodeUser", nodeUser.Name, "nodes", stale)
	return ctrl.Result{}, nil
}

func (r *Reconciler) existingGroupNodes(ctx context.Context) (map[string]struct{}, error) {
	nodes := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodes, client.HasLabels{nodecommon.NodeGroupLabel}); err != nil {
		return nil, err
	}
	set := make(map[string]struct{}, len(nodes.Items))
	for i := range nodes.Items {
		set[nodes.Items[i].Name] = struct{}{}
	}
	return set, nil
}

// clearStaleErrors removes the given node keys from status.errors with a JSON
// merge patch (null value deletes the key), mirroring the hook's PatchWithMerge
// on the status subresource.
func (r *Reconciler) clearStaleErrors(ctx context.Context, nodeUser *deckhousev1.NodeUser, staleNodes []string) error {
	errorsPatch := make(map[string]any, len(staleNodes))
	for _, node := range staleNodes {
		errorsPatch[node] = nil
	}
	body := map[string]any{"status": map[string]any{"errors": errorsPatch}}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return r.Client.Status().Patch(ctx, nodeUser, client.RawPatch(types.MergePatchType, raw))
}
