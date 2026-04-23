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

package nodetemplate

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

type Reconciler struct {
	Client client.Client
}

func SetupNodeTemplate(mgr ctrl.Manager) error {
	return (&Reconciler{Client: mgr.GetClient()}).SetupWithManager(mgr)
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	allMapper := handler.EnqueueRequestsFromMapFunc(func(context.Context, client.Object) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: allRequestName}}}
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		Watches(&v1.NodeGroup{}, allMapper).
		Named(controllerName).
		Complete(r)
}

func (r *Reconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var nodeList corev1.NodeList
	if err := r.Client.List(ctx, &nodeList); err != nil {
		return ctrl.Result{}, err
	}
	nodes := nodeList.Items

	var ngList v1.NodeGroupList
	if err := r.Client.List(ctx, &ngList); err != nil {
		return ctrl.Result{}, err
	}

	ngByName := make(map[string]v1.NodeGroup, len(ngList.Items))
	for _, ng := range ngList.Items {
		ngByName[ng.Name] = ng
	}

	r.syncUnmanagedNodesMetric(nodes)
	r.syncMissingMasterTaintMetric(ngList.Items, nodes)

	for i := range nodes {
		node := &nodes[i]
		nodeGroupName := node.Labels[nodeGroupNameLabel]
		if nodeGroupName == "" {
			continue
		}

		ng, ok := ngByName[nodeGroupName]
		if !ok {
			continue
		}

		changed, err := r.reconcileNode(ctx, node, &ng)
		if err != nil {
			return ctrl.Result{}, err
		}
		if changed {
			logger.V(1).Info("node template reconciled", "node", node.Name, "nodeGroup", ng.Name)
		}
	}

	return ctrl.Result{}, nil
}

var _ reconcile.Reconciler = (*Reconciler)(nil)
