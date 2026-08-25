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

package nodeuser

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	register.RegisterController("nodeuser-status", &deckhousev1.NodeUser{}, &Reconciler{})
}

type Reconciler struct {
	register.Base
}

// SetupWatches reacts to node deletion: that is the only event that can turn an existing
// status.errors key stale. Node updates are filtered out — kubelet heartbeats would otherwise
// enqueue every NodeUser several times a second.
func (r *Reconciler) SetupWatches(w register.Watcher) {
	w.Watches(&corev1.Node{},
		handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, node client.Object) []reconcile.Request {
			return r.nodeUsersBlaming(ctx, node.GetName())
		}),
		builder.WithPredicates(predicate.Funcs{
			CreateFunc:  func(event.CreateEvent) bool { return false },
			UpdateFunc:  func(event.UpdateEvent) bool { return false },
			GenericFunc: func(event.GenericEvent) bool { return false },
			DeleteFunc:  func(event.DeleteEvent) bool { return true },
		}),
	)
}

// nodeUsersBlaming returns the NodeUsers whose status.errors names the deleted node — the only
// ones the deletion can make stale. Enqueuing every NodeUser instead would make a scale-down cost
// one reconcile per user per node.
func (r *Reconciler) nodeUsersBlaming(ctx context.Context, nodeName string) []reconcile.Request {
	list := &deckhousev1.NodeUserList{}
	if err := r.Client.List(ctx, list); err != nil {
		log.FromContext(ctx).Error(err, "listing NodeUsers after a node deletion")
		return nil
	}

	var reqs []reconcile.Request
	for i := range list.Items {
		if _, blamed := list.Items[i].Status.Errors[nodeName]; !blamed {
			continue
		}
		reqs = append(reqs, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: list.Items[i].Name},
		})
	}
	return reqs
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	nodeUser := &deckhousev1.NodeUser{}
	if err := r.Client.Get(ctx, req.NamespacedName, nodeUser); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if len(nodeUser.Status.Errors) == 0 {
		return ctrl.Result{}, nil
	}

	stale, err := r.staleNodeNames(ctx, nodeUser.Status.Errors)
	if err != nil {
		return ctrl.Result{}, err
	}
	if len(stale) == 0 {
		logger.V(1).Info("skipping: every error refers to an existing node", "nodeUser", nodeUser.Name)
		return ctrl.Result{}, nil
	}

	logger.Info("clearing errors of nodes that no longer exist", "nodeUser", nodeUser.Name, "nodes", stale)

	patch := staleErrorsPatch(stale)
	if err := r.Client.Status().Patch(ctx, nodeUser, client.RawPatch(types.MergePatchType, patch)); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch status.errors of NodeUser %s: %w", nodeUser.Name, err)
	}

	logger.Info("errors cleared", "nodeUser", nodeUser.Name)
	return ctrl.Result{}, nil
}

// staleNodeNames returns the blamed nodes that are gone or no longer carry the node-group label —
// the hook's selector. Reached on every NodeUser write too, not only on node deletion. Nodes are
// read one by one: listing every node would copy the whole cluster out of the cache each time.
func (r *Reconciler) staleNodeNames(ctx context.Context, statusErrors map[string]string) ([]string, error) {
	var stale []string
	for name := range statusErrors {
		node := &corev1.Node{}
		err := r.Client.Get(ctx, types.NamespacedName{Name: name}, node)
		if err != nil && !errors.IsNotFound(err) {
			return nil, fmt.Errorf("get node %s: %w", name, err)
		}
		if err == nil {
			if _, managed := node.Labels[common.NodeGroupLabel]; managed {
				continue
			}
		}
		stale = append(stale, name)
	}

	sort.Strings(stale)
	return stale, nil
}

// staleErrorsPatch builds a merge patch that removes the named keys: in a JSON merge patch a null
// value deletes the key, leaving the errors of the remaining nodes untouched. It is spelled out by
// hand rather than through client.MergeFrom because omitempty makes a typed diff that drops the
// last key send `errors: null`, which deletes the whole map.
func staleErrorsPatch(stale []string) []byte {
	errs := make(map[string]any, len(stale))
	for _, node := range stale {
		errs[node] = nil
	}

	patch, err := json.Marshal(map[string]any{"status": map[string]any{"errors": errs}})
	if err != nil {
		// Marshalling a map of string keys and nil values has no failure mode.
		panic(fmt.Sprintf("marshal status.errors patch: %v", err))
	}
	return patch
}
