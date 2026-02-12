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

package updateapproval

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	nodecommon "github.com/deckhouse/node-controller/internal/common"
	ngcommon "github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
	ua "github.com/deckhouse/node-controller/internal/controller/updateapproval/common"
	"github.com/deckhouse/node-controller/internal/controller/updateapproval/engine"
	"github.com/deckhouse/node-controller/internal/controller/updateapproval/kubeclient"
	uametrics "github.com/deckhouse/node-controller/internal/controller/updateapproval/metrics"
	"github.com/deckhouse/node-controller/internal/register/dynctrl"
)

func init() {
	uametrics.Register()
}

type Reconciler struct {
	dynctrl.Base
	deckhouseNodeName string
}

func New() *Reconciler {
	return &Reconciler{
		deckhouseNodeName: os.Getenv("DECKHOUSE_NODE_NAME"),
	}
}

func (r *Reconciler) SetupWatches(w dynctrl.Watcher) {
	w.Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(ngcommon.NodeToNodeGroup), builder.WithPredicates(ngcommon.NodeHasGroupLabelPredicate()))
	w.Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.secretToAllNodeGroups), builder.WithPredicates(nodecommon.ChecksumSecretPredicate()))
}

func (r *Reconciler) secretToAllNodeGroups(ctx context.Context, _ client.Object) []reconcile.Request {
	return nodecommon.SecretToAllNodeGroups(ctx, r.Client)
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("reconciling update approval", "nodegroup", req.Name)

	ng, err := nodecommon.GetNodeGroup(ctx, r.Client, req.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get nodegroup %s: %w", req.Name, err)
	}
	logger.V(1).Info("updateapproval input snapshot", "nodegroup", ng.Name, "nodeType", ng.Spec.NodeType, "statusDesired", ng.Status.Desired, "statusReady", ng.Status.Ready, "statusNodes", ng.Status.Nodes)

	kubeSvc := kubeclient.Client{Client: r.Client}
	engineSvc := engine.Processor{
		Kube:              kubeSvc,
		Recorder:          r.Recorder,
		DeckhouseNodeName: r.deckhouseNodeName,
	}

	checksums, err := kubeSvc.GetConfigurationChecksums(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	if len(checksums) == 0 {
		logger.V(1).Info("no configuration checksums secret found, skipping")
		return ctrl.Result{}, nil
	}
	ngChecksum := checksums[ng.Name]

	nodes, err := kubeSvc.GetNodesForNodeGroup(ctx, ng.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	nodeInfos := make([]ua.NodeInfo, 0, len(nodes))
	for _, node := range nodes {
		nodeInfos = append(nodeInfos, ua.BuildNodeInfo(&node))
	}

	for _, node := range nodeInfos {
		uametrics.SetNodeMetrics(node, ng, ngChecksum)
	}

	finished, err := engineSvc.ProcessUpdatedNodes(ctx, ng, nodeInfos, ngChecksum)
	if err != nil {
		if errors.IsConflict(err) {
			logger.Info("process updated nodes conflict, likely concurrent node patch", "nodegroup", ng.Name)
		}
		return ctrl.Result{}, err
	}
	if finished {
		logger.V(1).Info("updateapproval phase finished", "phase", "ProcessUpdatedNodes", "nodegroup", ng.Name)
		return ctrl.Result{}, nil
	}

	finished, err = engineSvc.ApproveDisruptions(ctx, ng, nodeInfos)
	if err != nil {
		if errors.IsConflict(err) {
			logger.Info("approve disruptions conflict, likely concurrent node patch", "nodegroup", ng.Name)
		}
		return ctrl.Result{}, err
	}
	if finished {
		logger.V(1).Info("updateapproval phase finished", "phase", "ApproveDisruptions", "nodegroup", ng.Name)
		return ctrl.Result{}, nil
	}

	if _, err := engineSvc.ApproveUpdates(ctx, ng, nodeInfos); err != nil {
		if errors.IsConflict(err) {
			logger.Info("approve updates conflict, likely concurrent node patch", "nodegroup", ng.Name)
		}
		return ctrl.Result{}, err
	}
	logger.V(1).Info("updateapproval completed without mutations", "nodegroup", ng.Name)

	return ctrl.Result{}, nil
}
