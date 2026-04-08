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

package nodeuser

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/dynr"
	"github.com/deckhouse/node-controller/internal/rcname"
)

const (
	requeueInterval = 30 * time.Minute
	nodeGroupLabel  = "node.deckhouse.io/group"
)

func init() {
	dynr.RegisterReconciler(rcname.NodeUser, &deckhousev1.NodeUser{}, &Reconciler{})
}

var _ dynr.Reconciler = (*Reconciler)(nil)

// Reconciler clears stale errors from NodeUser status.
// An error is considered stale when its key (node name) references a node
// that no longer exists in the cluster.
type Reconciler struct {
	dynr.Base
}

func (r *Reconciler) SetupWatches(_ dynr.Watcher) {}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	nu := &deckhousev1.NodeUser{}
	if err := r.Client.Get(ctx, req.NamespacedName, nu); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if len(nu.Status.Errors) == 0 {
		return ctrl.Result{RequeueAfter: requeueInterval}, nil
	}

	// Build a set of existing node names.
	// The original hook lists nodes with label "node.deckhouse.io/group" (Exists).
	selector, err := labels.Parse(nodeGroupLabel)
	if err != nil {
		// Should never happen with a constant label key.
		return ctrl.Result{}, fmt.Errorf("parse label selector: %w", err)
	}

	nodeList := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodeList, client.MatchingLabelsSelector{
		Selector: selector,
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("list nodes: %w", err)
	}

	existingNodes := make(map[string]struct{}, len(nodeList.Items))
	for _, node := range nodeList.Items {
		existingNodes[node.Name] = struct{}{}
	}

	// Find error entries whose node no longer exists.
	staleNodes := make([]string, 0)
	for nodeName := range nu.Status.Errors {
		if _, exists := existingNodes[nodeName]; !exists {
			staleNodes = append(staleNodes, nodeName)
		}
	}

	if len(staleNodes) == 0 {
		return ctrl.Result{RequeueAfter: requeueInterval}, nil
	}

	log.Info("clearing stale NodeUser errors", "nodeUser", nu.Name, "staleNodes", staleNodes)

	// Patch the status to remove stale error entries.
	patch := client.MergeFrom(nu.DeepCopy())
	for _, nodeName := range staleNodes {
		delete(nu.Status.Errors, nodeName)
	}

	if err := r.Client.Status().Patch(ctx, nu, patch); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch NodeUser %s status: %w", nu.Name, err)
	}

	return ctrl.Result{RequeueAfter: requeueInterval}, nil
}
