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

package nodetemplate

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	register.RegisterController("node-template", &corev1.Node{}, &Reconciler{})
}

type Reconciler struct {
	register.Base
}

func (r *Reconciler) SetupWatches(w register.Watcher) {
	allMapper := handler.EnqueueRequestsFromMapFunc(func(_ context.Context, _ client.Object) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: allRequestName}}}
	})
	w.Watches(&v1.NodeGroup{}, allMapper)
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if req.Name != allRequestName {
		logger.V(1).Info("reconciling single node template", "node", req.Name)
		return r.reconcileSingleNode(ctx, req, logger)
	}

	logger.V(1).Info("reconciling all node templates (NodeGroup changed)")
	return r.reconcileAllNodes(ctx, logger)
}

func (r *Reconciler) reconcileSingleNode(ctx context.Context, req ctrl.Request, logger logr.Logger) (ctrl.Result, error) {
	node := &corev1.Node{}
	if err := r.Client.Get(ctx, req.NamespacedName, node); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	nodeGroupName := node.Labels[nodeGroupNameLabel]
	if nodeGroupName == "" {
		logger.V(1).Info("skipping: node has no nodeGroup label", "node", node.Name)
		return ctrl.Result{}, nil
	}

	ng, err := nodecommon.GetNodeGroup(ctx, r.Client, nodeGroupName)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.V(1).Info("skipping: NodeGroup not found", "node", node.Name, "nodeGroup", nodeGroupName)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	changed, err := r.reconcileNode(ctx, node, ng)
	if err != nil {
		logger.Error(err, "failed to reconcile node template", "node", node.Name, "nodeGroup", ng.Name)
		return ctrl.Result{}, err
	}
	if changed {
		logger.Info("node template applied", "node", node.Name, "nodeGroup", ng.Name, "nodeType", ng.Spec.NodeType)
	} else {
		logger.V(1).Info("node template unchanged", "node", node.Name, "nodeGroup", ng.Name)
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) reconcileAllNodes(ctx context.Context, logger logr.Logger) (ctrl.Result, error) {
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

	var changedCount int
	for i := range nodes {
		node := &nodes[i]
		nodeGroupName := node.Labels[nodeGroupNameLabel]
		if nodeGroupName == "" {
			continue
		}

		ng, ok := ngByName[nodeGroupName]
		if !ok {
			logger.V(1).Info("skipping: NodeGroup not found for node", "node", node.Name, "nodeGroup", nodeGroupName)
			continue
		}

		changed, err := r.reconcileNode(ctx, node, &ng)
		if err != nil {
			logger.Error(err, "failed to reconcile node template", "node", node.Name, "nodeGroup", ng.Name)
			return ctrl.Result{}, err
		}
		if changed {
			changedCount++
			logger.Info("node template applied", "node", node.Name, "nodeGroup", ng.Name, "nodeType", ng.Spec.NodeType)
		}
	}

	logger.V(1).Info("reconcile all nodes completed", "totalNodes", len(nodes), "nodeGroups", len(ngByName), "changed", changedCount)
	return ctrl.Result{}, nil
}
